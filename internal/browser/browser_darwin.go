//go:build darwin

package browser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// macCandidates lists known Chromium-family apps in preference order.
var macCandidates = []string{
	"Google Chrome",
	"Chromium",
	"Brave Browser",
	"Microsoft Edge",
}

// Detect finds the first installed Chromium-family browser, searching the
// system and per-user Applications folders.
func Detect() (*Browser, error) {
	dirs := []string{"/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}
	for _, name := range macCandidates {
		for _, dir := range dirs {
			if _, err := os.Stat(filepath.Join(dir, name+".app")); err == nil {
				return &Browser{Name: name, target: name, kind: kindMacOpen}, nil
			}
		}
	}
	return nil, fmt.Errorf("browser: no Chromium-family browser found in %v: %w", dirs, errors.ErrUnsupported)
}
