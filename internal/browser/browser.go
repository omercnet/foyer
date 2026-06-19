// Package browser detects an installed Chromium-family browser and builds the
// command that launches it in a throwaway, proxied, incognito session pointed
// at the captive portal.
package browser

import (
	"context"
	"os/exec"
)

// kind distinguishes how a browser is launched.
type kind int

const (
	kindExec    kind = iota // run the binary directly (Linux)
	kindMacOpen             // run via macOS `open -n -W -a`
)

// Browser describes a detected browser and how to launch it.
type Browser struct {
	// Name is a human-readable label, e.g. "Google Chrome".
	Name string

	target string // binary path (kindExec) or app name (kindMacOpen)
	kind   kind
}

// Options configures a browser launch.
type Options struct {
	// ProxyAddr is the host:port of the local SOCKS5 proxy.
	ProxyAddr string
	// ProfileDir is a throwaway user-data directory, deleted after exit.
	ProfileDir string
	// StartURL is the first page to load; any plain-HTTP URL triggers the
	// portal's redirect to its login page.
	StartURL string
}

// chromeArgs returns the Chromium flags shared by every platform.
//
// The two load-bearing flags are --proxy-server (route traffic through the
// proxy) and --host-resolver-rules (forbid local DNS so hostnames are sent to
// the proxy and resolved against the captive DNS server). The rest keep the
// session ephemeral and unobtrusive.
func chromeArgs(opt Options) []string {
	return []string{
		"--user-data-dir=" + opt.ProfileDir,
		"--proxy-server=socks5://" + opt.ProxyAddr,
		"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE localhost",
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		"--incognito",
		opt.StartURL,
	}
}

// Command builds the exec.Cmd that launches the browser. The command blocks
// until the browser window is closed (macOS via `open -W`, Linux because the
// child process is the browser itself).
func (b *Browser) Command(ctx context.Context, opt Options) *exec.Cmd {
	args := chromeArgs(opt)
	if b.kind == kindMacOpen {
		full := append([]string{"-n", "-W", "-a", b.target, "--args"}, args...)
		//nolint:gosec // launching the detected browser is the feature
		return exec.CommandContext(ctx, "open", full...)
	}
	//nolint:gosec // launching the detected browser binary is the feature
	return exec.CommandContext(ctx, b.target, args...)
}
