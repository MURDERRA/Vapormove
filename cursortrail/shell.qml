import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland

ShellRoot {

    QtObject {
        id: cursor
        property real lastX: -9999
        property real lastY: -9999
        signal moved(real x, real y)
    }

    Process {
        id: cursorProc
        command: ["hyprctl", "cursorpos", "-j"]
        running: true

        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const pos = JSON.parse(this.text.trim())
                    if (pos && typeof pos.x === "number") {
                        const dx = pos.x - cursor.lastX
                        const dy = pos.y - cursor.lastY
                        if (cursor.lastX !== -9999 && (Math.abs(dx) > 0.5 || Math.abs(dy) > 0.5))
                            cursor.moved(pos.x, pos.y)
                        cursor.lastX = pos.x
                        cursor.lastY = pos.y
                    }
                } catch(e) {}
                cursorProc.running = false
                cursorProc.running = true
            }
        }
    }

    readonly property int trailLength: 18

    Component {
        id: dotComponent

        Item {
            id: ghost
            property real dotX: 0
            property real dotY: 0
            property int trailIndex: 0

            x: dotX - 50
            y: dotY- 10
            width: 24
            height: 28
            opacity: 0.85

            // При изменении индекса — перерисовываем Canvas
            onTrailIndexChanged: arrow.requestPaint()

            Canvas {
                id: arrow
                anchors.fill: parent

                Component.onCompleted: requestPaint()

                onPaint: {
                    const ctx = getContext("2d")
                    ctx.clearRect(0, 0, width, height)

                    // t=0 свежий (cyan у курсора), t=1 старый (violet хвост)
                    const t = Math.min(ghost.trailIndex / trailLength, 1.0)

                    // Интерполяция cyan → violet
                    const r1 = 0.05 + 0.5  * t
                    const g1 = 0.85 - 0.75 * t
                    const b1 = 1.0  - 0.05 * t

                    const r2 = 0.0  + 0.8  * t
                    const g2 = 0.95 - 0.85 * t
                    const b2 = 1.0  - 0.3  * t

                    const rs = 0.1  + 0.6  * t
                    const gs = 0.95 - 0.65 * t
                    const bs = 1.0

                    ctx.beginPath()
                    ctx.moveTo(0, 0)
                    ctx.lineTo(0, 20)
                    ctx.lineTo(4.5, 15.5)
                    ctx.lineTo(8, 22)
                    ctx.lineTo(10.5, 21)
                    ctx.lineTo(7, 14.5)
                    ctx.lineTo(13, 14.5)
                    ctx.closePath()

                    const grad = ctx.createLinearGradient(0, 0, 13, 22)
                    grad.addColorStop(0.0, Qt.rgba(r1, g1, b1, 0.9))
                    grad.addColorStop(1.0, Qt.rgba(r2, g2, b2, 0.75))
                    ctx.fillStyle = grad
                    ctx.fill()

                    ctx.strokeStyle = Qt.rgba(rs, gs, bs, 0.95)
                    ctx.lineWidth = 1.2
                    ctx.stroke()

                    ctx.beginPath()
                    ctx.moveTo(1, 1)
                    ctx.lineTo(1, 17)
                    ctx.strokeStyle = Qt.rgba(1.0, 0.95, 1.0, 0.4)
                    ctx.lineWidth = 0.8
                    ctx.stroke()
                }
            }

            NumberAnimation {
                running: true
                target: ghost; property: "opacity"
                from: 0.85; to: 0.0
                duration: 380
                easing.type: Easing.OutCubic
                onStopped: ghost.destroy()
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

            WlrLayershell.layer: WlrLayer.Overlay
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
            WlrLayershell.namespace: "vaportrail-cursor"

            mask: Region {}

            Connections {
                target: cursor
                function onMoved(x, y) {
                    const scr = overlay.screen
                    if (!scr) return
                    if (x < scr.x || x >= scr.x + scr.width)  return
                    if (y < scr.y || y >= scr.y + scr.height) return

                    // Увеличиваем индекс всем существующим ghost'ам
                    const children = dotLayer.children
                    for (let i = 0; i < children.length; i++) {
                        if (children[i].trailIndex !== undefined)
                            children[i].trailIndex++
                    }

                    // Новый ghost — всегда trailIndex: 0 (cyan)
                    dotComponent.createObject(dotLayer, {
                        dotX: x - scr.x,
                        dotY: y - scr.y,
                        trailIndex: 0
                    })
                }
            }

            Item { id: dotLayer; anchors.fill: parent }
        }
    }
}
