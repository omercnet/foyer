# Security Policy

Foyer is a privacy tool, so its security posture matters. This document explains
its threat model and how to report issues.

## Threat model

Foyer exists to interact with **hostile, untrusted captive portals** without
exposing your real browser profile. Concretely:

- The local SOCKS5 proxy binds to **loopback only** (`127.0.0.1`) and offers
  **no authentication** — it is never exposed off-host.
- The browser runs in a **throwaway profile** that is created in a temp dir and
  **deleted on exit**, and in **incognito** mode, so the portal sees no existing
  cookies, history, or saved credentials.
- DNS for the captive session is resolved **only** through the network's
  DHCP-advertised resolver and **only** inside the proxied browser; your system
  resolver and normal browser are never reconfigured.
- Foyer implements no DHCP client and stores no secrets.

## Supply chain

- The only runtime dependency is `github.com/BurntSushi/toml`.
- Releases ship a CycloneDX **SBOM** and a **cosign**-signed checksum file,
  produced by GoReleaser with GitHub OIDC (keyless) signing.
- CI runs `govulncheck`, CodeQL, OpenSSF Scorecard, and a hardened runner.

## Reporting a vulnerability

Please report security issues privately via GitHub Security Advisories
("Report a vulnerability" on the repository's **Security** tab) rather than a
public issue. We aim to acknowledge reports within 7 days.
