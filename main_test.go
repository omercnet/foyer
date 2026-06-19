package main

import (
	"context"
	"testing"
)

func TestDispatchVersion(t *testing.T) {
	t.Parallel()
	if err := dispatch(context.Background(), []string{"version"}); err != nil {
		t.Errorf("version command: %v", err)
	}
	if err := dispatch(context.Background(), []string{"-version"}); err != nil {
		t.Errorf("-version flag: %v", err)
	}
}

func TestDispatchHelp(t *testing.T) {
	t.Parallel()
	if err := dispatch(context.Background(), []string{"help"}); err != nil {
		t.Errorf("help command: %v", err)
	}
}

func TestDispatchUnknownCommand(t *testing.T) {
	t.Parallel()
	if err := dispatch(context.Background(), []string{"frobnicate"}); err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestDispatchInvalidFlag(t *testing.T) {
	t.Parallel()
	if err := dispatch(context.Background(), []string{"-nonsense"}); err == nil {
		t.Error("expected error for invalid flag")
	}
}

func TestOrNone(t *testing.T) {
	t.Parallel()
	if got := orNone(""); got != "(none)" {
		t.Errorf("orNone(\"\") = %q", got)
	}
	if got := orNone("x"); got != "x" {
		t.Errorf("orNone(\"x\") = %q", got)
	}
}
