//go:build darwin

package dhcp

import (
	"context"
	"fmt"
	"os/exec"
)

// DefaultInterface returns the interface backing the default route, e.g. en0.
func DefaultInterface(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "route", "-n", "get", "default").Output()
	if err != nil {
		return "", fmt.Errorf("dhcp: route get default: %w", err)
	}
	return parseRouteGetDefault(string(out))
}

// DefaultCommand returns the shell command that prints the DHCP DNS server for
// iface on macOS. `ipconfig getoption` reads the option directly from the lease.
func DefaultCommand(iface string) string {
	return fmt.Sprintf("ipconfig getoption %s domain_name_server", iface)
}
