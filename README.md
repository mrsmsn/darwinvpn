# darwinvpn

**English** | [日本語](README.ja.md)

[![ci](https://github.com/mrsmsn/darwinvpn/actions/workflows/ci.yml/badge.svg)](https://github.com/mrsmsn/darwinvpn/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A Go-based CLI for starting and stopping macOS native IKEv2 VPN connections.

> **Status: Phase 0 (scaffold)** — only the scaffolding is in place. Actual VPN connect/disconnect functionality lands in Phase 1 and later.

## Why this exists

macOS's built-in `scutil` and `networksetup` cannot manage IKEv2 VPN services, and these services do not even appear in `scutil --nc list` (a long-standing Apple constraint, tracked as `rdar://41950946`). As a result you cannot control the VPN from an SSH session, which breaks workflows like pushing to an internal GitHub Enterprise through a home Mac when you are away from home.

`darwinvpn` solves this by going through `NEConfigurationManager` and `ne_session_*` (private APIs) from a Go CLI.

## Installation (Phase 0)

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

In Phase 0 only `version` and `--help` are functional. Every other subcommand prints `not yet implemented` and exits with status 1.

## Roadmap

| Phase | Scope | Status |
|-------|-------|--------|
| 0 | cobra skeleton, `vpn.Manager` interface, fake implementation, macOS CI | done |
| 1 | cgo + Objective-C bridge for `list/start/stop/status` MVP | next |
| 2 | `add` / `init` with `.mobileconfig` generation, Keychain / 1Password integration | planned |
| 3 | shell completion, `--json` output, LaunchAgent daemon mode, Homebrew tap, notarized release | planned |

Full specification: [`docs/pj.md`](docs/pj.md) (Japanese).

## Repository layout

```
.
├── cmd/darwinvpn/      # main entry point
├── internal/
│   ├── cli/            # cobra command tree
│   └── vpn/            # Manager interface + fake + darwin bridge stub
├── docs/pj.md          # Project specification (single source of truth)
├── justfile            # build / test / vet / fmt / lint / clean
└── .github/workflows/  # macOS x Go matrix CI
```

## Development

### Requirements
- Go 1.23 or later
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

The `vpn.Manager` interface separates the cgo-backed `bridge_darwin.go` (filled in during Phase 1) from the in-memory `fake.go`. CLI, config, and provisioning logic can be unit-tested quickly against the fake without touching a real VPN. The `ne_session_*` code path, which only runs on a live macOS host, will be isolated behind `//go:build integration` once it is implemented.

## License

MIT License — see [`LICENSE`](LICENSE).

## Credits

The macOS private APIs this tool depends on (`NEConfigurationManager`, `ne_session_*`, `libsystem_networkextension.dylib`) were originally reverse-engineered by **Alexandre Colucci (Timac)**.

- Blog: <https://blog.timac.org/2018/0717-macos-vpn-architecture/>
- VPNStatus walkthrough: <https://blog.timac.org/2018/0719-vpnstatus/>
- Reference implementation: <https://github.com/Timac/VPNStatus> (MIT)

The source code is not copied — `darwinvpn` is reimplemented from scratch in Go.
