package browser

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func testOptions() Options {
	return Options{
		ProxyAddr:  "127.0.0.1:1666",
		ProfileDir: "/tmp/foyer-xyz",
		StartURL:   "http://example.com",
	}
}

func TestChromeArgsContainsLoadBearingFlags(t *testing.T) {
	t.Parallel()
	args := chromeArgs(testOptions())

	want := []string{
		"--user-data-dir=/tmp/foyer-xyz",
		"--proxy-server=socks5://127.0.0.1:1666",
		"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE localhost",
		"--incognito",
		"http://example.com",
	}
	for _, w := range want {
		if !slices.Contains(args, w) {
			t.Errorf("chromeArgs missing %q; got %v", w, args)
		}
	}
	if args[len(args)-1] != "http://example.com" {
		t.Errorf("start URL must be last arg; got %v", args)
	}
}

func TestCommandExec(t *testing.T) {
	t.Parallel()
	b := &Browser{Name: "chromium", target: "/usr/bin/chromium", kind: kindExec}
	cmd := b.Command(context.Background(), testOptions())

	if cmd.Args[0] != "/usr/bin/chromium" {
		t.Errorf("argv0 = %q, want /usr/bin/chromium", cmd.Args[0])
	}
	if !slices.Contains(cmd.Args, "--incognito") {
		t.Errorf("missing chrome flags in %v", cmd.Args)
	}
}

func TestCommandMacOpen(t *testing.T) {
	t.Parallel()
	b := &Browser{Name: "Google Chrome", target: "Google Chrome", kind: kindMacOpen}
	cmd := b.Command(context.Background(), testOptions())

	prefix := []string{"-n", "-W", "-a", "Google Chrome", "--args"}
	if !slices.Equal(cmd.Args[1:1+len(prefix)], prefix) {
		t.Errorf("open prefix = %v, want %v", cmd.Args[1:1+len(prefix)], prefix)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "--proxy-server=socks5://127.0.0.1:1666") {
		t.Errorf("missing proxy flag in %v", cmd.Args)
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()
	// Detect's result depends on the host; assert only that it never panics and
	// returns a coherent (browser,err) pair.
	b, err := Detect()
	if err != nil && b != nil {
		t.Errorf("got both browser %+v and error %v", b, err)
	}
	if err == nil && (b == nil || b.Name == "") {
		t.Errorf("nil error but invalid browser %+v", b)
	}
}
