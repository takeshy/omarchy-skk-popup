import QtQuick
import qs.Commons

// Mode label (SKK かな / SKK OFF / ...). Clicking it toggles kana ↔ ASCII.
Rectangle {
  id: badge

  property string label: ""
  property color foreground: Color.popups.text
  property color accent: Color.accent
  property string fontFamily: Style.font.family
  signal clicked()

  width: badgeText.implicitWidth + Style.spacing.controlPaddingX * 2
  height: badgeText.implicitHeight + Style.spacing.xs * 2
  radius: Style.cornerRadius
  color: badgeMouse.containsMouse ? Util.alpha(accent, 0.3) : Util.alpha(accent, 0.18)

  Text {
    id: badgeText
    anchors.centerIn: parent
    textFormat: Text.PlainText
    text: badge.label
    color: badge.foreground
    font.family: badge.fontFamily
    font.pixelSize: Style.font.bodySmall
    font.bold: true
  }

  MouseArea {
    id: badgeMouse
    anchors.fill: parent
    hoverEnabled: true
    cursorShape: Qt.PointingHandCursor
    onClicked: badge.clicked()
  }
}
