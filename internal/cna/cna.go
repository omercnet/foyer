// Package cna toggles the macOS Captive Network Assistant (CNA), the system
// mini-browser that auto-pops on captive portals. Disabling it ensures portals
// are handled by this tool's private browser rather than Apple's stripped-down,
// cookie-bearing helper. All operations are no-ops with a clear error on
// non-macOS platforms.
package cna

import "context"

// State describes whether the Captive Network Assistant is active.
type State int

const (
	// StateUnknown means the setting could not be determined.
	StateUnknown State = iota
	// StateEnabled means the CNA pop-up is active (macOS default).
	StateEnabled
	// StateDisabled means the CNA pop-up has been suppressed.
	StateDisabled
)

func (s State) String() string {
	switch s {
	case StateEnabled:
		return "enabled"
	case StateDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// Disable suppresses the Captive Network Assistant. It requires root and will
// transparently elevate via sudo when not already privileged.
func Disable(ctx context.Context) error { return setActive(ctx, false) }

// Enable restores the Captive Network Assistant to its default behavior.
func Enable(ctx context.Context) error { return setActive(ctx, true) }

// Status reports the current Captive Network Assistant state.
func Status(ctx context.Context) (State, error) { return status(ctx) }
