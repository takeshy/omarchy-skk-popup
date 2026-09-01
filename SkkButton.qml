import QtQuick
import qs.Commons

// Footer / dialog action button. `primary` paints it with the accent.
Rectangle {
  id: button

  property string label: ""
  property bool primary: false
  property color foreground: Color.popups.text
  property color accent: Color.accent
  property string fontFamily: Style.font.family
  signal clicked()

  width: buttonText.implicitWidth + Style.spacing.controlPaddingX * 2
  height: Style.spacing.controlHeight
  radius: Style.cornerRadius
  color: primary
    ? (buttonMouse.containsMouse ? Util.alpha(accent, 0.5) : Util.alpha(accent, 0.35))
    : (buttonMouse.containsMouse ? Util.alpha(foreground, 0.16) : Util.alpha(foreground, 0.08))
  border.width: Math.max(1, Style.normalBorderWidth)
  border.color: primary ? Util.alpha(accent, 0.7) : Util.alpha(foreground, 0.3)

  Text {
    id: buttonText
    anchors.centerIn: parent
    textFormat: Text.PlainText
    text: button.label
    color: button.foreground
    font.family: button.fontFamily
    font.pixelSize: Style.font.body
    font.bold: button.primary
  }

  MouseArea {
    id: buttonMouse
    anchors.fill: parent
    hoverEnabled: true
    cursorShape: Qt.PointingHandCursor
    onClicked: button.clicked()
  }
}
