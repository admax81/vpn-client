//go:build windows

package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/user/vpn-client/internal/core"
	"github.com/user/vpn-client/internal/logger"
)

// RefreshConnectionWindow updates the connection window from the status listener.
func RefreshConnectionWindow(status *core.StatusPayload) {
	defer logger.Recover("RefreshConnectionWindow")

	connWindowMu.Lock()
	win := connWindow
	connWindowMu.Unlock()

	if win == nil || status == nil {
		return
	}

	win.Synchronize(func() {
		defer logger.Recover("cwRefreshUI-sync")
		cwRefreshUI(status)
	})
}

func cwRefreshUI(status *core.StatusPayload) {
	defer logger.Recover("cwRefreshUI")

	if status == nil {
		return
	}

	// Guard against nil widget pointers (window may be partially created/destroyed)
	if cwLblServer == nil || cwBtnConnect == nil || cwLblStatus == nil {
		return
	}

	switch status.State {
	case "connected":
		cwLblServer.SetText(status.ServerAddress)
		cwLblProtocol.SetText(strings.ToUpper(status.Protocol))
		cwLblLocalIP.SetText(status.LocalIP)
		cwLblSent.SetText(formatBytes(status.BytesSent))
		cwLblReceived.SetText(formatBytes(status.BytesReceived))
		if !status.ConnectedAt.IsZero() {
			cwLblUptime.SetText(formatDuration(time.Since(status.ConnectedAt)))
		}
		cwBtnConnect.SetText("Отключиться")
		cwBtnConnect.SetEnabled(true)
		cwLblStatus.SetText("Подключён")
		cwSetDot("connected")
		cwProtocolCB.SetEnabled(false)

	case "connecting":
		cwBtnConnect.SetText("Подключение...")
		cwBtnConnect.SetEnabled(false)
		cwLblStatus.SetText("Подключение...")
		cwSetDot("connecting")
		cwProtocolCB.SetEnabled(false)

	case "disconnecting":
		cwBtnConnect.SetText("Отключение...")
		cwBtnConnect.SetEnabled(false)
		cwLblStatus.SetText("Отключение...")
		cwSetDot("connecting")

	case "error":
		cwBtnConnect.SetText("Подключиться")
		cwBtnConnect.SetEnabled(true)
		cwLblStatus.SetText(fmt.Sprintf("Ошибка: %s", status.Error))
		cwSetDot("error")
		cwProtocolCB.SetEnabled(true)
		cwResetLabels()

	default: // disconnected
		cwBtnConnect.SetText("Подключиться")
		cwBtnConnect.SetEnabled(true)
		cwLblStatus.SetText("Отключён")
		cwSetDot("disconnected")
		cwProtocolCB.SetEnabled(true)
		cwResetLabels()
	}
}

func cwResetLabels() {
	cwLblServer.SetText("—")
	cwLblProtocol.SetText("—")
	cwLblUptime.SetText("00:00:00")
	cwLblSent.SetText("0 B")
	cwLblReceived.SetText("0 B")
	cwLblLocalIP.SetText("—")
}

func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = 1024 * 1024
		GB = 1024 * 1024 * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
