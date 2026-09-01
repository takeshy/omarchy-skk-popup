import QtQuick
import qs.Commons
import qs.Ui

// 「あ」 button for the bar: toggles the SKK Popup panel. The panel itself
// lives in Panel.qml and is owned by the shell's panel loader, so this
// widget only asks the shell to summon/hide it.
BarWidget {
  id: root
  moduleName: "takeshy.skk-popup"

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  function togglePanel() {
    if (root.bar && root.bar.shell && typeof root.bar.shell.toggle === "function")
      root.bar.shell.toggle(root.moduleName, "{}")
  }

  BarIconButton {
    id: button
    bar: root.bar
    text: "あ"
    tooltipText: "SKK Popup"
    onPressed: function(mouseButton) {
      if (mouseButton === Qt.LeftButton) root.togglePanel()
    }
  }
}
