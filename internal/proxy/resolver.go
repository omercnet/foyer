package proxy

import (
	"context"
	"fmt"
	"net"
)

// Resolver resolves a hostname to a single IP address. The SOCKS5 server uses
// it to turn the domain names that the browser sends (it is told not to resolve
// anything locally) into addresses it can dial.
type Resolver interface {
	Resolve(ctx context.Context, host string) (net.IP, error)
}

// UpstreamResolver performs every DNS lookup against a fixed upstream resolver,
// typically the DNS server handed out by the captive network's DHCP server.
//
// This is the load-bearing trick of the whole tool: captive portals hijack DNS,
// so the browser must resolve names through the portal's resolver rather than
// the user's system resolver (which may be encrypted/pinned and never sees the
// hijack). PreferGo forces Go's pure-Go resolver so the Dial override below is
// honored, sending every query to upstream:53 instead of the system config.
type UpstreamResolver struct {
	addr string // dial address (host:port) shown in errors
	r    *net.Resolver
}

// NewUpstreamResolver returns a Resolver that sends all DNS queries to upstream
// (an IP address) on the standard DNS port. Dialing is performed through dialer,
// so any device binding configured on the dialer also applies to DNS traffic.
func NewUpstreamResolver(upstream string, dialer *net.Dialer) *UpstreamResolver {
	return newUpstreamResolver(net.JoinHostPort(upstream, "53"), dialer)
}

// newUpstreamResolver is the addr-taking constructor used internally and in
// tests, where DNS must be served on a non-privileged port.
func newUpstreamResolver(addr string, dialer *net.Dialer) *UpstreamResolver {
	return &UpstreamResolver{
		addr: addr,
		r: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}
}

// Resolve looks up host through the upstream resolver and returns a single IP,
// preferring IPv4 because captive portals are an IPv4-era concern and the SOCKS5
// reply carries one address.
func (u *UpstreamResolver) Resolve(ctx context.Context, host string) (net.IP, error) {
	addrs, err := u.r.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q via %s: %w", host, u.addr, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve %q via %s: no addresses", host, u.addr)
	}
	for _, addr := range addrs {
		if v4 := addr.IP.To4(); v4 != nil {
			return v4, nil
		}
	}
	return addrs[0].IP, nil
}
