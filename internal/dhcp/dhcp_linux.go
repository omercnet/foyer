//go:build linux

package dhcp

import (
	"context"
	"fmt"
	"os"
)

// DefaultInterface returns the interface backing the default route by reading
// /proc/net/route, avoiding a dependency on iproute2/NetworkManager binaries.
func DefaultInterface(_ context.Context) (string, error) {
	content, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "", fmt.Errorf("dhcp: read /proc/net/route: %w", err)
	}
	return parseProcNetRoute(string(content))
}

// DefaultCommand returns a shell command that prints the DHCP DNS server for
// iface on Linux. resolvectl (systemd-resolved) is preferred; nmcli is a common
// fallback. Either way Discover scans the output for the first IPv4 address.
func DefaultCommand(iface string) string {
	return fmt.Sprintf(
		"resolvectl dns %s 2>/dev/null || nmcli -g IP4.DNS device show %s 2>/dev/null",
		iface, iface,
	)
}
