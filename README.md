# darwinvpn

**English** | [日本語](README.ja.md)

[![ci](https://github.com/mrsmsn/darwinvpn/actions/workflows/ci.yml/badge.svg)](https://github.com/mrsmsn/darwinvpn/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A Go-based CLI for starting and stopping macOS VPN connections — both native IKEv2 (which `scutil` cannot reach) and Tunnel Provider-based VPNs such as Tailscale, WireGuard, or OpenVPN.

> **Status: Phase 1 in progress** — `list` / `status` / `start` / `stop` work end-to-end against the live NetworkExtension stack. `add` / `init` (Phase 2) and YAML profile aliases land later.

## Why this exists

macOS's built-in `scutil` and `networksetup` cannot manage IKEv2 VPN services, and these services do not even appear in `scutil --nc list` (a long-standing Apple constraint, tracked as `rdar://41950946`). That breaks workflows like SSH'ing into a home Mac and pushing to an internal GitHub Enterprise: the VPN drops, scutil can't help, and you're stuck.

`darwinvpn` goes through `NEConfigurationManager` and `ne_session_*` (private APIs) to do what scutil can't. The same code path also drives Tunnel Provider VPNs (Tailscale et al.), so the tool covers every VPN macOS knows about with one consistent CLI.

## Installation

Release binaries are planned for Phase 3. For now, build from source.

```sh
git clone https://github.com/mrsmsn/darwinvpn.git
cd darwinvpn
just build
./darwinvpn version
```

## Usage (planned)

```
darwinvpn list                 # List registered profiles and their connection state
darwinvpn start [name]         # Connect (defaults to the default profile when name is omitted)
darwinvpn stop  [name]         # Disconnect
darwinvpn status [name]        # Show status (use --json for machine-readable output)
darwinvpn add                  # Interactively create a new profile
darwinvpn init                 # First-time setup (config generation + add)
darwinvpn version              # Print version information
```

In Phase 1, `list` / `status` / `start` / `stop` operate against the live VPN stack. `add` and `init` still print `not yet implemented` until Phase 2.

## Roadmap

| Phase | Scope | Status |
|-------|-------|--------|
| 0 | cobra skeleton, `vpn.Manager` interface, fake implementation, macOS CI | done |
| 1 | cgo + Objective-C bridge for `list/start/stop/status`, scope covers both IKEv2 and Tunnel Provider VPNs | in progress |
| 2 | `add` / `init` with `.mobileconfig` generation, YAML profile aliases, Keychain / 1Password integration | planned |
| 3 | shell completion, LaunchAgent daemon mode, Homebrew tap, notarized release | planned |

Full specification: [`docs/pj.md`](docs/pj.md) (Japanese).

## Repository layout

```
.
├── cmd/darwinvpn/      # main entry point
├── internal/
│   ├── cli/            # cobra command tree
│   └── vpn/            # Manager interface + fake + darwin cgo bridge
├── docs/pj.md          # Project specification (single source of truth)
├── justfile            # build / test / vet / fmt / lint / clean
└── .github/workflows/  # macOS x Go matrix CI
```

## Development

### Requirements
- Go 1.25 or later (huh pulls a newer x/term)
- macOS (the only supported runtime target)
- [`just`](https://github.com/casey/just) (task runner)

### Common commands

```sh
just            # list all recipes
just build      # produce ./darwinvpn (CGO_ENABLED=1)
just test       # go test -race -count=1 ./...
just vet        # go vet ./...
just fmt        # gofmt -w -s .
just lint       # go vet + go mod tidy -diff
just clean      # remove build artifacts
```

### Embedding a version string

`internal/cli.version` accepts an ldflags injection:

```sh
just build-versioned v0.1.0-dev
./darwinvpn version
# darwinvpn v0.1.0-dev
```

### Testing strategy

The `vpn.Manager` interface separates the cgo-backed `bridge_darwin.go` from the in-memory `fake.go`. CLI, config, and provisioning logic are unit-tested quickly against the fake without touching a real VPN. The `ne_session_*` code path runs on a live macOS host and is isolated behind `//go:build integration`:

```sh
go test -v -tags integration -count=1 ./internal/vpn/...
```

The integration suite is read-only on the host VPN state (no Start/Stop is issued), so it is safe to run while a session is active.

## License

MIT License — see [`LICENSE`](LICENSE).

## Credits

The macOS private APIs this tool depends on (`NEConfigurationManager`, `ne_session_*`, `libsystem_networkextension.dylib`) were originally reverse-engineered by **Alexandre Colucci (Timac)**.

- Blog: <https://blog.timac.org/2018/0717-macos-vpn-architecture/>
- VPNStatus walkthrough: <https://blog.timac.org/2018/0719-vpnstatus/>
- Reference implementation: <https://github.com/Timac/VPNStatus> (MIT)

The source code is not copied — `darwinvpn` is reimplemented from scratch in Go.
