//go:build !darwin && !linux

package dhcp

import (
	"context"
	"runtime"
)

// DefaultInterface is unsupported on this platform; callers must supply the
// interface explicitly via configuration.
func DefaultInterface(_ context.Context) (string, error) {
	return "", &UnsupportedError{GOOS: runtime.GOOS}
}

// DefaultCommand has no built-in default on this platform; callers must supply
// a dhcp-dns command via configuration.
func DefaultCommand(string) string { return "" }

// UnsupportedError indicates the current OS has no built-in discovery support.
type UnsupportedError struct{ GOOS string }

func (e *UnsupportedError) Error() string {
	return "dhcp: automatic discovery unsupported on " + e.GOOS + "; set interface and dhcp-dns in config"
}
