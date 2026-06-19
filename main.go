// Command foyer is a privacy-first captive-portal browser. It launches a
// throwaway, incognito Chromium-family window that resolves DNS through the
// captive network's own DHCP-advertised resolver — so portals load and log you
// in — while your real browser, cookies, and system DNS stay untouched.
//
// Inspired by FiloSottile/captive-browser by Filippo Valsorda.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/omercnet/foyer/internal/app"
	"github.com/omercnet/foyer/internal/browser"
	"github.com/omercnet/foyer/internal/cna"
	"github.com/omercnet/foyer/internal/config"
	"github.com/omercnet/foyer/internal/dhcp"
)

// Build information, overridden at release time via -ldflags by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `Foyer — a privacy-first captive-portal browser.

Usage:
  foyer [flags] [command]

Commands:
  run           Launch the captive-portal browser (default).
  status        Diagnose: show detected interface, DNS, browser, and CNA state.
  disable-cna   Disable the macOS Captive Network Assistant (needs sudo).
  enable-cna    Re-enable the macOS Captive Network Assistant (needs sudo).
  version       Print version information.

Flags:
  -config PATH  Path to a TOML config file (default: $XDG_CONFIG_HOME/foyer/config.toml).
  -verbose      Enable debug logging.
  -version      Print version information and exit.
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "foyer: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return dispatch(ctx, os.Args[1:])
}

func dispatch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("foyer", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	configPath := fs.String("config", "", "path to TOML config file")
	verbose := fs.Bool("verbose", false, "enable debug logging")
	showVersion := fs.Bool("version", false, "print version information and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logger := newLogger(*verbose)

	if *showVersion {
		printVersion()
		return nil
	}

	command := "run"
	if fs.NArg() > 0 {
		command = fs.Arg(0)
	}

	switch command {
	case "run":
		return runCommand(ctx, *configPath, logger)
	case "status":
		return statusCommand(ctx, *configPath, logger)
	case "disable-cna":
		return cna.Disable(ctx)
	case "enable-cna":
		return cna.Enable(ctx)
	case "version":
		printVersion()
		return nil
	case "help":
		fmt.Print(usage)
		return nil
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func newLogger(verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// loadConfig resolves the config path (explicit flag or default location) and
// loads it, returning defaults when no file exists.
func loadConfig(path string) (config.Config, error) {
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return config.Config{}, err
		}
	}
	return config.Load(path)
}

func runCommand(ctx context.Context, configPath string, logger *slog.Logger) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	return app.Run(ctx, cfg, logger)
}

// statusCommand prints a best-effort diagnostic of what Foyer would do, without
// launching anything. Each probe degrades gracefully so one failure still shows
// the rest.
func statusCommand(ctx context.Context, configPath string, _ *slog.Logger) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	fmt.Printf("Foyer %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)

	iface := cfg.Interface
	if iface == "" {
		if detected, derr := dhcp.DefaultInterface(ctx); derr == nil {
			iface = detected
		} else {
			iface = "(undetected: " + derr.Error() + ")"
		}
	}
	fmt.Printf("Interface:   %s\n", iface)

	dnsCmd := cfg.DHCPDNS
	if dnsCmd == "" {
		dnsCmd = dhcp.DefaultCommand(iface)
	}
	fmt.Printf("DHCP cmd:    %s\n", orNone(dnsCmd))
	if dnsCmd != "" {
		if dns, derr := dhcp.Discover(ctx, dnsCmd); derr == nil {
			fmt.Printf("Captive DNS: %s\n", dns)
		} else {
			fmt.Printf("Captive DNS: (failed: %s)\n", derr)
		}
	}

	if cfg.Browser != "" {
		fmt.Printf("Browser:     (configured command)\n")
	} else if b, berr := browser.Detect(); berr == nil {
		fmt.Printf("Browser:     %s\n", b.Name)
	} else {
		fmt.Printf("Browser:     (none found: %s)\n", berr)
	}

	if state, serr := cna.Status(ctx); serr == nil {
		fmt.Printf("CNA:         %s\n", state)
	} else {
		fmt.Printf("CNA:         %s\n", serr)
	}
	return nil
}

func printVersion() {
	fmt.Printf("foyer %s\ncommit: %s\nbuilt:  %s\n", version, commit, date)
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
