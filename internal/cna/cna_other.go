//go:build !darwin

package cna

import (
	"context"
	"fmt"
	"runtime"
)

// errUnsupported is returned by every operation off macOS.
var errUnsupported = fmt.Errorf("cna: Captive Network Assistant control is only available on macOS, not %s", runtime.GOOS)

func setActive(context.Context, bool) error { return errUnsupported }

func status(context.Context) (State, error) { return StateUnknown, errUnsupported }
