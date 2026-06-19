// Package app wires Foyer's pieces together: discover the captive network's DNS
// server, run a SOCKS5 proxy that resolves through it, and launch a throwaway
// browser pointed at the captive portal. When the browser closes (or a signal
// arrives) everything is torn down and the temporary profile is deleted.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/omercnet/foyer/internal/browser"
	"github.com/omercnet/foyer/internal/config"
	"github.com/omercnet/foyer/internal/dhcp"
	"github.com/omercnet/foyer/internal/proxy"
)

// Run executes Foyer's main flow with the given configuration. It blocks until
// the browser exits or ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	iface, err := resolveInterface(ctx, cfg)
	if err != nil {
		return err
	}

	dnsCmd := cfg.DHCPDNS
	if dnsCmd == "" {
		dnsCmd = dhcp.DefaultCommand(iface)
		if dnsCmd == "" {
			return fmt.Errorf("no dhcp-dns command configured and no default for this platform")
		}
	}

	upstream, err := dhcp.Discover(ctx, dnsCmd)
	if err != nil {
		return err
	}
	logger.Info("discovered captive DNS server", "interface", iface, "dns", upstream)

	dialer := &net.Dialer{}
	if cfg.BindDevice {
		if iface == "" {
			return errors.New("bind-device set but no interface resolved")
		}
		dialer.Control = deviceControl(iface)
		logger.Info("binding sockets to device", "interface", iface)
	}

	ln, err := net.Listen("tcp", cfg.SOCKS5Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.SOCKS5Addr, err)
	}
	proxyAddr := ln.Addr().String()
	logger.Info("SOCKS5 proxy listening", "addr", proxyAddr)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	srv := &proxy.Server{
		Resolver: proxy.NewUpstreamResolver(upstream, dialer),
		Dialer:   dialer,
		Logger:   logger,
	}
	proxyErr := make(chan error, 1)
	go func() { proxyErr <- srv.Serve(runCtx, ln) }()

	browserErr := launchBrowser(runCtx, cfg, proxyAddr, logger)

	// Browser is done (or was killed by ctx cancellation); stop the proxy.
	cancel()
	<-proxyErr

	if browserErr != nil && runCtx.Err() == nil {
		return browserErr
	}
	logger.Info("browser closed, shutting down")
	return nil
}

// resolveInterface returns the configured interface or auto-detects the
// default-route interface.
func resolveInterface(ctx context.Context, cfg config.Config) (string, error) {
	if cfg.Interface != "" {
		return cfg.Interface, nil
	}
	iface, err := dhcp.DefaultInterface(ctx)
	if err != nil {
		return "", fmt.Errorf("detect default interface (set `interface` in config): %w", err)
	}
	return iface, nil
}

// launchBrowser starts the browser and blocks until it exits. A custom browser
// command from config takes precedence; otherwise a Chromium-family browser is
// auto-detected and launched with a throwaway profile that is deleted on exit.
func launchBrowser(ctx context.Context, cfg config.Config, proxyAddr string, logger *slog.Logger) error {
	var cmd *exec.Cmd
	if cfg.Browser != "" {
		logger.Info("launching browser via configured command")
		//nolint:gosec // running the user's own configured browser command is the feature
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", cfg.Browser)
	} else {
		b, err := browser.Detect()
		if err != nil {
			return err
		}
		profileDir, err := os.MkdirTemp("", "foyer-profile-")
		if err != nil {
			return fmt.Errorf("create throwaway profile: %w", err)
		}
		defer func() {
			if rmErr := os.RemoveAll(profileDir); rmErr != nil {
				logger.Warn("failed to remove throwaway profile", "dir", profileDir, "err", rmErr)
			}
		}()
		logger.Info("launching browser", "browser", b.Name, "profile", filepath.Base(profileDir))
		cmd = b.Command(ctx, browser.Options{
			ProxyAddr:  proxyAddr,
			ProfileDir: profileDir,
			StartURL:   cfg.StartURL,
		})
	}

	cmd.Env = append(os.Environ(), "PROXY="+proxyAddr)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil // killed by our own shutdown
		}
		return fmt.Errorf("browser exited with error: %w", err)
	}
	return nil
}
