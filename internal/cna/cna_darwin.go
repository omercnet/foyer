//go:build darwin

package cna

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// captivePrefs is the SystemConfiguration preference domain that controls the
// Captive Network Assistant pop-up. Its `Active` key gates the helper.
const (
	captivePrefs = "/Library/Preferences/SystemConfiguration/com.apple.captive.control"
	activeKey    = "Active"
)

// setActive writes the Active key for the captive-control domain. Writing to
// /Library requires root, so we elevate with sudo when necessary; the user
// sees the standard interactive password prompt in their terminal.
func setActive(ctx context.Context, active bool) error {
	value := "false"
	if active {
		value = "true"
	}
	cmd := elevated(ctx, "defaults", "write", captivePrefs, activeKey, "-bool", value)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cna: write %s %s: %w", captivePrefs, activeKey, err)
	}
	return nil
}

func status(ctx context.Context) (State, error) {
	out, err := exec.CommandContext(ctx, "defaults", "read", captivePrefs, activeKey).Output()
	if err != nil {
		// A missing key means the default (enabled) is in effect.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return StateEnabled, nil
		}
		return StateUnknown, fmt.Errorf("cna: read %s %s: %w", captivePrefs, activeKey, err)
	}
	switch strings.TrimSpace(string(out)) {
	case "0":
		return StateDisabled, nil
	case "1":
		return StateEnabled, nil
	default:
		return StateUnknown, nil
	}
}

// elevated builds cmd, prefixing sudo when the process is not already root.
func elevated(ctx context.Context, name string, args ...string) *exec.Cmd {
	if os.Geteuid() == 0 {
		return exec.CommandContext(ctx, name, args...)
	}
	return exec.CommandContext(ctx, "sudo", append([]string{name}, args...)...)
}
