# omarchy-skk-popup

[Omarchy](https://omarchy.org) (Quattro / `omarchy-shell`) 向けの **SKK 日本語入力ポップアップ** プラグインです。どのアプリからでもホットキーで入力パネルを呼び出して SKK で日本語を入力し、確定文字列をクリップボードへ送って (任意で自動貼り付けして) 閉じます。

[skk-popup](https://github.com/takeshy/skk-popup) (Wails 製の常駐アプリ) を Omarchy ネイティブに作り直したものです。**UI は QML (Quickshell) で omarchy-shell の中に描画され、SKK エンジンだけが Go の常駐プロセス (`skk-popup-engine`) として動きます。** 辞書・ユーザー辞書・学習履歴・入力履歴のファイルは skk-popup と共通です。

```text
Ctrl+Shift+K (Hyprland bind)
      │  omarchy-shell shell summon takeshy.skk-popup '{}'
      ▼
omarchy-shell ── Panel.qml (kind: panel, keepLoaded)
                    │ stdin : {"op":"key","key":"a"} …   (JSON lines)
                    │ stdout: {"type":"state","text":"▽にほんご",…}
                    ▼
               skk-popup-engine serve   (Go: ローマ字→かな、辞書検索、候補、単語登録、履歴)
                    │ wl-copy / wl-paste / wtype
                    ▼
               クリップボード → 直前のウィンドウへ貼り付け
```

## 動作要件

- Omarchy Quattro 以降 (`omarchy-shell` が動いていること)
- `wl-clipboard` (`wl-copy` / `wl-paste`)
- 任意: `wtype` (確定後の自動貼り付けに使用)
- 任意: `curl` / `wget` (amd64・arm64 以外の arch でエンジンをフォールバック取得する場合のみ)
- 任意: Go 1.25 以降 (エンジンを自分でビルドする場合のみ)

## インストール

### 1. プラグインを追加する

```sh
omarchy plugin add https://github.com/takeshy/omarchy-skk-popup.git --enable
```

`omarchy plugin add` は git clone と manifest 検証しかしません (プラグインのコードを実行したり sudo を求めたりはしません)。**エンジン (`skk-popup-engine`) は arch 別バイナリがリポジトリに同梱されている**ので、これで一緒に入ります (linux amd64 / arm64)。`omarchy plugin update` で QML と一緒に更新されます。

`--enable` (および `omarchy plugin enable takeshy.skk-popup`) は、マニフェストに `bar-widget` があるため **バーの右セクションに「あ」ボタンを追加する形で有効化** します (パネル本体もそれで常駐します)。ボタンが不要なら、有効化後に `omarchy bar remove takeshy.skk-popup` でボタンだけ外せます (パネルは `shell.json` の `disabledPlugins` に入れない限り残ります)。

### 2. 辞書を取得する

バーの「あ」ボタンでパネルを出し、フッターの **「辞書を取得」ボタン** か **⋮ → 設定 → エンジン・辞書** から `dict fetch` を実行します (SKK-JISYO.L / geo / jinmei / propernoun / station、計 約9MB → `~/.local/share/skk-popup/dict/`)。辞書は同梱していないためここだけ 1 回必要です。ネットワークアクセスはこのボタンを押したときだけです。

エンジンについて:

- 同梱バイナリが既定で使われます (`<プラグインdir>/bin/skk-popup-engine-linux-<arch>`)。
- **自分のビルドを使う** → `SKK_POPUP_ENGINE=/path/to/skk-popup-engine` (これだけが同梱版に優先)。`go install …/cmd/skk-popup-engine@latest` や `make install-engine` (→ `~/.local/bin`) は同梱版より**後**に探索されます。
- **amd64 / arm64 以外** → 同梱バイナリも `fetch-engine.sh` のダウンロードも対象外なので、`go install github.com/takeshy/omarchy-skk-popup/cmd/skk-popup-engine@latest` か `make install-engine` で自分でビルドしてください。

探索順: `$SKK_POPUP_ENGINE` → `<プラグインdir>/bin/skk-popup-engine-linux-<arch>` → `<プラグインdir>/bin/skk-popup-engine` → `~/.local/share/skk-popup/bin` → `~/.local/bin` → `~/go/bin` → `/usr/local/bin` → `/usr/bin` → `PATH`。

### 3. (任意) ホットキーと設定

**⋮ → 設定** で:

- **エンジン・辞書**: 辞書の取得 / 再取得 (エンジンは同梱。arch 未対応時のみ `setup.sh` がダウンロード)
- **追加辞書**: 自分の SKK-JISYO / JSON ファイルのパスを追加・削除（エンジンが即ロード + `extra-dicts.json` に保存。`~/.local/share/skk-popup/dict/` に直接置いても可）
- **ショートカット**: Hyprland 0.56 は実行時バインド (`hyprctl keyword`) を無効化しているため、**パネルからキーは割り当てできません**。キー（既定 `CTRL SHIFT, K`）を入れて「反映」すると、貼り付け用の設定行を生成します。「bind = 行をコピー」/「o.bind 行をコピー」でクリップボードへ入るので、下記いずれかに貼って `hyprctl reload`（または `omarchy restart shell`）

```ini
# ~/.config/hypr/bindings.conf  (bind = 行)
bind = CTRL SHIFT, K, exec, omarchy-shell shell summon takeshy.skk-popup '{}'
```

```lua
-- ~/.config/hypr/bindings.lua  (Omarchy Quattro の Lua 設定はこちら)
o.bind("CTRL + SHIFT + K", "SKK popup", "omarchy-shell shell summon takeshy.skk-popup '{}'")
```

`summon` はパネル表示中にもう一度押すと**閉じずに入力欄へフォーカスを戻します**。閉じるのは `Escape` / `Close` / パネル外クリック。トグルが良ければ `toggle` に。IPC ターゲット `skk-popup` (`omarchy-shell skk-popup show` / `hide` / `toggle` / `state`) も使えます。

### 4. バーの「あ」ボタン

手順 1 で有効化した時点でバーの右セクションに「あ」ボタンが入っています。クリックでパネルをトグル。位置は `omarchy bar move` で変更できます。

## アンインストール

```sh
omarchy plugin remove takeshy.skk-popup
```

これで QML・同梱バイナリ・バーの「あ」ボタンが消えます。手順 3 で `bindings.conf` / `bindings.lua` に貼ったホットキー行があれば手動で削除して `hyprctl reload` してください。

辞書・ユーザー辞書・学習履歴・入力履歴・パネル位置・設定は **プラグインを消しても残ります** (skk-popup と共有する設計のため意図的)。完全に消すには:

```sh
rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/skk-popup"   # 辞書・ユーザー辞書・学習/入力履歴・パネル位置/設定
rm -rf "$HOME/.config/skk-popup"                          # config.toml (skk-popup と共有。単体で使うなら残す)
```

## セキュリティ・同梱バイナリ

- **同梱物**: `bin/skk-popup-engine-linux-amd64` と `-arm64` は、このリポジトリの Go ソース (`cmd/` / `internal/`) から `make vendor-engine` でビルドした静的 ELF です (バージョンは `manifest.json` の `version` を `-X main.version=` で埋め込み)。GitHub Releases の同名アセットは同じタグのソースを `.github/workflows/release.yml` がビルドしたものです。
- **`omarchy plugin add` / `update`** は git clone と manifest 検証だけで、プラグインのコードは実行しません。
- **実行時のネットワークアクセスは既定で無し**。amd64 / arm64 では同梱バイナリを直接使うため、何もダウンロードしません。
- **`scripts/fetch-engine.sh`** はフォールバック専用です (同梱バイナリが見つからない / 使えないときだけ、⋮ → 設定 の「辞書を取得」や `engineMissing` から起動)。動作は次の順:
  1. 同梱の `bin/skk-popup-engine-linux-<arch>` があればそれをコピーするだけ (ネットワーク無し)。
  2. 無いときだけ、このリポジトリの GitHub Release から該当アセットを取得し、**同梱バイナリの SHA-256 と一致した場合のみ**インストール。照合先の同梱バイナリすら無いときに限り、実行チェックのみで受け入れ、検証できなかった旨を警告します。

  `sudo` / パッケージマネージャ / `/etc` 書き換え / systemd / `curl | sh` は使いません。書き込み先は `~/.local/share/skk-popup/bin/` のみです。
- **`scripts/setup.sh`** は上記のエンジン準備 (必要時のみ) と `skk-popup-engine dict fetch` (辞書のダウンロード) を行います。
- **辞書の取得**は「辞書を取得」ボタンを押したときだけ実行され、取得元は `skk-popup-engine dict fetch` が参照する SKK-JISYO の標準配布先です。

## 使い方

1. `Ctrl+Shift+K` で入力パネルを出す。パネルは必ず `かな` モードで開きます
2. 通常の SKK 操作で入力する
3. 未変換状態で `Enter` (または `Copy` ボタン) → 確定文字列がクリップボードにコピーされ、パネルが閉じます
4. パネルが閉じるとフォーカスは自動的に直前のウィンドウへ戻り、`auto_paste = true` なら貼り付けショートカットが送出されます

エンジンは辞書をメモリに保持したまま omarchy-shell と一緒に常駐するため、2 回目以降の表示は即時です。`Escape` で閉じた場合の入力内容は次回まで保持されます。

パネルはヘッダー (タイトル行) をドラッグして移動でき、位置は `panel-position.json` に保存されて次回以降も維持されます。ヘッダー右クリック、または ⋮ メニューの「パネルを中央に戻す」で中央に戻ります。

ヘッダー右端の **⋮ メニュー**: 設定 / パネルを中央に戻す / ヘルプ (キー操作。パネル内にスクロール表示)。

## キー操作

[skk-popup](https://github.com/takeshy/skk-popup) / [chrome-skk-lite](https://github.com/takeshy/chrome-skk-lite) のクリップボード入力窓と同じ挙動です。

- 小文字ローマ字: かな入力
- 大文字で開始: 変換開始 (例: `Nihongo` → `▽にほんご`)
- 変換入力中の大文字: 送り仮名あり変換 (例: `KanJi` → `感じ`。`▽かん*じ` のように表示)
- `;`: sticky shift。変換開始 / 送り仮名開始位置の指定
- `Space`: 候補変換 / 次候補
- 5 候補目からは候補一覧を表示し、`A S D F J K L` で直接選択 (`Space`: 次ページ / `x`: 前ページ)
- 辞書注釈がある場合は `候補 ※注釈` の形で表示 (注釈は確定文字列に含まれない)
- 最後に確定した候補は、同じ読みの次回変換で優先表示
- 候補がない `Space` / 最終候補の次の `Space`: 単語登録ダイアログを開く
- 送り仮名あり (`▽かん*じ`) の登録では**漢字部分だけ**を入力する (送り仮名は自動で付く。ダイアログにゴースト表示)
- 登録ダイアログ内でもローマ字かな入力・候補変換・`q` / `Ctrl+Q` / `l` / `L` / `Ctrl+J` が使える
- 候補表示中の `x`: 前候補へ / 先頭で `x`: かな表示へ戻る
- 候補表示中の `X`: 表示中の候補をユーザー辞書・学習履歴から削除
- 候補表示中の `Ctrl+G`: 候補をキャンセルして変換バッファに戻る
- 送り仮名あり (`▽わた*し`) で `Ctrl+G`: 送り仮名を読みに畳んで `▽わたし` に戻す (単語登録ダイアログが開いていれば閉じてから畳む)
- 変換入力中の `Tab`: 過去に変換した読みから補完
- 読みに数字を含めると数値変換 (例: `だい5かい` → `第５回` / `第五回`)
- 変換入力中の `>`: 接頭辞変換 (例: `ちょう>` → `超`) / `▽>`: 接尾辞入力
- 変換入力中の `q`: カタカナで確定 (`Ctrl+Q`: 半角カタカナで確定)
- 非変換時の `q` / `Ctrl+Q`: カタカナ入力モード切替 (`SKK カナ` / `SKK 半ｶﾅ`)
- `l`: 英数モード (`SKK OFF`) へ / `L`: 全角英数モードへ / `Ctrl+J`: かなモードへ
- 空のかな入力状態で `/`: Abbrev モード (`▽/word`)。`//` で `/` を確定入力
- `zh zj zk zl` → `←↓↑→` / `z Space` → 全角スペース / `z. z, z- z/ z[ z]` → `… ‥ ～ ・ 『 』`
- `Shift+Enter`: 改行
- `Ctrl+V` / `Ctrl+Shift+V`: クリップボードの内容を挿入
- 非変換時の確定文字列の編集 (web の textarea 風):
  - `Shift+矢印` / `Shift+Home` / `Shift+End`: 範囲選択（選択中に入力・`Backspace` で置換）
  - `Ctrl+O`: 全選択 / `Ctrl+C`: 選択をコピー（閉じない）/ `Ctrl+X`: 選択を切り取り
  - Emacs 風: `Ctrl+F` / `Ctrl+B` カーソル前後 / `Ctrl+A` 行頭 / `Ctrl+E` 行末 / `Ctrl+K` 行末まで削除 / `Ctrl+U` 行頭まで削除
  - `Ctrl+Z`: 元に戻す
  - `↑` / `↓`: 複数行のときは上下の行へ移動。1 行目で `↑`（最終行で `↓`）はコピー履歴へ
- `Escape`: 選択中は選択解除 / 変換中はキャンセル / 未変換状態ならパネルを閉じる (コピーせず、入力内容は次回まで保持)
- `↑` / `↓`: コピー履歴を移動 (最大 30 件。`↓` で現在の下書きに戻る)
- パネル表示時、外部アプリでコピーされた新しいテキストも履歴へ自動追加
- `Enter`(未変換) / `Copy`: コピーして閉じる

状態はヘッダー右のバッジ (`SKK かな` / `SKK OFF` / ...) に表示され、クリックで `かな` / `OFF` を切り替えられます。

## 設定

`~/.config/skk-popup/config.toml` (skk-popup と共通。エンジンが読むのは以下のキーだけです)

```toml
[clipboard]
# コピー後に自動で貼り付けショートカットを送出 (wtype が必要)
auto_paste = true
# パネルが閉じてから送出までの待ち時間 (ミリ秒)
auto_paste_delay_ms = 80
# "ctrl+v" | "ctrl+shift+v"
# foot/alacritty/kitty などの多くのターミナルは Ctrl+V を readline の「次の
# 文字をリテラル入力」に使うため貼り付けに反応せず、ctrl+shift+v が必要。
# GUI アプリは主に ctrl+v だが ctrl+shift+v も通ることが多いため既定は ctrl+shift+v。
paste_key = "ctrl+shift+v"

[dictionary]
# 辞書ディレクトリ (既定: ~/.local/share/skk-popup/dict)
# dir = "~/.skk"
# 追加で 1 ファイル読み込む (SKK-JISYO 形式または JSON)
# external_path = "~/.skk/SKK-JISYO.user"
```

### データファイル

`~/.local/share/skk-popup/` (`$XDG_DATA_HOME/skk-popup`)。skk-popup と同じファイルなので、両方を使っても辞書と履歴は共有されます。

| ファイル | 内容 |
|---|---|
| `dict/` | システム辞書 (`dict fetch` の保存先) |
| `bin/skk-popup-engine` | arch 未対応で `setup.sh` がダウンロードしたエンジン (通常は同梱版を使うので不在) |
| `userdict.json` | 単語登録したユーザー辞書 |
| `history.json` | 候補の学習履歴 (読みごと最大 8 件) |
| `input-history.json` | コピー履歴 (最大 30 件) |
| `extra-dicts.json` | 設定で追加した辞書パスの一覧 (エンジンが読み書き。skk-popup とは非共有) |
| `panel-position.json` | ドラッグしたパネル位置 (QML。非共有) |
| `panel-settings.json` | 設定したホットキー (QML。非共有) |

## エンジンプロトコル

`skk-popup-engine serve` は stdin から 1 行 1 JSON のリクエストを受け取り、毎回描画用の状態を stdout に返します。UI はこの状態を表示するだけです。

```jsonc
// → stdin
{"op":"key","key":"a","ctrl":false,"shift":false,"alt":false}
// key: 印字可能 ASCII 1 文字 (Space は " ") か
//      "Enter" "Backspace" "Escape" "Tab" "Up" "Down" "Left" "Right" "Home" "End" "Delete"
{"op":"shown"}               // パネル表示 (外部クリップボードを履歴へ取り込む)
{"op":"hidden"}              // パネル非表示 (辞書を flush。コピー直後なら自動貼り付け)
{"op":"copy"} {"op":"close"} {"op":"paste"} {"op":"toggleMode"}
{"op":"registerSave"} {"op":"registerCancel"} {"op":"registerToggleMode"}
{"op":"setCursor","pos":3}   // 表示文字列上のオフセットへキャレット移動 (クリック)
{"op":"addDict","path":"~/.skk/SKK-JISYO.user"}   // 追加辞書を即ロード + 永続化
{"op":"removeDict","path":"~/.skk/SKK-JISYO.user"} // 一覧から削除 (メモリは次回起動で反映)
{"op":"quit"}

// ← stdout
{"type":"ready","version":"1.3.0","entries":123456,"dictionaries":["…/SKK-JISYO.L"],
 "extraDicts":["…"],"dataDir":"…","configPath":"…","enginePath":"…/skk-popup-engine"}
{"type":"state","text":"▽にほんご","cursor":5,"selStart":5,"selEnd":5,"mode":"SKK 変換",
 "candidate":"","candidateActive":false,
 "status":"Space: convert / Enter: copy / Ctrl+O: select all",
 "register":{"open":false,"reading":"","okuri":"","text":"","cursor":0,"selStart":0,"selEnd":0,"mode":"","candidate":"","error":""},
 "close":false,"copied":false}
{"type":"config","entries":123456,"extraDicts":["…"]}   // addDict / removeDict の後
{"type":"error","message":"…"}
```

`text` は確定文字列にプリエディット (`▽…` や未確定ローマ字) を差し込んだ表示用文字列、`cursor` はその中のキャレット位置です。`selStart` / `selEnd` は Shift 選択範囲 (未選択なら両方 `cursor` と同値)。`close` が `true` なら UI はパネルを閉じます。

## 開発

```sh
make vendor-engine   # bin/skk-popup-engine-linux-{amd64,arm64} (同梱バイナリ。Go を触ったら再生成してコミット)
make test            # go vet + go test
make validate        # manifest.json を omarchy plugin validate 相当でチェック (jq が必要)
make install         # vendor-engine + プラグイン一式を ~/.config/omarchy/plugins/takeshy.skk-popup へコピー
```

同梱バイナリの版数は `manifest.json` の `version` を埋め込みます。**Go を変更したら `make vendor-engine`（または `make install`）で再ビルドしてコミット**してください。CI が古い場合に警告します。

Omarchy 上では `omarchy plugin validate .` と `qmllint -I "$OMARCHY_PATH/shell" Panel.qml` で検証できます。`~/.config/omarchy/plugins/` 配下のファイルを保存すると omarchy-shell がエンジンプロセスを再起動します (旧プロセスは stdin の EOF で終了)。

**ただし `Panel.qml` などの QML 変更は `omarchy-shell shell rescanPlugins` では反映されません** — Quickshell がコンポーネントをキャッシュするため、`omarchy restart shell` (シェル完全再起動) が必要です。Go エンジンだけの変更なら `make install`（同梱バイナリを更新 → プラグイン再スキャンでエンジン再起動）で足ります。

構成:

```text
manifest.json              Omarchy plugin manifest (kinds: panel, bar-widget / keepLoaded)
Panel.qml                  入力パネル (エンジンの起動・キー送信・状態の描画)
BarWidget.qml              バーの「あ」ボタン
SkkButton.qml, SkkModeBadge.qml   パネル内の小さな部品
bin/skk-popup-engine-linux-*   同梱エンジン (make vendor-engine で生成・コミット)
scripts/fetch-engine.sh    Releases から arch 別エンジンバイナリを DL (arch 未対応時のフォールバック)
scripts/setup.sh           fetch-engine.sh（必要時）+ dict fetch (設定の「辞書を取得」)
cmd/skk-popup-engine/      CLI (serve / dict fetch / dict list / version)
internal/skk/              SKK エンジン (ローマ字変換、状態機械、単語登録、辞書)
internal/store/            userdict / history / input-history / extra-dicts の永続化
internal/config/           config.toml の読み込み
internal/clipboard/        wl-copy / wl-paste / wtype
```

## ライセンス

MIT
