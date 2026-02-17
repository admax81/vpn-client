// Package ui provides the system tray UI for the VPN client.
package ui

import (
	"fmt"
	"log"
	"runtime"

	"fyne.io/systray"

	"github.com/user/vpn-client/internal/config"
	"github.com/user/vpn-client/internal/core"
	"github.com/user/vpn-client/internal/logger"
)

var (
	service      *core.Service
	currentState string = "disconnected"
	settingsOpen bool   = false

	// Systray menu items
	mStatus     *systray.MenuItem
	mConnect    *systray.MenuItem
	mDisconnect *systray.MenuItem
	mConnMon    *systray.MenuItem
	mSettings   *systray.MenuItem
	mQuit       *systray.MenuItem
	connMonOpen bool = false
)

// Run starts the combined VPN + UI application
func Run() {
	// Initialize logger
	logger.Init()
	logger.ClearLogs()
	logger.Info("VPN Client starting (combined mode)")

	// Create the VPN service
	var err error
	service, err = core.NewService(config.GetConfigPath())
	if err != nil {
		log.Fatalf("Failed to create VPN service: %v", err)
	}

	// Set status listener — UI will be updated on every state change
	service.SetStatusListener(func(status *core.StatusPayload) {
		updateUI(status)
	})

	// Start the service (handles auto-connect if configured)
	if err := service.Start(); err != nil {
		log.Fatalf("Failed to start VPN service: %v", err)
	}

	// Start systray (blocks until quit)
	systray.Run(onReady, onExit)
}

// onReady is called when systray is ready
func onReady() {
	// Set icon
	systray.SetIcon(GetIcon("disconnected"))
	systray.SetTitle("VPN Client")
	systray.SetTooltip("VPN Client — Отключён")

	// Left click on tray icon opens connection window
	systray.SetOnTapped(func() {
		go ShowConnectionWindow()
	})

	// Create menu items
	mStatus = systray.AddMenuItem("Статус: Отключён", "")
	mStatus.Disable()

	systray.AddSeparator()

	mConnect = systray.AddMenuItem("Подключиться", "")
	mDisconnect = systray.AddMenuItem("Отключиться", "")
	mDisconnect.Disable()

	systray.AddSeparator()

	mConnMon = systray.AddMenuItem("Монитор соединений", "")
	mSettings = systray.AddMenuItem("Показать", "")

	systray.AddSeparator()

	mQuit = systray.AddMenuItem("Выход", "")

	// Refresh initial status from service
	updateUI(service.GetStatusPayload())

	// Handle menu clicks
	go func() {
		defer logger.Recover("systray-menu-loop")
		for {
			select {
			case <-mConnect.ClickedCh:
				go doConnect()
			case <-mDisconnect.ClickedCh:
				go doDisconnect()
			case <-mConnMon.ClickedCh:
				go openConnMon()
			case <-mSettings.ClickedCh:
				go ShowConnectionWindow()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// onExit is called when systray exits
func onExit() {
	logger.Info("VPN Client shutting down")
	if service != nil {
		service.Stop()
	}
	logger.Close()
}

func doConnect() {
	defer logger.Recover("doConnect")
	logger.Connection("User initiated VPN connection")

	mConnect.Disable()
	mStatus.SetTitle("Статус: Подключение...")
	systray.SetTooltip("VPN Client — Подключение...")
	systray.SetIcon(GetIcon("connecting"))

	if err := service.Connect(); err != nil {
		logger.Error("Failed to connect: %v", err)
		showError(fmt.Sprintf("Failed to connect: %v", err))
		// UI will be updated by the status listener
	}
}

func doDisconnect() {
	defer logger.Recover("doDisconnect")
	logger.Connection("User initiated VPN disconnection")

	mDisconnect.Disable()
	mStatus.SetTitle("Статус: Отключение...")
	systray.SetTooltip("VPN Client — Отключение...")

	if err := service.Disconnect(); err != nil {
		logger.Error("Failed to disconnect: %v", err)
		showError(fmt.Sprintf("Failed to disconnect: %v", err))
	}
}

func updateUI(status *core.StatusPayload) {
	defer logger.Recover("updateUI")

	if status == nil {
		return
	}

	prevState := currentState
	currentState = status.State

	// Log state changes
	if prevState != currentState {
		switch status.State {
		case "connected":
			logger.Connection("VPN connected successfully via %s", status.Protocol)
			logger.Info("Server: %s, Local IP: %s", status.ServerAddress, status.LocalIP)
		case "connecting":
			logger.Connection("VPN connecting...")
		case "disconnected":
			logger.Connection("VPN disconnected")
		case "error":
			logger.Error("VPN connection error: %s", status.Error)
		}
	}

	switch status.State {
	case "connected":
		mStatus.SetTitle(fmt.Sprintf("Статус: Подключён (%s)", status.Protocol))
		systray.SetTooltip(fmt.Sprintf("VPN Client — Подключён\nСервер: %s\nЛокальный IP: %s",
			status.ServerAddress, status.LocalIP))
		systray.SetIcon(GetIcon("connected"))
		mConnect.Disable()
		mDisconnect.Enable()

	case "connecting":
		mStatus.SetTitle("Статус: Подключение...")
		systray.SetTooltip("VPN Client — Подключение...")
		systray.SetIcon(GetIcon("connecting"))
		mConnect.Disable()
		mDisconnect.Disable()

	case "disconnecting":
		mStatus.SetTitle("Статус: Отключение...")
		systray.SetTooltip("VPN Client — Отключение...")
		systray.SetIcon(GetIcon("connecting"))
		mConnect.Disable()
		mDisconnect.Disable()

	case "error":
		mStatus.SetTitle(fmt.Sprintf("Статус: Ошибка — %s", status.Error))
		systray.SetTooltip(fmt.Sprintf("VPN Client — Ошибка\n%s", status.Error))
		systray.SetIcon(GetIcon("error"))
		mConnect.Enable()
		mDisconnect.Disable()

	default: // disconnected
		mStatus.SetTitle("Статус: Отключён")
		systray.SetTooltip("VPN Client — Отключён")
		systray.SetIcon(GetIcon("disconnected"))
		mConnect.Enable()
		mDisconnect.Disable()
	}

	// Update connection window if open
	RefreshConnectionWindow(status)
}

func openSettings() {
	if settingsOpen {
		return
	}
	settingsOpen = true

	go func() {
		defer logger.Recover("openSettings")
		runtime.LockOSThread()
		defer func() {
			settingsOpen = false
			runtime.UnlockOSThread()
		}()
		ShowSettingsWindow()
	}()
}

func openConnMon() {
	if connMonOpen {
		ShowConnMonWindow()
		return
	}
	connMonOpen = true

	go func() {
		defer logger.Recover("openConnMon")
		defer func() {
			connMonOpen = false
		}()
		ShowConnMonWindow()
	}()
}

func showError(message string) {
	log.Printf("Error: %s", message)
	// TODO: Show notification or message box
}
