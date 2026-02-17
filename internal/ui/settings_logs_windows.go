//go:build windows

package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"

	"github.com/user/vpn-client/internal/logger"
	"github.com/user/vpn-client/internal/procutil"
)

// loadLogs reads and displays VPN logs in the text edit widget.
func loadLogs(textEdit *walk.TextEdit) {
	content, err := logger.ReadLogs()
	if err != nil {
		if os.IsNotExist(err) {
			textEdit.SetText("Логов пока нет. Подключитесь к VPN, чтобы увидеть логи.")
		} else {
			textEdit.SetText("Ошибка чтения логов: " + err.Error())
		}
		return
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 1000 {
		lines = lines[len(lines)-1000:]
	}

	textEdit.SetText(strings.Join(lines, "\r\n"))
	textEdit.SetTextSelection(len(textEdit.Text()), len(textEdit.Text()))
}

// openLogFile opens the log file in the default editor.
func openLogFile() {
	logPath := logger.GetLogPath()
	if logPath == "" {
		logPath = filepath.Join(os.Getenv("PROGRAMDATA"), "VPNClient", "vpn.log")
	}
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		dir := filepath.Dir(logPath)
		os.MkdirAll(dir, 0755)
		os.WriteFile(logPath, []byte("VPN Client Log\n"), 0644)
	}
	cmd := exec.Command("cmd", "/c", "start", "", logPath)
	procutil.HideWindow(cmd)
	cmd.Start()
}
