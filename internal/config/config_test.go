package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAppliesDefaults(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if err := Parse([]byte(`start-url = "http://neverssl.com"`), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StartURL != "http://neverssl.com" {
		t.Errorf("StartURL = %q, want override", cfg.StartURL)
	}
	if cfg.SOCKS5Addr != DefaultSOCKS5Addr {
		t.Errorf("SOCKS5Addr = %q, want default %q", cfg.SOCKS5Addr, DefaultSOCKS5Addr)
	}
}

func TestParseFull(t *testing.T) {
	t.Parallel()
	const data = `
socks5-addr = "127.0.0.1:1666"
interface = "en0"
dhcp-dns = "echo 1.2.3.4"
browser = "open -a Foo"
start-url = "http://example.net"
bind-device = true
`
	cfg := Default()
	if err := Parse([]byte(data), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Config{
		SOCKS5Addr: "127.0.0.1:1666",
		Interface:  "en0",
		DHCPDNS:    "echo 1.2.3.4",
		Browser:    "open -a Foo",
		StartURL:   "http://example.net",
		BindDevice: true,
	}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestParseRejectsUnknownKeys(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if err := Parse([]byte(`bogus-key = true`), &cfg); err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestParseEmptyResetsToDefaults(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if err := Parse([]byte(`socks5-addr = ""`+"\n"+`start-url = ""`), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SOCKS5Addr != DefaultSOCKS5Addr || cfg.StartURL != DefaultStartURL {
		t.Errorf("blank values not reset to defaults: %+v", cfg)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != Default() {
		t.Errorf("got %+v, want defaults", cfg)
	}
}

func TestLoadFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`interface = "wlan0"`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Interface != "wlan0" {
		t.Errorf("Interface = %q, want wlan0", cfg.Interface)
	}
}

func TestDefaultPathUsesXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "/tmp/xdg/foyer/config.toml"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
