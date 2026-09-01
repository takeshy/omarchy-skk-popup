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

  // ---- lifecycle (shell contract: open/close/opened) ----
  function open(payloadJson) {
    root.opened = true
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
      root.engineRestarts = 0
      if (!msg.dictionaries || msg.dictionaries.length === 0)
        root.statusText = "No dictionary. Run: skk-popup-engine dict fetch"
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
    var r = editor.cursorRectangle
    if (r.y < editorFlick.contentY) editorFlick.contentY = Math.max(0, r.y)
    else if (r.y + r.height > editorFlick.contentY + editorFlick.height)
      editorFlick.contentY = Math.max(0, r.y + r.height - editorFlick.height)
  }

  // The engine binary: $SKK_POPUP_ENGINE, then <plugin dir>/bin, then the
  // usual user install locations, then PATH.
  readonly property string engineBootstrap: 'for p in "$SKK_POPUP_ENGINE" "$1/bin/skk-popup-engine" "$HOME/.local/bin/skk-popup-engine" "$HOME/go/bin/skk-popup-engine" /usr/local/bin/skk-popup-engine /usr/bin/skk-popup-engine; do [ -n "$p" ] && [ -x "$p" ] && exec "$p" serve; done; exec skk-popup-engine serve'

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
      root.statusText = "Engine exited (" + exitCode + "). Install skk-popup-engine and reopen."
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

  IpcHandler {
    target: "skk-popup"
    function show(): string { if (!root.opened) root.shell && root.shell.summon(root.pluginId, "{}"); return "ok" }
    function hide(): string { root.dismiss(); return "ok" }
    function toggle(): string { root.shell ? root.shell.toggle(root.pluginId, "{}") : root.toggle(); return "ok" }
    function state(): string { return root.opened ? "open" : "closed" }
    function ping(): string { return "ok" }
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
      anchors.centerIn: parent
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
        Item {
          width: parent.width
          height: Math.max(modeBadge.height, title.implicitHeight)

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
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            label: root.modeLabel
            foreground: root.foreground
            accent: root.accent
            fontFamily: root.fontFamily
            onClicked: root.send({ op: "toggleMode" })
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
              cursorVisible: root.opened && !root.registerOpen
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
            SkkButton { label: "Copy"; primary: true; foreground: root.foreground; accent: root.accent; fontFamily: root.fontFamily; onClicked: root.send({ op: "copy" }) }
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
              cursorVisible: root.registerOpen
              selectByMouse: false
              wrapMode: TextEdit.NoWrap
              textFormat: TextEdit.PlainText
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.heading
              activeFocusOnPress: false
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
