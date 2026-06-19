//go:build !darwin

package browser

import (
	"errors"
	"fmt"
	"os/exec"
)

// unixCandidates lists known Chromium-family executables in preference order.
var unixCandidates = []string{
	"google-chrome",
	"google-chrome-stable",
	"chromium",
	"chromium-browser",
	"brave-browser",
	"microsoft-edge",
}

// Detect finds the first Chromium-family browser available on PATH.
func Detect() (*Browser, error) {
	for _, name := range unixCandidates {
		if path, err := exec.LookPath(name); err == nil {
			return &Browser{Name: name, target: path, kind: kindExec}, nil
		}
	}
	return nil, fmt.Errorf("browser: no Chromium-family browser found on PATH (%v): %w", unixCandidates, errors.ErrUnsupported)
}
