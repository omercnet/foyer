// Package config defines Foyer's configuration: a small TOML file whose every
// field is optional. Anything left unset is auto-detected at runtime, so the
// zero-config path works out of the box.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Default values used when the corresponding field is unset.
const (
	// DefaultSOCKS5Addr binds to an ephemeral loopback port, avoiding the
	// port-in-use failures of a fixed port.
	DefaultSOCKS5Addr = "127.0.0.1:0"
	// DefaultStartURL is a plain-HTTP page that reliably triggers a portal
	// redirect without leaking which site the user intended to visit.
	DefaultStartURL = "http://example.com"
)

// Config holds Foyer's settings. Empty string / false means "auto-detect".
type Config struct {
	// SOCKS5Addr is the loopback listen address for the proxy.
	SOCKS5Addr string `toml:"socks5-addr"`
	// Interface is the network interface to query and (optionally) bind to.
	// Empty means detect the default-route interface.
	Interface string `toml:"interface"`
	// DHCPDNS overrides the shell command used to discover the DHCP DNS
	// server. Empty means use the platform default command.
	DHCPDNS string `toml:"dhcp-dns"`
	// Browser overrides browser launching with a shell command. The proxy
	// address is exported to it as $PROXY. Empty means auto-detect.
	Browser string `toml:"browser"`
	// StartURL is the first page the browser opens.
	StartURL string `toml:"start-url"`
	// BindDevice binds outbound sockets to Interface (needs CAP_NET_RAW/root,
	// Linux only). Useful when address spaces collide across interfaces.
	BindDevice bool `toml:"bind-device"`
}

// Default returns a Config populated with built-in defaults.
func Default() Config {
	return Config{
		SOCKS5Addr: DefaultSOCKS5Addr,
		StartURL:   DefaultStartURL,
	}
}

// Load reads the TOML file at path, layering it over the defaults. A missing
// file is not an error: the defaults are returned so Foyer runs with no config.
func Load(path string) (Config, error) {
	cfg := Default()
	//nolint:gosec // path is the user's own config file location
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := Parse(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return cfg, nil
}

// Parse decodes TOML into cfg and re-applies defaults for any field the file
// left blank. cfg should be seeded with Default() before calling.
func Parse(data []byte, cfg *Config) error {
	md, err := toml.Decode(string(data), cfg)
	if err != nil {
		return err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return fmt.Errorf("unknown keys: %v", undecoded)
	}
	if cfg.SOCKS5Addr == "" {
		cfg.SOCKS5Addr = DefaultSOCKS5Addr
	}
	if cfg.StartURL == "" {
		cfg.StartURL = DefaultStartURL
	}
	return nil
}

// DefaultPath returns the conventional config location:
// $XDG_CONFIG_HOME/foyer/config.toml, falling back to ~/.config/foyer/config.toml.
func DefaultPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("config: locate home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "foyer", "config.toml"), nil
}
