package proxy

import (
	"context"
	"net"
	"testing"
	"time"
)

// startDNS starts a minimal UDP DNS server that answers every A query with ip
// (and returns NODATA for other types). It returns the server's address.
func startDNS(t *testing.T, ip net.IP) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if resp := buildDNSResponse(buf[:n], ip); resp != nil {
				_, _ = pc.WriteTo(resp, addr)
			}
		}
	}()
	return pc.LocalAddr().String()
}

// buildDNSResponse crafts a reply to a single-question DNS query: an A record
// pointing at ip for type-A queries, NODATA for anything else.
func buildDNSResponse(query []byte, ip net.IP) []byte {
	if len(query) < 12 {
		return nil
	}
	i := 12
	for i < len(query) && query[i] != 0 {
		i += int(query[i]) + 1
	}
	if i >= len(query) {
		return nil
	}
	questionEnd := i + 1 + 4 // null label + QTYPE + QCLASS
	if questionEnd > len(query) {
		return nil
	}
	qtype := uint16(query[i+1])<<8 | uint16(query[i+2])
	v4 := ip.To4()
	answer := qtype == 1 && v4 != nil

	resp := []byte{query[0], query[1], 0x81, 0x80, 0x00, 0x01}
	if answer {
		resp = append(resp, 0x00, 0x01)
	} else {
		resp = append(resp, 0x00, 0x00)
	}
	resp = append(resp, 0x00, 0x00, 0x00, 0x00) // NSCOUNT, ARCOUNT
	resp = append(resp, query[12:questionEnd]...)
	if answer {
		resp = append(resp, 0xC0, 0x0C, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x3C, 0x00, 0x04)
		resp = append(resp, v4...)
	}
	return resp
}

func TestUpstreamResolverResolvesViaUpstream(t *testing.T) {
	t.Parallel()
	want := net.IPv4(203, 0, 113, 7)
	dnsAddr := startDNS(t, want)
	r := newUpstreamResolver(dnsAddr, &net.Dialer{})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got, err := r.Resolve(ctx, "portal.example.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("Resolve = %v, want %v", got, want)
	}
}

func TestUpstreamResolverErrorsWhenUpstreamDown(t *testing.T) {
	t.Parallel()
	// Grab a port, then release it so dials are refused.
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	_ = pc.Close()

	r := newUpstreamResolver(addr, &net.Dialer{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := r.Resolve(ctx, "portal.example."); err == nil {
		t.Error("expected error when upstream resolver is unreachable")
	}
}
