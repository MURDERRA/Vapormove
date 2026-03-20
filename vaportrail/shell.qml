import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Quickshell.Hyprland

ShellRoot {

    QtObject {
        id: tracker

        property int lastX: -9999
        property int lastY: -9999
        property bool isMoving: false
        property bool fetchPending: false

        signal ghostSpawn(int x, int y, int w, int h)

        // Таймер окончания движения — если activewindow не пришёл 150мс
        property Timer idleTimer: Timer {
            interval: 150
            repeat: false
            onTriggered: tracker.isMoving = false
        }

        function onActiveWindow() {
            isMoving = true
            idleTimer.restart()
            // Запускаем цепочку только если она не идёт уже
            if (!hyprctlProc.running) {
                hyprctlProc.running = false
                hyprctlProc.running = true
            }
        }
    }

    Connections {
        target: Hyprland
        function onRawEvent(event) {
            if (event.name === "activewindow" || event.name === "activewindowv2") {
                tracker.onActiveWindow()
            }
        }
    }

    Process {
        id: hyprctlProc
        command: ["hyprctl", "activewindow", "-j"]
        running: false

        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const win = JSON.parse(this.text.trim())
                    if (!win || !win.at || !win.size || win.size[0] <= 0) {
                        // Продолжаем цепочку если ещё двигаемся
                        if (tracker.isMoving) {
                            hyprctlProc.running = false
                            hyprctlProc.running = true
                        }
                        return
                    }

                    const x = win.at[0]
                    const y = win.at[1]
                    const w = win.size[0]
                    const h = win.size[1]

                    const dx = Math.abs(x - tracker.lastX)
                    const dy = Math.abs(y - tracker.lastY)

                    // Спавним ghost на СТАРОЙ позиции (lastX/lastY) — это и есть след
                    if (tracker.lastX !== -9999 && (dx > 3 || dy > 3)) {
                        tracker.ghostSpawn(tracker.lastX, tracker.lastY, w, h)
                    }

                    // Обновляем позицию ПОСЛЕ спавна
                    tracker.lastX = x
                    tracker.lastY = y

                } catch (e) {}

                // Цепочка: сразу перезапускаем пока окно двигается
                if (tracker.isMoving) {
                    hyprctlProc.running = false
                    hyprctlProc.running = true
                }
            }
        }
    }

    Component {
        id: ghostComponent

        Rectangle {
            id: self
            property int ghostX: 0
            property int ghostY: 0
            property int ghostW: 200
            property int ghostH: 100

            x: ghostX
            y: ghostY
            width: ghostW
            height: ghostH
            radius: 12
            opacity: 1.0

            gradient: Gradient {
                orientation: Gradient.Horizontal
                GradientStop { position: 0.0;  color: Qt.rgba(0.05, 0.25, 1.0,  0.55) }
                GradientStop { position: 0.4;  color: Qt.rgba(0.45, 0.05, 0.9,  0.5)  }
                GradientStop { position: 0.75; color: Qt.rgba(0.65, 0.0,  0.75, 0.45) }
                GradientStop { position: 1.0;  color: Qt.rgba(0.85, 0.05, 0.55, 0.4)  }
            }

            border.width: 1
            border.color: Qt.rgba(0.75, 0.35, 1.0, 0.85)

            // Scanline highlight
            Rectangle {
                anchors { top: parent.top; left: parent.left; right: parent.right; margins: 4 }
                height: 1.5; radius: 1
                color: Qt.rgba(0.85, 0.65, 1.0, 0.55)
            }

            SequentialAnimation {
                running: true
                PauseAnimation  { duration: 40 }
                NumberAnimation {
                    target: self; property: "opacity"
                    from: 1.0; to: 0.0
                    duration: 300
                    easing.type: Easing.OutCubic
                }
                ScriptAction { script: self.destroy() }
            }
        }
    }

    Variants {
        model: Quickshell.screens

        PanelWindow {
            id: overlay
            property var modelData

            screen: modelData
            color: "transparent"
            anchors { top: true; bottom: true; left: true; right: true }

            WlrLayershell.layer: WlrLayer.Bottom
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
            WlrLayershell.namespace: "vaportrail"

            mask: Region {}

            Connections {
                target: tracker
                function onGhostSpawn(x, y, w, h) {
                    const scr = overlay.screen
                    if (!scr) return
                    if (x + w <= scr.x || x >= scr.x + scr.width)  return
                    if (y + h <= scr.y || y >= scr.y + scr.height) return

                    ghostComponent.createObject(ghostLayer, {
                        ghostX: x - scr.x,
                        ghostY: y - scr.y,
                        ghostW: w,
                        ghostH: h
                    })
                }
            }

            Item {
                id: ghostLayer
                anchors.fill: parent
            }
        }
    }
}
