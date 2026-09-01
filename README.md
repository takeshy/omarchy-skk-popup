# omarchy-skk-popup

[Omarchy](https://omarchy.org) (Quattro / `omarchy-shell`) 向けの **SKK 日本語入力ポップアップ** プラグインです。どのアプリからでもホットキーで入力パネルを呼び出して SKK で日本語を入力し、確定文字列をクリップボードへ送って (任意で自動貼り付けして) 閉じます。

[skk-popup](https://github.com/takeshy/skk-popup) (Wails 製の常駐アプリ) を Omarchy ネイティブに作り直したものです。**UI は QML (Quickshell) で omarchy-shell の中に描画され、SKK エンジンだけが Go の常駐プロセス (`skk-popup-engine`) として動きます。** 辞書・ユーザー辞書・学習履歴・入力履歴のファイルは skk-popup と共通です。

```text
Ctrl+Shift+K (Hyprland bind)
      │  omarchy-shell shell toggle takeshy.skk-popup '{}'
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
- エンジンのビルドに Go 1.25 以降 (Releases のバイナリを使う場合は不要)

## インストール

### 1. エンジン (`skk-popup-engine`) を入れる

Releases からダウンロードする場合:

```sh
install -Dm755 skk-popup-engine-linux-amd64 ~/.local/bin/skk-popup-engine
```

ソースから入れる場合:

```sh
go install github.com/takeshy/omarchy-skk-popup/cmd/skk-popup-engine@latest   # → ~/go/bin
# または
git clone https://github.com/takeshy/omarchy-skk-popup.git && cd omarchy-skk-popup
make install-engine                                                            # → ~/.local/bin
```

Panel.qml は `$SKK_POPUP_ENGINE` → `<プラグインdir>/bin/skk-popup-engine` → `~/.local/bin` → `~/go/bin` → `/usr/local/bin` → `/usr/bin` → `PATH` の順にエンジンを探します。

### 2. 辞書をダウンロードする

```sh
skk-popup-engine dict fetch      # SKK-JISYO.L / geo / jinmei / propernoun / station
skk-popup-engine dict list       # 読み込まれる辞書ファイルを確認
```

辞書は `~/.local/share/skk-popup/dict/` に置かれます。既存の SKK-JISYO ファイル (EUC-JP / UTF-8) や skk-popup の `dictionary.json` をこのディレクトリに置いても読み込まれます。

### 3. プラグインを追加する

```sh
omarchy plugin add https://github.com/takeshy/omarchy-skk-popup.git --enable
```

`omarchy plugin add` は git clone と manifest 検証しかしません (プラグインのコードを実行したり sudo を求めたりはしません)。エンジンは上の手順 1 で別途入れてください。

### 4. Hyprland にキーを割り当てる

```ini
# ~/.config/hypr/bindings.conf
bind = CTRL SHIFT, K, exec, omarchy-shell shell toggle takeshy.skk-popup '{}'
```

`omarchy-shell shell summon takeshy.skk-popup '{}'` / `omarchy-shell shell hide takeshy.skk-popup` も使えます。プラグイン側の IPC ターゲット `skk-popup` (`omarchy-shell skk-popup toggle` / `show` / `hide` / `state`) でも同じことができます。

### 5. (任意) バーに「あ」ボタンを置く

_Setup > Plugins_ で SKK Popup を有効化すると、バーの右セクションに「あ」ボタンが追加されます。クリックでパネルをトグルします。位置は `omarchy bar move` で変更できます。

## 使い方

1. `Ctrl+Shift+K` で入力パネルを出す。パネルは必ず `かな` モードで開きます
2. 通常の SKK 操作で入力する
3. 未変換状態で `Enter` (または `Copy` ボタン) → 確定文字列がクリップボードにコピーされ、パネルが閉じます
4. パネルが閉じるとフォーカスは自動的に直前のウィンドウへ戻り、`auto_paste = true` なら貼り付けショートカットが送出されます

エンジンは辞書をメモリに保持したまま omarchy-shell と一緒に常駐するため、2 回目以降の表示は即時です。`Escape` で閉じた場合の入力内容は次回まで保持されます。

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
- 登録ダイアログ内でもローマ字かな入力・候補変換・`q` / `Ctrl+Q` / `l` / `L` / `Ctrl+J` が使える
- 候補表示中の `x`: 前候補へ / 先頭で `x`: かな表示へ戻る
- 候補表示中の `X`: 表示中の候補をユーザー辞書・学習履歴から削除
- 候補表示中の `Ctrl+G`: 候補をキャンセルして変換バッファに戻る
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
- `Escape`: 変換中はキャンセル / 未変換状態ならパネルを閉じる (コピーせず、入力内容は次回まで保持)
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
| `userdict.json` | 単語登録したユーザー辞書 |
| `history.json` | 候補の学習履歴 (読みごと最大 8 件) |
| `input-history.json` | コピー履歴 (最大 30 件) |

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
{"op":"quit"}

// ← stdout
{"type":"ready","entries":123456,"dictionaries":["…/SKK-JISYO.L"],"dataDir":"…","configPath":"…"}
{"type":"state","text":"▽にほんご","cursor":5,"mode":"SKK 変換","candidate":"","candidateActive":false,
 "status":"Enter: copy / Shift+Enter: newline / Space: convert",
 "register":{"open":false,"reading":"","text":"","cursor":0,"mode":"","candidate":"","error":""},
 "close":false,"copied":false}
{"type":"error","message":"…"}
```

`text` は確定文字列にプリエディット (`▽…` や未確定ローマ字) を差し込んだ表示用文字列、`cursor` はその中のキャレット位置です。`close` が `true` なら UI はパネルを閉じます。

## 開発

```sh
make build      # bin/skk-popup-engine
make test       # go vet + go test
make validate   # manifest.json を omarchy plugin validate 相当でチェック (jq が必要)
make install    # ~/.local/bin にエンジン、~/.config/omarchy/plugins/takeshy.skk-popup にプラグインをコピー
```

Omarchy 上では `omarchy plugin validate .` と `qmllint -I "$OMARCHY_PATH/shell" Panel.qml` で検証できます。`~/.config/omarchy/plugins/` 配下のファイルを保存すると omarchy-shell が自動で再読み込みします (`omarchy-shell shell rescanPlugins` で強制再読み込み)。プラグインが再読み込みされると旧エンジンプロセスは stdin の EOF で終了し、新しいものが起動します。

構成:

```text
manifest.json              Omarchy plugin manifest (kinds: panel, bar-widget / keepLoaded)
Panel.qml                  入力パネル (エンジンの起動・キー送信・状態の描画)
BarWidget.qml              バーの「あ」ボタン
SkkButton.qml, SkkModeBadge.qml   パネル内の小さな部品
cmd/skk-popup-engine/      CLI (serve / dict fetch / dict list / version)
internal/skk/              SKK エンジン (ローマ字変換、状態機械、単語登録、辞書)
internal/store/            userdict.json / history.json / input-history.json の永続化
internal/config/           config.toml の読み込み
internal/clipboard/        wl-copy / wl-paste / wtype
```

## ライセンス

MIT
