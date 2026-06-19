package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/omercnet/foyer/internal/config"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestRunWiresProxyAndBrowser drives the whole flow using a shell command as a
// stand-in browser. The "browser" records the $PROXY address it was handed; we
// assert the proxy was listening at that address.
func TestRunWiresProxyAndBrowser(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "proxy.txt")
	cfg := config.Config{
		SOCKS5Addr: "127.0.0.1:0",
		Interface:  "lo",
		DHCPDNS:    "echo 192.0.2.53",
		Browser:    "printf %s \"$PROXY\" > " + out,
		StartURL:   config.DefaultStartURL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Run(ctx, cfg, quietLogger()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("browser did not record proxy address: %v", err)
	}
	addr := strings.TrimSpace(string(data))
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("PROXY %q is not host:port: %v", addr, err)
	}
	if host != "127.0.0.1" || port == "0" || port == "" {
		t.Errorf("unexpected proxy address %q", addr)
	}
}

func TestRunFailsOnDiscoveryError(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		SOCKS5Addr: "127.0.0.1:0",
		Interface:  "lo",
		DHCPDNS:    "exit 1",
		Browser:    "true",
		StartURL:   config.DefaultStartURL,
	}
	if err := Run(context.Background(), cfg, quietLogger()); err == nil {
		t.Error("expected error when DHCP discovery fails")
	}
}

func TestRunFailsOnBadListenAddr(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		SOCKS5Addr: "256.256.256.256:99999",
		Interface:  "lo",
		DHCPDNS:    "echo 192.0.2.53",
		Browser:    "true",
		StartURL:   config.DefaultStartURL,
	}
	if err := Run(context.Background(), cfg, quietLogger()); err == nil {
		t.Error("expected error for invalid listen address")
	}
}
