# 🪞 Foyer

> A privacy-first captive-portal browser.

[![CI](https://github.com/omercnet/foyer/actions/workflows/ci.yml/badge.svg)](https://github.com/omercnet/foyer/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/omercnet/foyer?sort=semver)](https://github.com/omercnet/foyer/releases)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/omercnet/foyer/badge)](https://scorecard.dev/viewer/?uri=github.com/omercnet/foyer)
[![Go Reference](https://pkg.go.dev/badge/github.com/omercnet/foyer.svg)](https://pkg.go.dev/github.com/omercnet/foyer)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A **foyer** is the small entryway you pass through before stepping inside — you
deal with the lock at the door and leave the mud (and the tracking cookies)
outside. Foyer launches a **throwaway, incognito browser** just to get you
through a WiFi captive portal, so your real browser — full of your sessions and
cookies — never touches the hotel/airport/café login page.

---

## Why

Captive portals hijack DNS: the network's DHCP server hands out its own resolver
and redirects everything to a login page until you authenticate. But if you run
encrypted/pinned DNS (DoH, a custom resolver, a VPN), that hijack never reaches
you and **the network just looks broken**. macOS pops its own *Captive Network
Assistant* — a stripped-down mini-browser — but you might prefer not to trust it,
and it can't help on every platform.

Foyer solves this **without changing any global setting**:

1. It discovers the captive network's DHCP-advertised DNS server.
2. It runs a tiny local **SOCKS5 proxy** that resolves DNS **through that server**.
3. It opens a **disposable, incognito** Chromium window routed through the proxy.

Nothing about your system DNS or normal browser changes. Close the window and
the throwaway profile is deleted.

## Features

- **Zero-config.** Auto-detects your default interface, the DHCP DNS server, and
  an installed Chromium-family browser (Chrome, Chromium, Brave, Edge).
- **Privacy-first.** Loopback-only proxy, incognito, throwaway profile deleted on
  exit, and your real browser is never involved.
- **Minimal supply chain.** Pure Go, one dependency, a self-contained SOCKS5
  proxy (RFC 1928) so the network path stays auditable.
- **Graceful.** Clean SIGINT/SIGTERM shutdown; an ephemeral proxy port avoids
  "address already in use".
- **macOS integration.** Toggle the built-in Captive Network Assistant so Foyer
  is the only thing that handles portals.

## Install

### Go

```sh
go install github.com/omercnet/foyer@latest
```

The binary is named `foyer`.

### Pre-built binaries

Download from [Releases](https://github.com/omercnet/foyer/releases).
Each release includes a CycloneDX SBOM and a cosign-signed `checksums.txt`.

Verify the checksums signature (keyless / Sigstore):

```sh
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/omercnet/foyer/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
```

## Usage

```sh
foyer                # detect everything and open the captive-portal browser
foyer status         # diagnose: show detected interface, DNS, browser, CNA state
foyer disable-cna    # macOS: stop the system Captive Network Assistant pop-up
foyer enable-cna     # macOS: restore it
foyer -verbose       # debug logging
foyer version
```

Typical flow: join the WiFi, run `foyer`, log in on the portal page that opens,
then close the window — done.

### macOS: make Foyer the only portal handler

macOS auto-launches its own Captive Network Assistant, which races with Foyer.
Disable it once:

```sh
foyer disable-cna    # runs: sudo defaults write \
                     #   /Library/Preferences/SystemConfiguration/com.apple.captive.control Active -bool false
```

Now joining a portal network won't pop the system helper; run `foyer` instead.
Reverse it any time with `foyer enable-cna`.

## Configuration

Foyer needs **no config file**. To override defaults, copy
[`foyer.example.toml`](foyer.example.toml) to
`~/.config/foyer/config.toml` (or `$XDG_CONFIG_HOME/foyer/config.toml`) and edit.

| Key           | Default                                   | Meaning                                                        |
| ------------- | ----------------------------------------- | -------------------------------------------------------------- |
| `socks5-addr` | `127.0.0.1:0` (ephemeral)                 | Loopback listen address for the proxy.                         |
| `interface`   | default-route interface                   | Interface to query / bind to.                                  |
| `dhcp-dns`    | platform default                          | Shell command; first IPv4 in its output is the captive DNS.    |
| `browser`     | auto-detected                             | Shell command to launch the browser (`$PROXY` is exported).    |
| `start-url`   | `http://example.com`                      | First page; any plain-HTTP URL triggers the portal redirect.   |
| `bind-device` | `false`                                   | Bind sockets to `interface` (Linux, needs `CAP_NET_RAW`).      |

## How it works

```
            ┌─────────────────────── foyer ───────────────────────┐
            │                                                      │
  DHCP ───▶ │  discover DNS    SOCKS5 proxy (loopback, no auth)    │
  lease     │  (ipconfig/      ├─ CONNECT example.com ───┐         │
            │   resolvectl)    │   resolve via captive DNS│         │
            │        │         │   dial the result        ▼         │
            │        └────────▶│                    captive network │
            │                  └─ pipe bytes ◀───────────┘         │
            │                         ▲                            │
            │   incognito browser ────┘  (--proxy-server=socks5,   │
            │   throwaway profile        --host-resolver-rules)    │
            └──────────────────────────────────────────────────────┘
```

The two load-bearing tricks:

- **Proxy-side DNS.** The upstream resolver uses Go's pure-Go resolver with a
  `Dial` override so every query goes to the captive DNS server — see
  [`internal/proxy/resolver.go`](internal/proxy/resolver.go).
- **No local resolution in the browser.** Chrome is launched with
  `--host-resolver-rules="MAP * ~NOTFOUND , EXCLUDE localhost"` plus
  `--proxy-server="socks5://…"`, so it hands hostnames to the proxy instead of
  resolving them itself.

IPv4 only — captive portals are an IPv4-era concern.

## Development

```sh
go test -race -cover ./...   # unit + integration tests
go vet ./...
golangci-lint run            # strict lint (see .golangci.yml)
goreleaser build --snapshot --clean
```

CI runs lint, a `-race` test matrix (Ubuntu + macOS), `govulncheck`, CodeQL, and
OpenSSF Scorecard on a hardened runner. Releases are automated with
**release-please** (Conventional Commits → version PR) and **GoReleaser**
(cross-platform binaries, SBOM, cosign signatures).

## Credits

Foyer is a from-scratch Go reimplementation **inspired by
[captive-browser](https://github.com/FiloSottile/captive-browser) by
[Filippo Valsorda](https://filippo.io)**. The original tool pioneered the
"resolve through DHCP DNS via a local SOCKS5 proxy" approach; all credit for the
idea goes to him. Foyer reimplements it independently with a self-contained
proxy, zero-config detection, graceful shutdown, and macOS CNA integration.

## License

[MIT](LICENSE).
