// Package proxy implements a minimal, self-contained SOCKS5 proxy (RFC 1928,
// CONNECT only, no authentication) that resolves hostnames through a fixed
// upstream DNS server. It is intentionally small and dependency-free so that a
// privacy tool's network path stays auditable.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

// SOCKS5 protocol constants (RFC 1928).
const (
	version5     = 0x05
	cmdConnect   = 0x01
	rsv          = 0x00
	authNone     = 0x00
	authNoAccept = 0xFF

	atypIPv4   = 0x01
	atypDomain = 0x03
	atypIPv6   = 0x04
)

// Reply codes (RFC 1928 §6).
const (
	repSuccess          = 0x00
	repGeneralFailure   = 0x01
	repHostUnreachable  = 0x04
	repConnRefused      = 0x05
	repCmdNotSupported  = 0x07
	repAddrNotSupported = 0x08
)

// handshakeTimeout bounds the SOCKS negotiation so a stalled client cannot hold
// a goroutine open forever; it does not limit the proxied connection itself.
const handshakeTimeout = 10 * time.Second

// Server is a SOCKS5 CONNECT proxy. It must be configured with a Resolver and a
// Dialer before Serve is called.
type Server struct {
	// Resolver turns the hostnames the browser sends into IP addresses.
	Resolver Resolver
	// Dialer dials the upstream connection; it may bind to a specific device.
	Dialer *net.Dialer
	// Logger receives per-request diagnostics. Defaults to slog.Default().
	Logger *slog.Logger
}

func (s *Server) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// Serve accepts connections on l until l is closed or ctx is cancelled. When
// ctx is cancelled it closes l, which unblocks Accept. It always returns a
// non-nil error explaining why it stopped.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	if s.Resolver == nil {
		return errors.New("proxy: Resolver is nil")
	}
	if s.Dialer == nil {
		return errors.New("proxy: Dialer is nil")
	}

	var wg sync.WaitGroup
	context.AfterFunc(ctx, func() { _ = l.Close() })

	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				wg.Wait()
				return fmt.Errorf("proxy: stopped: %w", ctx.Err())
			}
			return fmt.Errorf("proxy: accept: %w", err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.handle(ctx, conn)
		}()
	}
}

func (s *Server) handle(ctx context.Context, client net.Conn) {
	defer func() { _ = client.Close() }()

	_ = client.SetDeadline(time.Now().Add(handshakeTimeout))

	if err := negotiateAuth(client); err != nil {
		s.logger().Debug("auth negotiation failed", "err", err)
		return
	}

	host, port, err := readConnectRequest(client)
	if err != nil {
		s.logger().Debug("request parse failed", "err", err)
		var re replyError
		if errors.As(err, &re) {
			_ = sendReply(client, re.code)
		} else {
			_ = sendReply(client, repGeneralFailure)
		}
		return
	}

	ip, err := s.Resolver.Resolve(ctx, host)
	if err != nil {
		s.logger().Warn("dns lookup failed", "host", host, "err", err)
		_ = sendReply(client, repHostUnreachable)
		return
	}
	s.logger().Info("proxying", "host", host, "ip", ip.String(), "port", port)

	target := net.JoinHostPort(ip.String(), fmt.Sprint(port))
	upstream, err := s.Dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		s.logger().Warn("dial failed", "target", target, "err", err)
		_ = sendReply(client, repConnRefused)
		return
	}
	defer func() { _ = upstream.Close() }()

	if err := sendReply(client, repSuccess); err != nil {
		return
	}

	// The handshake is done; let the tunnel run without a deadline.
	_ = client.SetDeadline(time.Time{})
	pipe(client, upstream)
}

// negotiateAuth reads the client greeting and selects the no-authentication
// method. The proxy only ever listens on the loopback interface.
func negotiateAuth(client net.Conn) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(client, header); err != nil {
		return fmt.Errorf("read greeting: %w", err)
	}
	if header[0] != version5 {
		return fmt.Errorf("unsupported SOCKS version %d", header[0])
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(client, methods); err != nil {
		return fmt.Errorf("read methods: %w", err)
	}
	for _, m := range methods {
		if m == authNone {
			_, err := client.Write([]byte{version5, authNone})
			return err
		}
	}
	_, _ = client.Write([]byte{version5, authNoAccept})
	return errors.New("client offered no acceptable auth method")
}

// replyError carries a SOCKS reply code so the caller can inform the client.
type replyError struct {
	code byte
	msg  string
}

func (e replyError) Error() string { return e.msg }

// readConnectRequest parses a CONNECT request and returns the target host
// (domain or IP literal) and port.
func readConnectRequest(client net.Conn) (host string, port uint16, err error) {
	header := make([]byte, 4)
	if _, err = io.ReadFull(client, header); err != nil {
		return "", 0, fmt.Errorf("read request header: %w", err)
	}
	if header[0] != version5 {
		return "", 0, replyError{repGeneralFailure, fmt.Sprintf("unsupported version %d", header[0])}
	}
	if header[1] != cmdConnect {
		return "", 0, replyError{repCmdNotSupported, fmt.Sprintf("unsupported command %d", header[1])}
	}

	switch header[3] {
	case atypIPv4:
		buf := make([]byte, net.IPv4len)
		if _, err = io.ReadFull(client, buf); err != nil {
			return "", 0, fmt.Errorf("read ipv4: %w", err)
		}
		host = net.IP(buf).String()
	case atypIPv6:
		buf := make([]byte, net.IPv6len)
		if _, err = io.ReadFull(client, buf); err != nil {
			return "", 0, fmt.Errorf("read ipv6: %w", err)
		}
		host = net.IP(buf).String()
	case atypDomain:
		lenByte := make([]byte, 1)
		if _, err = io.ReadFull(client, lenByte); err != nil {
			return "", 0, fmt.Errorf("read domain length: %w", err)
		}
		buf := make([]byte, int(lenByte[0]))
		if _, err = io.ReadFull(client, buf); err != nil {
			return "", 0, fmt.Errorf("read domain: %w", err)
		}
		host = string(buf)
	default:
		return "", 0, replyError{repAddrNotSupported, fmt.Sprintf("unsupported address type %d", header[3])}
	}

	portBuf := make([]byte, 2)
	if _, err = io.ReadFull(client, portBuf); err != nil {
		return "", 0, fmt.Errorf("read port: %w", err)
	}
	port = uint16(portBuf[0])<<8 | uint16(portBuf[1])
	return host, port, nil
}

// sendReply writes a SOCKS5 reply with a zero bind address (RFC 1928 §6).
func sendReply(client net.Conn, code byte) error {
	_, err := client.Write([]byte{version5, code, rsv, atypIPv4, 0, 0, 0, 0, 0, 0})
	return err
}

// pipe copies data in both directions until either side closes, propagating a
// half-close so the peer sees EOF promptly.
func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go copyAndCloseWrite(a, b, &wg)
	go copyAndCloseWrite(b, a, &wg)
	wg.Wait()
}

type closeWriter interface{ CloseWrite() error }

func copyAndCloseWrite(dst, src net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	_, _ = io.Copy(dst, src)
	if cw, ok := dst.(closeWriter); ok {
		_ = cw.CloseWrite()
	} else {
		_ = dst.SetReadDeadline(time.Now())
	}
}
