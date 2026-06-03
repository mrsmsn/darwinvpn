# darwinvpn

macOS のネイティブ IKEv2 VPN を CLI から start/stop するための Go 製ツール。

> **Status: Phase 0 (scaffold)** — 足場のみ。VPN の実接続/切断機能は Phase 1 以降で実装予定。

## なぜこれを作るのか

macOS 標準の `scutil` / `networksetup` は IKEv2 の VPN サービスを扱えず、`scutil --nc list` にすら表示されない（Apple の長年の制約、`rdar://41950946`）。このため SSH セッションから VPN を制御できず、外出先から自宅 Mac 経由で社内 GitHub Enterprise に push できないといった困りごとが起こる。

`darwinvpn` は `NEConfigurationManager` と `ne_session_*`（非公開 API）経由でこれを解決する Go 製 CLI を提供する。

## ライセンス

MIT License（`LICENSE` ファイルを参照）。

## クレジット

このツールが依存する macOS の非公開 API（`NEConfigurationManager` / `ne_session_*` / `libsystem_networkextension.dylib`）の解析は、**Alexandre Colucci (Timac) 氏**のリバースエンジニアリング成果に基づく。

- Blog: <https://blog.timac.org/2018/0717-macos-vpn-architecture/>
- Reference implementation: <https://github.com/Timac/VPNStatus>

ソースコードは直接コピーせず Go で再実装している。

## ロードマップ

- **Phase 0**: 足場（cobra スケルトン、`vpn.Manager` interface、fake 実装、macOS CI）← *現在地*
- **Phase 1**: 非公開 API ブリッジ（cgo + Objective-C）で `list/start/stop/status` の MVP
- **Phase 2**: `add`/`init` で `.mobileconfig` 生成、Keychain / 1Password 連携
- **Phase 3**: シェル補完、`--json` 出力、LaunchAgent 常駐、Homebrew tap、notarized リリース

詳細仕様は `docs/pj.md` を参照。

## 開発

Go 1.23+ が必要。macOS でのビルドのみサポート。

```sh
just build    # ローカルビルド
just test     # テスト（-race 込み）
just vet      # go vet
```

## 動作確認（Phase 0 時点）

```sh
./darwinvpn version    # バージョン表示
./darwinvpn --help     # サブコマンド一覧
```

`list` / `start` / `stop` / `status` / `add` / `init` は Phase 0 ではスタブで、`not yet implemented` を返して終了する。
