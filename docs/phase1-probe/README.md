# Phase 1 §6.5 probe

このディレクトリは pj.md §6.5 / §15 の確認タスクを実機で潰すための最小スニペット。Phase 1 で `internal/vpn/bridge_darwin.go` の cgo 実装を書き始める前に、以下を確定したい:

1. `-framework NetworkExtension` で `ne_session_*` のシンボルがリンクできるか
2. `NESessionTypeVPN` の整数値（Timac の知見では `1`）
3. `ne_session_status_t` の各値（Invalid=0, Disconnected=1, Connecting=2, Connected=3, Reasserting=4, Disconnecting=5）
4. サポート対象（macOS Sonoma 14 / Sequoia 15）で挙動が一致するか

## 前提

- Xcode Command Line Tools（`xcode-select --install`）が入っていること
- macOS 上で 1 つ以上 IKEv2 VPN 設定が「システム設定 > ネットワーク」に登録されていること（無くてもクラス・シンボル解決までは確認できる）

## 実行手順

### A. 直リンク版（推奨）

```sh
cd docs/phase1-probe
clang -fobjc-arc -framework Foundation -framework NetworkExtension \
    probe.m -o probe
./probe
```

出力を**そのまま貼り戻してくれ**。

期待する出力例（VPN が 1 件 disconnected で登録されている場合）:

```
=== probe: NetworkExtension private API check ===
macOS version: 15.x
architecture:  arm64
OK: NEConfigurationManager class resolved (0x...)
OK: +sharedManager returned an instance
OK: loaded 1 configuration(s)
  - mnxlab-vpn                    [XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX]

=== ne_session_* link check ===
ne_session_create     address: 0x...
ne_session_get_status address: 0x...
ne_session_release    address: 0x...
OK: ne_session_create succeeded for mnxlab-vpn
OK: ne_session_get_status -> 1 (Disconnected)
```

### B. dlopen フォールバック（A が unresolved symbol で失敗した場合）

ハードンドランタイムやライブラリ検証で直リンクが拒否された場合、`probe.m` のヘッダ `extern` 宣言を関数ポインタ変数に置き換え、以下のように `dlopen` / `dlsym` で取る:

```c
#include <dlfcn.h>

static void *libne = NULL;
static ne_session_t (*ne_session_create_p)(uuid_t, int) = NULL;
static void (*ne_session_release_p)(ne_session_t) = NULL;
static void (*ne_session_get_status_p)(ne_session_t, dispatch_queue_t,
                                       ne_session_status_block) = NULL;

static int load_ne_symbols(void) {
    libne = dlopen("/usr/lib/system/libsystem_networkextension.dylib",
                   RTLD_LAZY | RTLD_GLOBAL);
    if (!libne) {
        fprintf(stderr, "dlopen failed: %s\n", dlerror());
        return -1;
    }
    ne_session_create_p     = dlsym(libne, "ne_session_create");
    ne_session_release_p    = dlsym(libne, "ne_session_release");
    ne_session_get_status_p = dlsym(libne, "ne_session_get_status");
    if (!ne_session_create_p || !ne_session_get_status_p) {
        fprintf(stderr, "dlsym missing required symbol\n");
        return -1;
    }
    return 0;
}
```

その上で `ne_session_create(...)` → `ne_session_create_p(...)` のように呼び出しを書き換える。dlopen 版が必要になった場合は、`probe.m` のフル版を別途用意するので声かけて。

## 確認したい情報（貼り戻してほしい内容）

1. `sw_vers -productVersion` と `uname -m`
2. 直リンクのビルドが通ったか（clang のエラーがあれば全文）
3. `OK: ne_session_create succeeded ...` まで到達したか
4. `ne_session_get_status -> N (Name)` の N と、その時の VPN の実際の状態（disconnected/connected/etc.）が一致しているか
5. 警告（deprecated 等）が出ていればその全文

Sonoma と Sequoia の両方で確認できればベスト。片方だけでも先に進める。

## probe バイナリの扱い

probe バイナリは git で追跡しない（リポジトリ全体の `.gitignore` で `*.test` 等は除外しているが、`probe` は無印実行ファイル名なので `docs/phase1-probe/probe` だけ別途無視するか手動削除すること）。

---

## probe2.m — scope 拡張用の調査

`docs/pj.md` 当初の IKEv2 専用スコープを `scutil --nc list` 対象（Tailscale 等 Tunnel Provider 系）まで広げるかを判断するために、各 NEConfiguration の VPN payload と `ne_session_create` の `sessionType` 引数のマッピングを実機で観察する。

### 確認したいこと

1. NEConfiguration.VPN が non-nil なエントリのうち、protocol class が何か（`NEVPNProtocolIKEv2` / `NETunnelProviderProtocol` / 等）
2. 各 NEConfiguration に対して `ne_session_create(uuid, sessionType)` を `1`（VPN）/ `5`（PacketTunnel）/ `9`（PluginVPN）の 3 値で呼び、`ne_session_get_status` がどう返るか
3. どの protocol class がどの sessionType と対応するか

### 実行

```sh
cd docs/phase1-probe
clang -fobjc-arc -framework Foundation -framework NetworkExtension \
    probe2.m -o probe2
./probe2
```

`Start` も `Stop` も呼ばないので副作用なし。Tailscale が動いていてもそのまま走らせて大丈夫。

### 期待する貼り戻し

出力全文。特に:
- mnxlab-vpn の `protocol class` と各 sessionType での status
- Tailscale の `protocol class` と各 sessionType での status（NULL / TIMEOUT / 数値）
- 他の VPN payload を持つエントリの結果
