package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

// stubResolver returns a fixed IP, or an error when ip is nil.
type stubResolver struct {
	ip  net.IP
	err error
}

func (s stubResolver) Resolve(context.Context, string) (net.IP, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.ip, nil
}

// startEcho starts a TCP server that echoes everything back; returns its port.
func startEcho(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(conn, conn); _ = conn.Close() }()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// startProxy starts a Server with the given resolver; returns its address.
func startProxy(t *testing.T, res Resolver) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		Resolver: res,
		Dialer:   &net.Dialer{},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Serve(ctx, ln) }()
	return ln.Addr().String()
}

// handshake performs the SOCKS5 greeting and sends a CONNECT request, returning
// the connection and the reply code.
func handshake(t *testing.T, proxyAddr string, request []byte) (net.Conn, byte) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", proxyAddr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := conn.Write([]byte{version5, 1, authNone}); err != nil {
		t.Fatal(err)
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(conn, methodReply); err != nil {
		t.Fatalf("read method reply: %v", err)
	}
	if methodReply[0] != version5 || methodReply[1] != authNone {
		t.Fatalf("unexpected method reply %v", methodReply)
	}

	if _, err := conn.Write(request); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 10)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read connect reply: %v", err)
	}
	return conn, reply[1]
}

func connectDomain(host string, port int) []byte {
	req := []byte{version5, cmdConnect, rsv, atypDomain, byte(len(host))}
	req = append(req, host...)
	return append(req, byte(port>>8), byte(port))
}

func TestConnectByDomainEchoes(t *testing.T) {
	t.Parallel()
	echoPort := startEcho(t)
	proxyAddr := startProxy(t, stubResolver{ip: net.IPv4(127, 0, 0, 1)})

	conn, code := handshake(t, proxyAddr, connectDomain("portal.test", echoPort))
	defer func() { _ = conn.Close() }()
	if code != repSuccess {
		t.Fatalf("reply code = %d, want success", code)
	}

	const msg = "hello captive portal"
	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != msg {
		t.Errorf("echo = %q, want %q", buf, msg)
	}
}

func TestConnectByIPv4Literal(t *testing.T) {
	t.Parallel()
	echoPort := startEcho(t)
	proxyAddr := startProxy(t, stubResolver{ip: net.IPv4(127, 0, 0, 1)})

	req := []byte{version5, cmdConnect, rsv, atypIPv4, 127, 0, 0, 1, byte(echoPort >> 8), byte(echoPort)}
	conn, code := handshake(t, proxyAddr, req)
	defer func() { _ = conn.Close() }()
	if code != repSuccess {
		t.Fatalf("reply code = %d, want success", code)
	}
}

func TestUnsupportedCommand(t *testing.T) {
	t.Parallel()
	proxyAddr := startProxy(t, stubResolver{ip: net.IPv4(127, 0, 0, 1)})
	const cmdBind = 0x02
	req := append([]byte{version5, cmdBind, rsv, atypDomain, 4}, "host"...)
	req = append(req, 0, 80)
	conn, code := handshake(t, proxyAddr, req)
	defer func() { _ = conn.Close() }()
	if code != repCmdNotSupported {
		t.Errorf("reply code = %d, want %d (cmd not supported)", code, repCmdNotSupported)
	}
}

func TestUnsupportedAddressType(t *testing.T) {
	t.Parallel()
	proxyAddr := startProxy(t, stubResolver{ip: net.IPv4(127, 0, 0, 1)})
	req := []byte{version5, cmdConnect, rsv, 0x09 /* bogus atyp */, 0, 80}
	conn, code := handshake(t, proxyAddr, req)
	defer func() { _ = conn.Close() }()
	if code != repAddrNotSupported {
		t.Errorf("reply code = %d, want %d (addr not supported)", code, repAddrNotSupported)
	}
}

func TestResolverFailure(t *testing.T) {
	t.Parallel()
	proxyAddr := startProxy(t, stubResolver{err: errors.New("nxdomain")})
	conn, code := handshake(t, proxyAddr, connectDomain("nope.test", 80))
	defer func() { _ = conn.Close() }()
	if code != repHostUnreachable {
		t.Errorf("reply code = %d, want %d (host unreachable)", code, repHostUnreachable)
	}
}

func TestBadVersionRejected(t *testing.T) {
	t.Parallel()
	proxyAddr := startProxy(t, stubResolver{ip: net.IPv4(127, 0, 0, 1)})
	conn, err := net.DialTimeout("tcp", proxyAddr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte{0x04 /* not SOCKS5 */, 1, authNone}); err != nil {
		t.Fatal(err)
	}
	// Server closes the connection without a reply.
	if _, err := io.ReadFull(conn, make([]byte, 2)); err == nil {
		t.Error("expected connection to be closed on bad version")
	}
}

func TestServeRequiresResolverAndDialer(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	if err := (&Server{}).Serve(context.Background(), ln); err == nil {
		t.Error("expected error when Resolver is nil")
	}
	if err := (&Server{Resolver: stubResolver{}}).Serve(context.Background(), ln); err == nil {
		t.Error("expected error when Dialer is nil")
	}
}

func TestServeStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		Resolver: stubResolver{ip: net.IPv4(127, 0, 0, 1)},
		Dialer:   &net.Dialer{},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln) }()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Serve returned %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not stop after context cancel")
	}
}
