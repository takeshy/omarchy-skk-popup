import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import qs.Commons
import qs.Ui

// SKK Popup: a summoned floating input panel. Keys go to the Go engine
// (`skk-popup-engine serve`, JSON lines over stdin/stdout) and the engine
// answers with a render state; this file is a pure view of that state.
//
// The plugin is keepLoaded, so the engine (and its dictionaries) stay
// resident between summons and reopening is instant. Summon it with:
//   omarchy-shell shell toggle takeshy.skk-popup '{}'
Item {
  id: root

  property string omarchyPath: Quickshell.env("OMARCHY_PATH")
  property var shell: null
  property var manifest: null

  readonly property string pluginId: (manifest && manifest.id) ? String(manifest.id) : "takeshy.skk-popup"
  readonly property string pluginDir: (manifest && manifest.__sourceDir) ? String(manifest.__sourceDir) : ""

  property bool opened: false

  // ---- engine render state ----
  property bool engineReady: false
  property int engineRestarts: 0
  property string engineVersion: ""
  property string enginePath: ""
  property bool menuOpen: false
  property bool helpOpen: false
  readonly property string pluginVersion: (manifest && manifest.version) ? String(manifest.version) : ""

  readonly property string helpText:
    "── かな入力 ──\n" +
    "小文字ローマ字 → かな\n" +
    "大文字で開始 → 変換開始 (Nihongo → ▽にほんご)\n" +
    "変換中の大文字 → 送り仮名あり変換 (KanJi → 感じ)\n" +
    ";  sticky shift / 送り仮名開始位置\n" +
    "Space  変換 / 次候補\n" +
    "5候補目から一覧表示、A S D F J K L で選択 (Space 次頁 / x 前頁)\n" +
    "x  前候補へ / 先頭で x はかな表示へ\n" +
    "X  表示中の候補をユーザー辞書・学習履歴から削除\n" +
    "Ctrl+G  候補をキャンセルして変換バッファへ\n" +
    "Tab  過去に変換した読みから補完\n" +
    "候補なしで Space / 最終候補の次の Space  単語登録\n" +
    "読みに数字 → 数値変換 (だい5かい → 第５回 / 第五回)\n" +
    ">  接頭辞変換 (ちょう> → 超) / ▽> 接尾辞\n" +
    "q  カタカナで確定 (Ctrl+Q 半角カタカナ)\n" +
    "非変換の q / Ctrl+Q  カタカナ入力モード切替\n" +
    "l 英数モード / L 全角英数 / Ctrl+J かなモード\n" +
    "空のかな入力で /  Abbrev (▽/word)、// で / を入力\n" +
    "zh zj zk zl → ← ↓ ↑ → / z Space → 全角スペース\n" +
    "z. z, z- z/ z[ z] → … ‥ ～ ・ 『 』\n" +
    "\n── 編集・その他 ──\n" +
    "Shift+Enter  改行\n" +
    "Ctrl+V / Ctrl+Shift+V  クリップボードを挿入\n" +
    "Escape  変換中はキャンセル / 未変換なら閉じる (コピーせず保持)\n" +
    "↑ / ↓  コピー履歴 (最大30。↓ で下書きに戻る)\n" +
    "Enter (未変換) / Copy  コピーして閉じる\n" +
    "ヘッダーをドラッグ  パネル移動 / 右クリックで中央\n" +
    "Ctrl+Shift+K (表示中)  入力欄へフォーカス"
  // The engine binary was not found anywhere; offer to download it.
  property bool engineMissing: false
  property bool engineFetching: false
  property string engineFetchError: ""
  // True when the running engine is the copy fetch-engine.sh manages
  // (<data>/skk-popup/bin or a vendored <plugin dir>/bin), i.e. the one the
  // update button would actually replace. A manual ~/.local/bin or PATH
  // install, or a pinned $SKK_POPUP_ENGINE, is updated by its own means.
  readonly property bool engineManaged: root.enginePath.indexOf("/skk-popup/bin/") >= 0
    || (root.pluginDir.length > 0 && root.enginePath.indexOf(root.pluginDir + "/bin/") === 0)
  readonly property bool engineOverridden: {
    var v = Quickshell.env("SKK_POPUP_ENGINE")
    return !!v && v.length > 0
  }
  readonly property bool engineUpdatable: root.engineReady && root.engineManaged
    && !root.engineOverridden && !root.engineFetching
  property string bufferText: ""
  property int bufferCursor: 0
  property string modeLabel: "SKK かな"
  property string candidateText: ""
  property bool candidateActive: false
  property string statusText: "Starting skk-popup-engine…"
  property bool registerOpen: false
  property string registerReading: ""
  property string registerText: ""
  property int registerCursor: 0
  property string registerMode: "SKK かな"
  property string registerCandidate: ""
  property string registerError: ""

  // ---- theme ----
  // Shares the [popups] surface tokens, so themes that style the bar's
  // popups style this panel too.
  readonly property color background: Color.popups.background
  readonly property color foreground: Color.popups.text
  readonly property color accent: Color.accent
  readonly property color border: Color.popups.border
  readonly property var borderSpec: Border.surfaceSpec("popups", "border", border, Math.max(1, Style.space(2)))
  readonly property color scrim: Util.alpha(Color.background, 0.25)
  readonly property color fieldFill: Util.alpha(foreground, 0.06)
  readonly property color fieldBorder: Util.alpha(foreground, 0.25)
  readonly property int cornerRadius: Style.cornerRadius
  readonly property string fontFamily: Style.font.family
  readonly property int pad: Style.spacing.panelPadding
  readonly property int gap: Style.spacing.md
  readonly property int cardWidth: Math.min(Style.space(640), panel.width - Style.gapsOut * 2)
  readonly property int editorHeight: Style.space(132)

  // ---- card position (drag the header to move; persisted) ----
  // Mirrors store.DataDir(): $XDG_DATA_HOME/skk-popup or ~/.local/share/skk-popup.
  readonly property string dataHome: {
    var x = Quickshell.env("XDG_DATA_HOME")
    return (x && x.length > 0 ? x : Quickshell.env("HOME") + "/.local/share") + "/skk-popup"
  }
  // NaN on an axis means "centre that axis". Dragging the header sets these.
  property real cardX: NaN
  property real cardY: NaN

  function clampCardX(v) {
    var m = Style.gapsOut
    if (isNaN(v) || panel.width <= 0 || card.width <= 0) return Math.round((panel.width - card.width) / 2)
    return Math.round(Math.max(m, Math.min(v, panel.width - card.width - m)))
  }
  function clampCardY(v) {
    var m = Style.gapsOut
    if (isNaN(v) || panel.height <= 0 || card.height <= 0) return Math.round((panel.height - card.height) / 2)
    return Math.round(Math.max(m, Math.min(v, panel.height - card.height - m)))
  }
  function recenterCard() { root.cardX = NaN; root.cardY = NaN; cardPosFile.save() }
  function persistCardPos() { cardPosFile.save() }

  // ---- lifecycle (shell contract: open/close/opened) ----
  function open(payloadJson) {
    root.opened = true
    // Revive an engine that crashed past its restart budget, or start one
    // that never came up. `shown` is (re)sent from the `ready` handler once
    // the engine answers, so a cold first summon still captures the
    // clipboard even though this send() lands before the process is up.
    if (!engine.running) {
      root.engineRestarts = 0
      engine.running = true
    }
    root.send({ op: "shown" })
    Qt.callLater(function() { keyCatcher.forceActiveFocus() })
  }

  function close() {
    if (!root.opened) return
    root.opened = false
    root.send({ op: "hidden" })
  }

  function dismiss() {
    if (root.shell && typeof root.shell.hide === "function") root.shell.hide(root.pluginId)
    else root.close()
  }

  function toggle() {
    if (root.opened) root.dismiss()
    else root.open("{}")
  }

  // ---- engine protocol ----
  function send(obj) {
    if (!engine.running) return
    engine.write(JSON.stringify(obj) + "\n")
  }

  function keyRequest(event) {
    var name = ""
    switch (event.key) {
      case Qt.Key_Return:
      case Qt.Key_Enter: name = "Enter"; break
      case Qt.Key_Backspace: name = "Backspace"; break
      case Qt.Key_Escape: name = "Escape"; break
      case Qt.Key_Tab:
      case Qt.Key_Backtab: name = "Tab"; break
      case Qt.Key_Up: name = "Up"; break
      case Qt.Key_Down: name = "Down"; break
      case Qt.Key_Left: name = "Left"; break
      case Qt.Key_Right: name = "Right"; break
      case Qt.Key_Home: name = "Home"; break
      case Qt.Key_End: name = "End"; break
      case Qt.Key_Delete: name = "Delete"; break
      case Qt.Key_Space: name = " "; break
      default:
        if ((event.modifiers & Qt.ControlModifier) && event.key >= Qt.Key_A && event.key <= Qt.Key_Z) {
          // Ctrl+letter arrives as a control character; recover the letter.
          name = String.fromCharCode(event.key - Qt.Key_A + 97)
        } else if (event.text && event.text.length === 1) {
          var code = event.text.charCodeAt(0)
          if (code >= 32 && code < 127) name = event.text
        }
    }
    if (!name) return null
    return {
      op: "key",
      key: name,
      ctrl: (event.modifiers & Qt.ControlModifier) !== 0,
      shift: (event.modifiers & Qt.ShiftModifier) !== 0 || event.key === Qt.Key_Backtab,
      alt: (event.modifiers & Qt.AltModifier) !== 0
    }
  }

  function applyMessage(line) {
    var msg
    try { msg = JSON.parse(line) } catch (e) {
      console.warn("skk-popup: bad engine message:", line)
      return
    }
    if (msg.type === "ready") {
      root.engineReady = true
      root.engineMissing = false
      root.engineRestarts = 0
      root.engineVersion = msg.version || ""
      root.enginePath = msg.enginePath || ""
      if (!msg.dictionaries || msg.dictionaries.length === 0)
        root.statusText = "No dictionary. Run: skk-popup-engine dict fetch"
      // The engine came up (or restarted) while the panel is already open:
      // deliver the shown() the earlier send() could not.
      if (root.opened) root.send({ op: "shown" })
      return
    }
    if (msg.type === "error") {
      console.warn("skk-popup-engine:", msg.message)
      root.statusText = msg.message || "Engine error"
      return
    }
    if (msg.type !== "state") return

    root.bufferText = msg.text || ""
    root.bufferCursor = msg.cursor || 0
    root.modeLabel = msg.mode || ""
    root.candidateText = msg.candidate || ""
    root.candidateActive = msg.candidateActive === true
    root.statusText = msg.status || ""
    var reg = msg.register || {}
    root.registerOpen = reg.open === true
    root.registerReading = reg.reading || ""
    root.registerText = reg.text || ""
    root.registerCursor = reg.cursor || 0
    root.registerMode = reg.mode || ""
    root.registerCandidate = reg.candidate || ""
    root.registerError = reg.error || ""

    editor.text = root.bufferText
    editor.cursorPosition = root.bufferCursor
    registerEditor.text = root.registerText
    registerEditor.cursorPosition = root.registerCursor
    Qt.callLater(root.ensureCursorVisible)

    if (msg.close === true && root.opened) root.dismiss()
  }

  function ensureCursorVisible() {
    var r = editor.positionToRectangle(editor.cursorPosition)
    if (r.y < editorFlick.contentY) editorFlick.contentY = Math.max(0, r.y)
    else if (r.y + r.height > editorFlick.contentY + editorFlick.height)
      editorFlick.contentY = Math.max(0, r.y + r.height - editorFlick.height)
  }

  // The engine binary, in priority order: $SKK_POPUP_ENGINE, a copy vendored
  // in <plugin dir>/bin, the one fetch-engine.sh downloads into the data
  // dir, the usual manual install locations, then PATH. Exits 127 when
  // nothing is exec'able — that is what flips engineMissing on.
  readonly property string engineBootstrap: 'D="${XDG_DATA_HOME:-$HOME/.local/share}/skk-popup/bin/skk-popup-engine"; for p in "$SKK_POPUP_ENGINE" "$1/bin/skk-popup-engine" "$D" "$HOME/.local/bin/skk-popup-engine" "$HOME/go/bin/skk-popup-engine" /usr/local/bin/skk-popup-engine /usr/bin/skk-popup-engine; do [ -n "$p" ] && [ -x "$p" ] && exec "$p" serve; done; exec skk-popup-engine serve'

  // Downloads the prebuilt engine for this architecture into the data dir.
  readonly property string engineFetchScript: root.pluginDir + "/scripts/fetch-engine.sh"

  function fetchEngine() {
    if (root.engineFetching) return
    root.engineFetching = true
    root.engineFetchError = ""
    root.statusText = root.engineReady
      ? "skk-popup-engine を更新中…"
      : "skk-popup-engine をダウンロード中…"
    engineFetch.running = true
  }

  // Stop the running engine (if any) and start it again so it re-execs the
  // freshly downloaded binary.
  property bool engineRestartPending: false
  function restartEngine() {
    root.engineRestarts = 0
    if (engine.running) {
      root.engineRestartPending = true
      engine.running = false
    } else {
      engine.running = true
    }
  }

  // The shell injects `manifest` right after the item is created; wait for
  // that before starting so `<plugin dir>/bin` is on the lookup list.
  Component.onCompleted: Qt.callLater(function() { if (!engine.running) engine.running = true })

  Process {
    id: engine
    command: ["sh", "-c", root.engineBootstrap, "sh", root.pluginDir]
    running: false
    stdinEnabled: true
    stdout: SplitParser {
      onRead: function(data) { root.applyMessage(data) }
    }
    stderr: SplitParser {
      onRead: function(data) { console.warn("skk-popup-engine:", data) }
    }
    onExited: function(exitCode, exitStatus) {
      root.engineReady = false
      if (root.engineRestartPending) {
        // We stopped it on purpose (engine update); bring it straight back.
        root.engineRestartPending = false
        engine.running = true
        return
      }
      if (exitCode === 127) {
        // Nothing on the lookup path was exec'able.
        root.engineMissing = true
        root.statusText = "skk-popup-engine が見つかりません。取得してください。"
        return
      }
      root.statusText = "Engine exited (" + exitCode + ")."
      if (root.engineRestarts < 5) {
        root.engineRestarts += 1
        restartTimer.restart()
      }
    }
  }

  Timer {
    id: restartTimer
    interval: 2000
    onTriggered: engine.running = true
  }

  Process {
    id: engineFetch
    command: ["sh", root.engineFetchScript]
    running: false
    stdout: SplitParser { onRead: function(data) { console.warn("fetch-engine:", data) } }
    stderr: SplitParser {
      onRead: function(data) { console.warn("fetch-engine:", data); root.engineFetchError = data }
    }
    onExited: function(exitCode, exitStatus) {
      root.engineFetching = false
      if (exitCode === 0) {
        root.engineFetchError = ""
        root.statusText = "skk-popup-engine を起動中…"
        root.restartEngine()
      } else {
        root.statusText = root.engineFetchError !== ""
          ? "取得失敗: " + root.engineFetchError
          : "skk-popup-engine の取得に失敗しました (" + exitCode + ")"
      }
    }
  }

  // Remembers where the card was dragged, across restarts.
  FileView {
    id: cardPosFile
    path: root.dataHome + "/panel-position.json"
    atomicWrites: true
    printErrors: false
    onLoaded: {
      try {
        var p = JSON.parse(text())
        if (p && typeof p.x === "number") root.cardX = p.x
        if (p && typeof p.y === "number") root.cardY = p.y
      } catch (e) {}
    }
    function save() {
      var out = {}
      if (!isNaN(root.cardX)) out.x = Math.round(root.cardX)
      if (!isNaN(root.cardY)) out.y = Math.round(root.cardY)
      setText(JSON.stringify(out) + "\n")
    }
  }

  IpcHandler {
    target: "skk-popup"
    // Re-summoning while open just refocuses the field (never hides): the
    // hotkey is bound to `shell summon`, so pressing it again brings the
    // keyboard back to the panel instead of closing it.
    function show(): string {
      if (!root.opened) {
        if (root.shell && typeof root.shell.summon === "function") root.shell.summon(root.pluginId, "{}")
        else root.open("{}")
      } else {
        keyCatcher.forceActiveFocus()
      }
      return "ok"
    }
    function hide(): string { root.dismiss(); return "ok" }
    function toggle(): string { root.shell ? root.shell.toggle(root.pluginId, "{}") : root.toggle(); return "ok" }
    function state(): string { return root.opened ? "open" : "closed" }
    function ping(): string { return "ok" }
  }

  // One row of the ⋮ menu.
  component MenuRow: Rectangle {
    id: menuRow
    property string label: ""
    signal activated()
    width: parent ? parent.width : 0
    height: menuRowText.implicitHeight + Style.spacing.sm * 2
    radius: Math.max(2, Style.space(4))
    color: menuRowMouse.containsMouse && menuRow.enabled ? Util.alpha(root.foreground, 0.12) : "transparent"
    opacity: menuRow.enabled ? 1 : 0.4

    Text {
      id: menuRowText
      anchors.left: parent.left
      anchors.right: parent.right
      anchors.verticalCenter: parent.verticalCenter
      anchors.leftMargin: Style.spacing.sm
      anchors.rightMargin: Style.spacing.sm
      textFormat: Text.PlainText
      text: menuRow.label
      color: root.foreground
      font.family: root.fontFamily
      font.pixelSize: Style.font.body
      elide: Text.ElideRight
    }

    MouseArea {
      id: menuRowMouse
      anchors.fill: parent
      hoverEnabled: true
      enabled: menuRow.enabled
      cursorShape: Qt.PointingHandCursor
      onClicked: menuRow.activated()
    }
  }

  // ---- surface ----
  PanelWindow {
    id: panel
    visible: root.opened
    anchors { top: true; bottom: true; left: true; right: true }
    color: "transparent"
    WlrLayershell.namespace: "omarchy-skk-popup"
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.keyboardFocus: root.opened ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
    exclusionMode: ExclusionMode.Ignore

    Rectangle {
      anchors.fill: parent
      color: root.scrim
    }

    // Click outside closes without copying (the draft is kept).
    MouseArea {
      anchors.fill: parent
      onClicked: root.dismiss()
    }

    BorderSurface {
      id: card
      width: root.cardWidth
      height: content.implicitHeight + card.contentTopInset + card.contentBottomInset
      radius: root.cornerRadius
      // Centred until the header is dragged; clampCard* keeps it on screen
      // (and re-centres a NaN axis or a position left off a smaller output).
      x: root.clampCardX(root.cardX)
      y: root.clampCardY(root.cardY)
      color: root.background
      borderSpec: root.borderSpec
      padding: root.pad

      MouseArea { anchors.fill: parent; onClicked: function(mouse) { keyCatcher.forceActiveFocus(); mouse.accepted = true } }

      Item {
        id: keyCatcher
        anchors.fill: parent
        focus: true
        Keys.priority: Keys.BeforeItem
        Keys.onPressed: function(event) {
          if (root.menuOpen || root.helpOpen) {
            if (event.key === Qt.Key_Escape) { root.menuOpen = false; root.helpOpen = false }
            event.accepted = true
            return
          }
          var req = root.keyRequest(event)
          if (!req) return
          root.send(req)
          event.accepted = true
        }
      }

      Column {
        id: content
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.leftMargin: card.contentLeftInset
        anchors.rightMargin: card.contentRightInset
        anchors.topMargin: card.contentTopInset
        spacing: root.gap

        // Header: title + mode badge (click toggles かな / OFF).
        // Drag it to move the card; right-click re-centres.
        Item {
          id: header
          width: parent.width
          height: Math.max(menuButton.height, modeBadge.height, title.implicitHeight)

          HoverHandler { cursorShape: Qt.SizeAllCursor }

          DragHandler {
            target: null
            dragThreshold: 4
            property real baseX: 0
            property real baseY: 0
            onActiveChanged: {
              if (active) {
                baseX = card.x
                baseY = card.y
              } else {
                root.persistCardPos()
              }
            }
            onActiveTranslationChanged: {
              if (!active) return
              root.cardX = baseX + activeTranslation.x
              root.cardY = baseY + activeTranslation.y
            }
          }

          TapHandler {
            acceptedButtons: Qt.RightButton
            onTapped: root.recenterCard()
          }

          Text {
            id: title
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            textFormat: Text.PlainText
            text: "SKK Clipboard Input"
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.title
            font.bold: true
          }

          SkkModeBadge {
            id: modeBadge
            anchors.right: menuButton.left
            anchors.rightMargin: Style.spacing.sm
            anchors.verticalCenter: parent.verticalCenter
            label: root.modeLabel
            foreground: root.foreground
            accent: root.accent
            fontFamily: root.fontFamily
            onClicked: root.send({ op: "toggleMode" })
          }

          SkkButton {
            id: menuButton
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            label: "⋮"
            foreground: root.foreground
            accent: root.accent
            fontFamily: root.fontFamily
            onClicked: root.menuOpen = !root.menuOpen
          }
        }

        // The input buffer. Read-only: the engine owns text and caret.
        BorderSurface {
          width: parent.width
          height: root.editorHeight
          radius: root.cornerRadius
          color: root.fieldFill
          borderSpec: Border.controlSpec(keyCatcher.activeFocus && !root.registerOpen ? "focus" : "normal", root.foreground, root.accent)
          padding: Style.spacing.lg

          Flickable {
            id: editorFlick
            anchors.fill: parent
            anchors.margins: Style.spacing.lg
            clip: true
            contentWidth: width
            contentHeight: editor.implicitHeight
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height

            TextEdit {
              id: editor
              width: editorFlick.width
              readOnly: true
              selectByMouse: false
              wrapMode: TextEdit.Wrap
              textFormat: TextEdit.PlainText
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.heading
              activeFocusOnPress: false

              MouseArea {
                anchors.fill: parent
                cursorShape: Qt.IBeamCursor
                onClicked: function(mouse) {
                  root.send({ op: "setCursor", pos: editor.positionAt(mouse.x, mouse.y) })
                  keyCatcher.forceActiveFocus()
                }
              }

              // The engine owns the caret and this field never takes active
              // focus, so TextEdit's built-in cursor never paints. Draw our
              // own at positionToRectangle() — the comma list forces the
              // binding to re-run whenever the text, caret offset or width
              // changes.
              Rectangle {
                id: mainCaret
                readonly property rect cr: (editor.text, editor.cursorPosition, editor.width,
                  editor.positionToRectangle(editor.cursorPosition))
                x: cr.x
                y: cr.y
                width: Math.max(2, Style.space(2))
                height: cr.height > 0 ? cr.height : editor.font.pixelSize
                color: root.accent
                visible: root.opened && !root.registerOpen
                Timer {
                  running: mainCaret.visible
                  repeat: true
                  interval: 530
                  onTriggered: mainCaret.opacity = mainCaret.opacity > 0 ? 0 : 1
                }
              }
            }
          }
        }

        // Candidate line: the current candidate with its annotation, or
        // the A S D F J K L list from the fifth candidate on.
        Rectangle {
          width: parent.width
          height: Math.max(Style.space(26), candidateLabel.implicitHeight + Style.spacing.sm * 2)
          radius: root.cornerRadius
          color: root.candidateActive ? Util.alpha(root.accent, 0.16) : "transparent"

          Text {
            id: candidateLabel
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: Style.spacing.lg
            anchors.rightMargin: Style.spacing.lg
            textFormat: Text.PlainText
            text: root.candidateActive ? root.candidateText : ""
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.subtitle
            elide: Text.ElideRight
          }
        }

        // Footer: status + Copy / Close.
        Item {
          width: parent.width
          height: Math.max(status.implicitHeight, footerButtons.height)

          Text {
            id: status
            anchors.left: parent.left
            anchors.right: footerButtons.left
            anchors.rightMargin: root.gap
            anchors.verticalCenter: parent.verticalCenter
            textFormat: Text.PlainText
            text: root.statusText
            color: root.foreground
            opacity: root.engineReady ? 0.7 : 1
            font.family: root.fontFamily
            font.pixelSize: Style.font.bodySmall
            elide: Text.ElideRight
          }

          Row {
            id: footerButtons
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: root.gap

            SkkButton { label: "Close"; foreground: root.foreground; accent: root.accent; fontFamily: root.fontFamily; onClicked: root.dismiss() }
            SkkButton {
              visible: root.engineMissing || root.engineFetching
              label: root.engineFetching ? "取得中…" : "エンジンを取得"
              primary: true
              opacity: root.engineFetching ? 0.6 : 1
              foreground: root.foreground; accent: root.accent; fontFamily: root.fontFamily
              onClicked: root.fetchEngine()
            }
            SkkButton {
              visible: !root.engineMissing && !root.engineFetching
              label: "Copy"; primary: true
              foreground: root.foreground; accent: root.accent; fontFamily: root.fontFamily
              onClicked: root.send({ op: "copy" })
            }
          }
        }
      }

      // ⋮ menu: version line + recentre / help / engine update.
      MouseArea {
        anchors.fill: parent
        visible: root.menuOpen
        onClicked: root.menuOpen = false

        BorderSurface {
          anchors.top: parent.top
          anchors.right: parent.right
          anchors.topMargin: card.contentTopInset + Style.space(38)
          anchors.rightMargin: card.contentRightInset
          width: Math.min(Style.space(300), card.width - card.contentLeftInset - card.contentRightInset)
          height: menuCol.implicitHeight + Style.spacing.md * 2
          radius: root.cornerRadius
          color: root.background
          borderSpec: root.borderSpec

          MouseArea { anchors.fill: parent; onClicked: function(mouse) { mouse.accepted = true } }

          Column {
            id: menuCol
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Style.spacing.md
            spacing: Style.spacing.xs

            Text {
              width: parent.width
              textFormat: Text.PlainText
              text: "SKK Popup" + (root.pluginVersion ? " v" + root.pluginVersion : "")
                + "  ・  engine " + (root.engineReady ? (root.engineVersion || "?")
                : (root.engineFetching ? "取得中…" : "未取得"))
              color: root.foreground
              opacity: 0.7
              font.family: root.fontFamily
              font.pixelSize: Style.font.bodySmall
              elide: Text.ElideRight
            }

            MenuRow {
              label: "パネルを中央に戻す"
              onActivated: { root.recenterCard(); root.menuOpen = false }
            }
            MenuRow {
              label: "ヘルプ (キー操作)"
              onActivated: { root.menuOpen = false; root.helpOpen = true }
            }
            MenuRow {
              visible: root.engineReady || root.engineFetching
              enabled: root.engineUpdatable
              label: root.engineFetching
                ? "エンジンを更新中…"
                : (root.engineUpdatable ? "エンジンを更新" : "エンジンを更新 (手動版は対象外)")
              onActivated: { root.fetchEngine(); root.menuOpen = false }
            }
          }
        }
      }

      // Help overlay: scrollable key-operation cheat sheet.
      Rectangle {
        anchors.fill: parent
        radius: root.cornerRadius
        color: Util.alpha(root.background, 0.98)
        visible: root.helpOpen

        MouseArea { anchors.fill: parent; onClicked: function(mouse) { mouse.accepted = true } }

        Text {
          id: helpTitle
          anchors { top: parent.top; left: parent.left; topMargin: root.pad; leftMargin: root.pad }
          textFormat: Text.PlainText
          text: "キー操作"
          color: root.foreground
          font.family: root.fontFamily
          font.pixelSize: Style.font.title
          font.bold: true
        }

        SkkButton {
          anchors { top: parent.top; right: parent.right; topMargin: root.pad; rightMargin: root.pad }
          label: "閉じる"
          foreground: root.foreground
          accent: root.accent
          fontFamily: root.fontFamily
          onClicked: root.helpOpen = false
        }

        Flickable {
          anchors {
            left: parent.left; right: parent.right; bottom: parent.bottom; top: helpTitle.bottom
            leftMargin: root.pad; rightMargin: root.pad; bottomMargin: root.pad; topMargin: root.gap
          }
          clip: true
          contentWidth: width
          contentHeight: helpBody.implicitHeight
          boundsBehavior: Flickable.StopAtBounds

          Text {
            id: helpBody
            width: parent.width
            textFormat: Text.PlainText
            text: root.helpText
            wrapMode: Text.Wrap
            lineHeight: 1.35
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.bodySmall
          }
        }
      }

      // Word registration dialog, drawn over the card.
      Rectangle {
        anchors.fill: parent
        radius: root.cornerRadius
        color: Util.alpha(root.background, 0.92)
        visible: root.registerOpen

        MouseArea { anchors.fill: parent; onClicked: function(mouse) { mouse.accepted = true } }

        Column {
          anchors.left: parent.left
          anchors.right: parent.right
          anchors.verticalCenter: parent.verticalCenter
          anchors.leftMargin: root.pad
          anchors.rightMargin: root.pad
          spacing: root.gap

          Text {
            textFormat: Text.PlainText
            text: "単語登録"
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.title
            font.bold: true
          }

          Item {
            width: parent.width
            height: Math.max(registerModeBadge.height, registerPrompt.implicitHeight)

            Text {
              id: registerPrompt
              anchors.left: parent.left
              anchors.right: registerModeBadge.left
              anchors.rightMargin: root.gap
              anchors.verticalCenter: parent.verticalCenter
              textFormat: Text.PlainText
              text: root.registerReading + " に登録する単語を入力してください。"
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.body
              elide: Text.ElideRight
            }

            SkkModeBadge {
              id: registerModeBadge
              anchors.right: parent.right
              anchors.verticalCenter: parent.verticalCenter
              label: root.registerMode
              foreground: root.foreground
              accent: root.accent
              fontFamily: root.fontFamily
              onClicked: root.send({ op: "registerToggleMode" })
            }
          }

          BorderSurface {
            width: parent.width
            height: registerEditor.implicitHeight + Style.spacing.lg * 2
            radius: root.cornerRadius
            color: root.fieldFill
            borderSpec: Border.controlSpec(root.registerOpen ? "focus" : "normal", root.foreground, root.accent)

            TextEdit {
              id: registerEditor
              anchors.fill: parent
              anchors.margins: Style.spacing.lg
              readOnly: true
              selectByMouse: false
              wrapMode: TextEdit.NoWrap
              textFormat: TextEdit.PlainText
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.heading
              activeFocusOnPress: false

              Rectangle {
                id: regCaret
                readonly property rect cr: (registerEditor.text, registerEditor.cursorPosition, registerEditor.width,
                  registerEditor.positionToRectangle(registerEditor.cursorPosition))
                x: cr.x
                y: cr.y
                width: Math.max(2, Style.space(2))
                height: cr.height > 0 ? cr.height : registerEditor.font.pixelSize
                color: root.accent
                visible: root.registerOpen
                Timer {
                  running: regCaret.visible
                  repeat: true
                  interval: 530
                  onTriggered: regCaret.opacity = regCaret.opacity > 0 ? 0 : 1
                }
              }
            }
          }

          Text {
            width: parent.width
            textFormat: Text.PlainText
            text: root.registerCandidate
            visible: text !== ""
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.subtitle
            elide: Text.ElideRight
          }

          Text {
            width: parent.width
            textFormat: Text.PlainText
            text: root.registerError
            visible: text !== ""
            color: Color.urgent
            font.family: root.fontFamily
            font.pixelSize: Style.font.bodySmall
            wrapMode: Text.Wrap
          }

          Row {
            anchors.right: parent.right
            spacing: root.gap

            SkkButton { label: "閉じる"; foreground: root.foreground; accent: root.accent; fontFamily: root.fontFamily; onClicked: root.send({ op: "registerCancel" }) }
            SkkButton { label: "登録"; primary: true; foreground: root.foreground; accent: root.accent; fontFamily: root.fontFamily; onClicked: root.send({ op: "registerSave" }) }
          }
        }
      }
    }
  }
}
