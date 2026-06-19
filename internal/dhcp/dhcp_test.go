package dhcp

import (
	"context"
	"testing"
)

func TestParseFirstIPv4(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{"macos ipconfig", "192.168.1.1\n", "192.168.1.1", true},
		{"nmcli line", "IP4.DNS[1]:                             10.0.0.53", "10.0.0.53", true},
		{"resolvectl", "Link 3 (wlan0): 1.1.1.1 8.8.8.8", "1.1.1.1", true},
		{"embedded in text", "Current DNS Server: 172.16.0.1 (reachable)", "172.16.0.1", true},
		{"skip invalid octet then take valid", "999.999.999.999 then 8.8.4.4", "8.8.4.4", true},
		{"leading zero rejected", "010.0.0.1 192.0.2.1", "192.0.2.1", true},
		{"no address", "no servers configured", "", false},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseFirstIPv4(tt.input)
			if tt.ok != (err == nil) {
				t.Fatalf("parseFirstIPv4(%q) err = %v, want ok=%v", tt.input, err, tt.ok)
			}
			if got != tt.want {
				t.Errorf("parseFirstIPv4(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRouteGetDefault(t *testing.T) {
	t.Parallel()
	const out = `   route to: default
destination: default
       mask: default
    gateway: 192.168.1.1
  interface: en0
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING>
`
	got, err := parseRouteGetDefault(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "en0" {
		t.Errorf("got %q, want en0", got)
	}

	if _, err := parseRouteGetDefault("no interface here"); err == nil {
		t.Error("expected error for output without interface")
	}
}

func TestParseProcNetRoute(t *testing.T) {
	t.Parallel()
	const content = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0102A8C0	0003	0	0	100	00000000	0	0	0
eth0	0002A8C0	00000000	0001	0	0	100	00FFFFFF	0	0	0
`
	got, err := parseProcNetRoute(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "eth0" {
		t.Errorf("got %q, want eth0", got)
	}

	if _, err := parseProcNetRoute("Iface\tDestination\n"); err == nil {
		t.Error("expected error when no default route present")
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()
	t.Run("parses command output", func(t *testing.T) {
		t.Parallel()
		got, err := Discover(context.Background(), "echo 'nameserver 192.0.2.53'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "192.0.2.53" {
			t.Errorf("got %q, want 192.0.2.53", got)
		}
	})
	t.Run("empty command", func(t *testing.T) {
		t.Parallel()
		if _, err := Discover(context.Background(), "   "); err == nil {
			t.Error("expected error for empty command")
		}
	})
	t.Run("no address in output", func(t *testing.T) {
		t.Parallel()
		if _, err := Discover(context.Background(), "echo nothing-here"); err == nil {
			t.Error("expected error when output has no IP")
		}
	})
	t.Run("command failure surfaces stderr", func(t *testing.T) {
		t.Parallel()
		if _, err := Discover(context.Background(), "echo boom >&2; exit 3"); err == nil {
			t.Error("expected error for failing command")
		}
	})
}
