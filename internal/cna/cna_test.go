package cna

import (
	"context"
	"runtime"
	"testing"
)

func TestStateString(t *testing.T) {
	t.Parallel()
	cases := map[State]string{
		StateEnabled:  "enabled",
		StateDisabled: "disabled",
		StateUnknown:  "unknown",
		State(99):     "unknown",
	}
	for state, want := range cases {
		if got := state.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", state, got, want)
		}
	}
}

func TestUnsupportedOffMacOS(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("CNA control is supported on macOS")
	}
	ctx := context.Background()
	if err := Disable(ctx); err == nil {
		t.Error("Disable should fail off macOS")
	}
	if err := Enable(ctx); err == nil {
		t.Error("Enable should fail off macOS")
	}
	if _, err := Status(ctx); err == nil {
		t.Error("Status should fail off macOS")
	}
}
