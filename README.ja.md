# darwinvpn

[English](README.md) | **日本語**

[![ci](https://github.com/mrsmsn/darwinvpn/actions/workflows/ci.yml/badge.svg)](https://github.com/mrsmsn/darwinvpn/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

macOS のネイティブ IKEv2 VPN を CLI から start/stop するための Go 製ツール。

> **Status: Phase 0 (scaffold)** — 足場のみ。VPN の実接続/切断機能は Phase 1 以降で実装予定。

## なぜこれを作るのか

macOS 標準の `scutil` / `networksetup` は IKEv2 の VPN サービスを扱えず、`scutil --nc list` にすら表示されない（Apple の長年の制約、`rdar://41950946`）。このため SSH セッションから VPN を制御できず、外出先から自宅 Mac 経由で社内 GitHub Enterprise に push できないといった困りごとが起こる。

`darwinvpn` は `NEConfigurationManager` と `ne_session_*`（非公開 API）経由でこれを解決する Go 製 CLI を提供する。

## インストール（Phase 0）

リリースバイナリは Phase 3 で配布予定。現状はソースからビルドする。

```sh
git clone https://github.com/mrsmsn/darwinvpn.git
cd darwinvpn
just build
./darwinvpn version
```

## 使い方（予定）

```
darwinvpn list                 # 登録プロファイルと接続状態を一覧
darwinvpn start [name]         # 接続（name 省略時は default プロファイル）
darwinvpn stop  [name]         # 切断
darwinvpn status [name]        # 状態表示（--json で機械可読出力）
darwinvpn add                  # 対話で新規プロファイルを作成
darwinvpn init                 # 初回セットアップ（config 生成 + add）
darwinvpn version              # バージョン情報
```

Phase 0 時点で動くのは `version` / `--help` のみ。他は `not yet implemented` を返して exit 1。

## ロードマップ

| Phase | スコープ | 状態 |
|-------|---------|------|
| 0 | cobra スケルトン、`vpn.Manager` interface、fake、macOS CI | done |
| 1 | cgo + Objective-C ブリッジで `list/start/stop/status` MVP | next |
| 2 | `add`/`init` で `.mobileconfig` 生成、Keychain / 1Password 連携 | planned |
| 3 | シェル補完、`--json` 出力、LaunchAgent 常駐、Homebrew tap、notarized release | planned |

詳細仕様は [`docs/pj.md`](docs/pj.md) を参照。

## リポジトリ構造

```
.
├── cmd/darwinvpn/      # main エントリーポイント
├── internal/
│   ├── cli/            # cobra コマンドツリー
│   └── vpn/            # Manager interface + fake + darwin bridge スタブ
├── docs/pj.md          # プロジェクト方針書(仕様の一次情報)
├── justfile            # build / test / vet / fmt / lint / clean
└── .github/workflows/  # macOS x Go matrix CI
```

## 開発

### 必要環境
- Go 1.23 以上
- macOS（ビルド成果物の動作対象）
- [`just`](https://github.com/casey/just)（タスクランナー）

### よく使うコマンド

```sh
just            # recipes 一覧
just build      # ./darwinvpn を生成（CGO_ENABLED=1）
just test       # go test -race -count=1 ./...
just vet        # go vet ./...
just fmt        # gofmt -w -s .
just lint       # go vet + go mod tidy -diff
just clean      # 生成物の削除
```

### バージョン文字列の埋め込み

`internal/cli.version` に ldflags で値を注入できる。

```sh
just build-versioned v0.1.0-dev
./darwinvpn version
# darwinvpn v0.1.0-dev
```

### テスト方針

`vpn.Manager` interface を境界として、cgo 依存の `bridge_darwin.go`（Phase 1 で実装）と in-memory な `fake.go` を分離している。CLI・config・provision のロジックは fake に対して高速に単体テストできる。実機 macOS でのみ動く `ne_session_*` 経路は Phase 1 以降に `//go:build integration` で隔離して追加予定。

## ライセンス

MIT License（[`LICENSE`](LICENSE) を参照）。

## クレジット

このツールが依存する macOS の非公開 API（`NEConfigurationManager` / `ne_session_*` / `libsystem_networkextension.dylib`）の解析は、**Alexandre Colucci (Timac) 氏**のリバースエンジニアリング成果に基づく。

- Blog: <https://blog.timac.org/2018/0717-macos-vpn-architecture/>
- VPNStatus 解説: <https://blog.timac.org/2018/0719-vpnstatus/>
- Reference implementation: <https://github.com/Timac/VPNStatus> (MIT)

ソースコードは直接コピーせず Go で再実装している。
