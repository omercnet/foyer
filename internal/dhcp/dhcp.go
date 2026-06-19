// Package dhcp discovers the DNS server advertised by the current network's
// DHCP lease. It does not implement a DHCP client; instead it shells out to the
// platform's existing network tooling (the OS already holds the lease) and
// extracts the resolver address. The platform-specific command lives in the
// build-tagged files; the parsing here is shared and unit-tested everywhere.
package dhcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ipv4Pattern matches a dotted-quad anywhere in a line of command output.
var ipv4Pattern = regexp.MustCompile(`\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b`)

// Discover runs shellCommand via /bin/sh and returns the first IPv4 address in
// its output, which is taken to be the DHCP-advertised DNS server.
func Discover(ctx context.Context, shellCommand string) (string, error) {
	if strings.TrimSpace(shellCommand) == "" {
		return "", errors.New("dhcp: empty discovery command")
	}
	//nolint:gosec // running the configured DHCP-DNS discovery command is the feature
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", shellCommand)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("dhcp: command %q failed: %w: %s",
				shellCommand, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("dhcp: command %q failed: %w", shellCommand, err)
	}
	ip, err := parseFirstIPv4(string(out))
	if err != nil {
		return "", fmt.Errorf("dhcp: command %q: %w", shellCommand, err)
	}
	return ip, nil
}

// parseFirstIPv4 returns the first syntactically valid IPv4 address found in s.
func parseFirstIPv4(s string) (string, error) {
	for _, m := range ipv4Pattern.FindAllStringSubmatch(s, -1) {
		if isValidIPv4Octets(m[1:]) {
			return m[0], nil
		}
	}
	return "", errors.New("no IPv4 address found in command output")
}

func isValidIPv4Octets(octets []string) bool {
	for _, o := range octets {
		if len(o) > 1 && o[0] == '0' { // reject leading zeros like 010
			return false
		}
		n := 0
		for _, c := range o {
			n = n*10 + int(c-'0')
		}
		if n > 255 {
			return false
		}
	}
	return true
}

// parseRouteGetDefault extracts the interface name from the output of
// `route -n get default` on macOS/BSD (the line `  interface: en0`).
func parseRouteGetDefault(output string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if name, ok := strings.CutPrefix(line, "interface:"); ok {
			if iface := strings.TrimSpace(name); iface != "" {
				return iface, nil
			}
		}
	}
	return "", errors.New("no default interface in route output")
}

// parseProcNetRoute extracts the default-route interface from the contents of
// /proc/net/route on Linux (the entry whose hex Destination is 00000000).
func parseProcNetRoute(content string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	if scanner.Scan() { // skip header
		_ = scanner.Text()
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		if fields[1] == "00000000" {
			return fields[0], nil
		}
	}
	return "", errors.New("no default route in /proc/net/route")
}
