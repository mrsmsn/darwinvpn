# darwinvpn プロジェクト方針書

> macOS のネイティブ IKEv2 VPN を CLI から start/stop するための、Go 製 OSS コマンドラインツールの方針書。
> このドキュメントは Claude（Claude Code 等）に実装を依頼する際のベース仕様として使うことを想定している。

---

## 1. 背景と目的

macOS 標準の `scutil` / `networksetup` は IKEv2 の VPN サービスを扱えず、`scutil --nc list` にすら表示されない（Apple の長年の制約。Timac が `rdar://41950946` として報告済み）。このため、自宅常駐の開発用 Mac に外出先から SSH 接続したとき、VPN が切れていると Enterprise GitHub への push 等ができない。

既存の解決策に Timac の `vpnutil`（`Timac/VPNStatus` 同梱）があるが、Objective-C 製で機能も最小限。本プロジェクトでは、これを Go で再実装し、設定ファイル管理・対話セットアップ・自動再接続まで視野に入れた、より使いやすい CLI を OSS として開発する。

### 解決したいこと
- SSH セッションからワンコマンドで対象 VPN を start/stop できる。
- 複数の接続先を YAML で管理し、名前で選べる。
- 新しい接続先を対話形式でセットアップできる。

---

## 2. プロジェクト名

第一候補は **`darwinvpn`**。macOS（Darwin）専用であることが明確で、衝突も少ない。

代替候補（必要なら検討）:
- **`nevpn` / `nevpnctl`** … 内部で叩く NetworkExtension / `ne_session` 由来。「何をするか」が伝わる短い名前。
- **`vpnup`** … 親しみやすい。`vpnup start` などやや冗長になりうる。
- **`tunnelctl`** … 汎用的すぎて意味が薄い。

> 推奨は `darwinvpn`。CLI バイナリ名・モジュールパスともにこれで進める。気が変わった場合に備え `nevpnctl` を次点とする。

---

## 3. 機能要件

| ID | 要件 |
|----|------|
| F-1 | `darwinvpn start [name]` / `stop [name]` でワンコマンド接続・切断（`name` 省略時は default プロファイル） |
| F-2 | `darwinvpn list` で登録プロファイルと現在の接続状態を一覧表示 |
| F-3 | `darwinvpn status [name]` で状態を表示（`--json` で機械可読出力） |
| F-4 | YAML 設定ファイルで接続先（プロファイル）を管理し、名前で選択できる |
| F-5 | `darwinvpn add` で対話形式に新しい接続先を作成できる（IKEv2 パラメータ収集 → `.mobileconfig` 生成・インストール → プロファイル登録） |
| F-6 | `darwinvpn init` で初回セットアップ（設定ファイルのひな形生成＋`add` の実行） |
| F-7 | 秘密情報（パスワード・共有シークレット）を平文 YAML に保存しない |

---

## 4. 非機能要件・制約

- **対象 OS**: macOS のみ（Apple Silicon / Intel 両対応）。サポート対象の macOS メジャーバージョンを明記し、CI で検証する。
- **非公開 API 依存**: 内部実装は Apple の undocumented API に依存する（後述）。OS のメジャーアップデートで挙動が変わりうる前提で設計・検証する。
- **特権**: start/stop/list/status は root 不要（`vpnutil` 同様、実処理は root デーモン `nesessionmanager` に委譲される）。
- **ユーザーセッション依存**: Personal VPN（IKEv2）はログイン中のユーザーセッションに紐づく。未ログイン状態では接続を維持できないため、将来の常駐モードは LaunchAgent（ユーザー）として実装する（LaunchDaemon 不可）。
- **配布**: 非公開シンボルにリンクするため、コード署名・ハードンドランタイム・notarization の扱いを設計に含める。App Store 配布は不可。

---

## 5. アーキテクチャ

```
+-------------------------------------------------------------+
|  CLI (Go)  cobra: start / stop / list / status / add / init |
+-------------------------------------------------------------+
|  config (Go)        provision (Go)        secret (Go)        |
|  YAML load/save      mobileconfig生成      Keychain/1Password |
+-------------------------------------------------------------+
|  vpn.Manager (Go interface)                                 |
|    - List() / Start(uuid) / Stop(uuid) / Status(uuid)       |
|    +-- bridge_darwin.go (cgo)  ←→  fake.go (テスト用)         |
+-------------------------------------------------------------+
|  C/Objective-C ブリッジ (bridge.m / bridge.h)                |
|    NEConfigurationManager で設定列挙                          |
|    ne_session_create / start / stop / get_status を呼ぶ      |
+-------------------------------------------------------------+
|  libsystem_networkextension.dylib (非公開)                   |
|         |  XPC                                              |
|  nesessionmanager (root デーモン) → 実際に IKEv2 を接続/切断  |
+-------------------------------------------------------------+
```

### 実装方式の選択: cgo + Objective-C ブリッジ（推奨）

- 設定列挙に使う `NEConfigurationManager` は Objective-C の非公開クラスで、`loadConfigurationsWithCompletionQueue:handler:` は **completion ブロック＋ dispatch queue** の非同期 API。
- `ne_session_get_status` / `ne_session_set_event_handler` も **ブロック** を取る。
- これらを Go から直接扱うのは煩雑なため、**非公開 API と Objective-C の面倒な部分を小さな C/ObjC ブリッジに閉じ込め**、`dispatch_semaphore` で同期化した素直な C ABI を Go に公開する方式を推奨する。

### 代替案: purego（cgo を使わない）
- `ebitengine/purego` で dylib を `dlopen` し、`objc_msgSend` を動的に呼ぶ方式。cgo 非依存にできるが、Objective-C ブロックの扱いが難しい。
- 評価はするが、ブロック中心の API 構成のため **第一候補は cgo + ObjC ブリッジ**とする。

---

## 6. macOS 非公開 API の利用詳細（実装の核）

> このスレッドの調査と `Timac/VPNStatus`（および解説記事 `blog.timac.org/2018/0717-macos-vpn-architecture`）が一次情報。Claude はこの章を起点に実装し、ゼロから再調査しないこと。

### 6.1 設定の列挙
- 公開フレームワーク `NetworkExtension.framework` 内の **非公開クラス** を使用する。
  - `NEConfigurationManager`（`+sharedManager`, `-loadConfigurationsWithCompletionQueue:handler:` → `NSArray<NEConfiguration*>`）
  - `NEConfiguration`（`identifier`: `NSUUID`, `name`: `NSString`, `VPN`: `NEVPN`）
- これはシステム設定のネットワークペインが `ANPNEServicesManager` 経由で行っているのと同じ経路。

### 6.2 start / stop / status
- 各設定の UUID から `ne_session_create(uuid, NESessionTypeVPN)` でセッションを生成。
- 開始/停止は `ne_session_start()` / `ne_session_stop()`、状態取得は `ne_session_get_status()`。
- これらの C 関数は非公開 dylib `/usr/lib/system/libsystem_networkextension.dylib` に実装。
- 各関数は **XPC 経由で root デーモン `nesessionmanager`（`/usr/libexec/nesessionmanager`）** にコマンドを送り、実処理はデーモンが行う（IKEv2 は `NESMIKEv2VPNSession`）。

### 6.3 参考にする C プロトタイプ
```c
// /usr/lib/system/libsystem_networkextension.dylib（非公開）
ne_session_t ne_session_create(uuid_t serviceID, int sessionConfigType); // NESessionTypeVPN
void ne_session_release(ne_session_t session);
void ne_session_start(ne_session_t session);
void ne_session_stop(ne_session_t session);
void ne_session_cancel(ne_session_t session);

typedef void (^ne_session_event_block)(xpc_object_t result);
void ne_session_set_event_handler(ne_session_t session, dispatch_queue_t queue, ne_session_event_block block);

typedef void (^ne_session_status_block)(ne_session_status_t result);
void ne_session_get_status(ne_session_t session, dispatch_queue_t queue, ne_session_status_block block);
```

### 6.4 ブリッジが Go に公開する C ABI（案）
```c
typedef struct {
    char name[256];
    char uuid[40];
    int  status;   // ne_session_status_t を整数で
} dvpn_service_t;

int dvpn_list(dvpn_service_t **services, int *count); // 呼び出し側が free
int dvpn_start(const char *uuid);
int dvpn_stop(const char *uuid);
int dvpn_status(const char *uuid, int *status_out);
```
- 内部で `NEConfigurationManager` による列挙と `ne_session_*` 呼び出しを行い、非同期は `dispatch_semaphore_wait` で同期化する。
- 戻り値は 0=成功 / 非 0=エラーコードに統一。

### 6.5 実装前に確認が必要な点（Claude へのタスク化推奨）
- `NESessionTypeVPN` の整数値、`ne_session_status_t` の各値（Connected/Disconnected/Connecting/Disconnecting/Invalid 等）を実機の runtime / ヘッダダンプで確定する。
- リンク方法: まず `// #cgo LDFLAGS: -framework Foundation -framework NetworkExtension` を試す。`ne_session_*` シンボルが解決できない場合は `dlopen("/usr/lib/system/libsystem_networkextension.dylib")` + `dlsym` の動的解決にフォールバックする。
- サポート対象 macOS バージョンごとに、上記値とシンボルが一致するか検証する。

### 6.6 クレジット
- 非公開 API の知見は Timac（Alexandre Colucci）のリバースエンジニアリング成果に基づく。**ソースコードを直接コピーせず Go で再実装**し、README とソースに同氏のブログ（`blog.timac.org`）へのクレジットを明記する。

---

## 7. CLI 仕様

```
darwinvpn list                 # プロファイルと接続状態を一覧
darwinvpn start [name]         # 接続（name 省略時は default）
darwinvpn stop  [name]         # 切断
darwinvpn status [name]        # 状態表示
darwinvpn add                  # 対話で新規作成: パラメータ収集→mobileconfig生成→承認→登録
darwinvpn init                 # 初回セットアップ（config生成 + add）
darwinvpn connect|disconnect   # start|stop のエイリアス（任意）
darwinvpn version              # バージョン情報
```

共通フラグ:
- `--config <path>` … 設定ファイルパス（既定 `~/.config/darwinvpn/config.yaml`）
- `--json` … 機械可読出力（list/status）
- `-v, --verbose` … 詳細ログ

依存ライブラリ（案）:
- CLI: `spf13/cobra`
- YAML: `gopkg.in/yaml.v3`
- 対話プロンプト: `charmbracelet/huh`（モダン）または `AlecAivazis/survey`

---

## 8. 設定ファイル仕様（YAML）

```yaml
# ~/.config/darwinvpn/config.yaml
version: 1
default: work-ghe
profiles:
  - name: work-ghe                 # darwinvpn 上の識別名（エイリアス）
    description: "Enterprise GitHub VPN"

    # add/init が生成・インストールした macOS 設定の識別子（インストール後に記録）
    system:
      display_name: "mnxlab-vpn"   # システム上の VPN 表示名
      uuid: ""                     # NEConfiguration の UUID（解決後にキャッシュ）

    # add/init で対話収集し、.mobileconfig 生成の元になる IKEv2 パラメータ
    ikev2:
      server: vpn.example.com
      remote_id: vpn.example.com
      local_id: user@example.com
      auth: eap                    # eap | certificate | shared-secret
      on_demand:                   # 任意: Connect On Demand を有効化
        enabled: false
        match_domains: ["ghe.example.com"]

    # 秘密情報の参照（平文では保存しない）
    secret:
      provider: keychain           # keychain | 1password
      ref: "darwinvpn/work-ghe"
```

### 秘密情報の扱い
- パスワード・共有シークレットは **YAML に平文で書かない**。
- `provider: keychain` … macOS Keychain に保存し、`ref` で参照（`security` コマンド or Keychain API）。
- `provider: 1password` … 1Password CLI（`op read`）で実行時に解決。
- mobileconfig には認証情報が平文で埋め込まれる。生成ファイルは一時ディレクトリに 0600 で置き、インストール後に必ず削除する（§9 参照）。

> `start` / `stop` / `status` は `system.uuid`（あれば優先）または `system.display_name` を `dvpn_list` の結果に突き合わせて対象設定を解決する。

---

## 9. VPN 設定の作成方法（`init` / `add` の実装方針）

**方針: `add` / `init` は mobileconfig 方式に一本化する。** `ne_session_*` で start できるのはシステムに既に存在する設定のみで、設定の新規作成を entitlement なしで安全に行う唯一の現実的手段が、構成プロファイル（`.mobileconfig`）の生成＋ユーザー承認インストールであるため。

### 処理フロー（`add`）
1. 対話で IKEv2 パラメータを収集する（サーバ、Remote ID、Local ID、認証方式 = EAP / 証明書 / 共有シークレット、必要なら On Demand 対象ドメイン）。
2. 秘密情報（パスワード／共有シークレット）を入力させる、または `secret.provider`（Keychain / 1Password）から取得する。
3. 収集値から `.mobileconfig`（VPN ペイロード, `VPNType=IKEv2`）を生成する。On Demand 指定時は `OnDemandEnabled` / `OnDemandRules` を含める。
4. 一時ファイルを **0600 権限**で書き出し、`open <file>` でインストール用 UI を起動してユーザーに承認を促す（非監視 Mac では手動承認が必須で、サイレントインストールは不可）。
5. インストール完了を `dvpn_list` のポーリングで検出し、新しく現れた設定の `name` / `uuid` を取得する。
6. IKEv2 パラメータ（秘密は除外し `secret` は参照のみ）と、解決した `system.display_name` / `system.uuid` を YAML に書き込む。
7. **生成した一時 mobileconfig を安全に削除する**（秘密が埋め込まれているため）。

### `init`
- 設定ファイルのひな形（`~/.config/darwinvpn/config.yaml`）を生成し、続けて `add` を実行する初回セットアップ用コマンド。

### 留意点
- mobileconfig には認証情報が平文で埋め込まれる。一時ディレクトリに 0600 で置き、インストール後に必ず削除する。
- 既存のシステム VPN を darwinvpn 管理下に取り込みたい場合は、YAML の `system.display_name` を手書きする運用で対応する（`add` の対象外。将来 `import` サブコマンドとして検討可）。

### 却下した代替案
- **非公開 write API（`NEConfigurationManager` の保存系）で直接作成**: 最も非公開 API に踏み込み、権限・互換性のリスクが高いため採用しない。
- **公開 `NEVPNManager` で作成**: Personal VPN entitlement（`com.apple.developer.networking.vpn.api`）と Apple Developer 署名が必要になり、OSS の手元ビルドと相性が悪いため採用しない。

---

## 10. ビルド / 署名 / 配布

- **ビルド**: cgo 有効（`CGO_ENABLED=1`）。`bridge.m` を含むため Xcode Command Line Tools が必要。
- **署名**: ローカルビルドは ad-hoc 署名で可。配布バイナリは Developer ID 署名＋ハードンドランタイム＋notarization を行う（`Timac/VPNStatus` も 2.0 で署名・ハードンドランタイム化済み）。
- **ライブラリ検証**: 非公開シンボルにリンクするため、ハードンドランタイム有効時の挙動を検証する。問題があれば `dlopen`/`dlsym` 方式に切替。
- **配布**: GitHub Releases にバイナリ、Homebrew tap（`brew install <tap>/darwinvpn`）を用意。

---

## 11. テスト方針

> Morishima が重視する「高速なローカルテスト」と相性を取り、非公開 API 依存部とロジックを分離する。

- **境界の分離**: `vpn.Manager` を Go インターフェースとして定義し、cgo 実装（`bridge_darwin.go`）と **fake 実装（`fake.go`）** を用意。CLI・config・provision のロジックは fake に対して高速にユニットテストする（実機・VPN 不要）。
- **設定/生成のテスト**: YAML の読み書き、mobileconfig 生成はゴールデンファイルでテスト。
- **統合テスト**: 実機 macOS でのみ動く `ne_session_*` 経路は、`//go:build integration` 等のビルドタグで隔離。CI（macOS ランナー）で対象 macOS バージョンごとに `list`/`status` の疎通を確認。
- **SSH 経路の確認**: ログイン中ユーザーとして SSH 越しに start/stop が成功することを手動テスト項目に含める。

---

## 12. リスクと留意点

- 非公開 API は無保証で、macOS メジャーアップデートで壊れうる（enum 値・シンボル・XPC 仕様の変更）。→ バージョンごとの検証を CI と手順に組み込む。
- ハードンドランタイム／ライブラリ検証によりロードが拒否される可能性。→ `dlopen` フォールバックを用意。
- Personal VPN はユーザーセッション依存。→ 常駐／自動再接続は LaunchAgent 前提。
- App Store 配布は不可（非公開 API）。

---

## 13. 開発フェーズ / マイルストーン

- **Phase 0: 足場づくり**
  Go モジュール初期化、cobra スケルトン、MIT ライセンス、README、macOS CI、`vpn.Manager` インターフェースと `fake.go`。
- **Phase 1: MVP**
  `bridge.m`（list/start/stop/status）実装、`ne_session_*` 経路の疎通確認、`list`/`start`/`stop`/`status` コマンド、YAML（モードA: 既存登録）、`add`（既存登録のみ）。
- **Phase 2: プロビジョニング**
  `add`/`init` のモードB（IKEv2 パラメータ収集 → mobileconfig 生成 → 承認 → 検出登録）、秘密情報（Keychain / 1Password）対応。
- **Phase 3: 仕上げ**
  シェル補完、`--json` 出力、（任意）watchdog / 自動再接続の LaunchAgent モード、Connect On Demand 連携、Homebrew tap、notarized リリース。

---

## 14. ライセンス / クレジット

- ライセンス: **MIT**（`vpnutil` と整合）。
- クレジット: Timac（Alexandre Colucci）の macOS VPN アーキテクチャ解析および `Timac/VPNStatus` を参考にした旨を README とソースに明記。コードは直接コピーせず Go で再実装する。

---

## 15. 未決事項 / 要判断

- [ ] サポートする macOS 最小バージョン（例: Sonoma 以降にするか）。
- [ ] `NESessionTypeVPN` / `ne_session_status_t` の確定値（実機で確認）。
- [ ] リンク方式（`-framework` 直リンク vs `dlopen`/`dlsym`）の最終決定。
- [ ] 対話プロンプトのライブラリ（`huh` vs `survey`）。
- [ ] `init`/`add` の既定モード（A=既存登録を既定にするか、B=新規作成を前面に出すか）。
- [ ] 秘密情報のデフォルト provider（keychain か none か）。
- [ ] バイナリ名の最終確定（`darwinvpn` で確定して良いか）。

---

## 16. このドキュメントの使い方（Claude への依頼方法）

1. まずこのファイル全体を Claude に渡し、「§13 の Phase 0 から着手」を依頼する。
2. Phase 1 着手前に、Claude に **§6.5 の確認タスク**（enum 値・シンボル解決）を先に潰させる。実機での確認が要る箇所は、確認用の最小 Obj-C/C スニペットを書かせて Morishima が実行 → 結果を貼り戻す運用にする。
3. 以降は Phase 単位で「実装 → 統合テスト項目の提示 → レビュー」を回す。
4. 非公開 API に踏み込む実装（特にモードC）は、リスクを明記したうえで Morishima の判断を仰ぐようにさせる。

---

## 付録: 本スレッドの調査結果サマリ（一次情報）

- `scutil` / `networksetup` は IKEv2 を扱えず `scutil --nc list` に出ない（Apple の制約。`rdar://41950946`）。Tailscale 等の Tunnel Provider 型はバンドル ID 付きで `scutil` に出るが、ネイティブ Personal VPN（IKEv2）は出ない。
- `vpnutil`（`Timac/VPNStatus` 同梱, MIT）は `NEConfigurationManager` で設定列挙し、`ne_session_create/start/stop`（`libsystem_networkextension.dylib`）→ XPC → `nesessionmanager`（root デーモン）で接続/切断する。GUI と同じ経路のため IKEv2 でも動作する。
- 参考リンク:
  - リポジトリ: https://github.com/Timac/VPNStatus
  - アーキテクチャ解説: https://blog.timac.org/2018/0717-macos-vpn-architecture/
  - VPNStatus 解説: https://blog.timac.org/2018/0719-vpnstatus/
