//go:build linux

package app

import (
	"fmt"
	"syscall"
)

// deviceControl returns a net.Dialer Control hook that binds each socket to the
// named interface via SO_BINDTODEVICE. Requires CAP_NET_RAW (typically root).
func deviceControl(iface string) func(network, address string, c syscall.RawConn) error {
	return func(_, _ string, c syscall.RawConn) error {
		var setErr error
		ctrlErr := c.Control(func(fd uintptr) {
			setErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface)
		})
		if ctrlErr != nil {
			return fmt.Errorf("bind-device %q: %w", iface, ctrlErr)
		}
		if setErr != nil {
			return fmt.Errorf("bind-device %q (needs CAP_NET_RAW/root): %w", iface, setErr)
		}
		return nil
	}
}
