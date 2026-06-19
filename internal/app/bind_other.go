//go:build !linux

package app

import (
	"fmt"
	"runtime"
	"syscall"
)

// deviceControl is unsupported off Linux; binding sockets to a device is a
// Linux-specific capability (SO_BINDTODEVICE).
func deviceControl(string) func(network, address string, c syscall.RawConn) error {
	return func(string, string, syscall.RawConn) error {
		return fmt.Errorf("bind-device is only supported on Linux, not %s", runtime.GOOS)
	}
}
