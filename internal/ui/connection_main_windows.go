//go:build windows

package ui

import (
	"sync"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/user/vpn-client/internal/logger"
)

const appVersion = "1.0.1"

var (
	connWindow   *walk.MainWindow
	connWindowMu sync.Mutex

	// Connection window widget refs
	cwLblServer   *walk.Label
	cwLblProtocol *walk.Label
	cwLblUptime   *walk.Label
	cwLblSent     *walk.Label
	cwLblReceived *walk.Label
	cwLblLocalIP  *walk.Label
	cwLblStatus   *walk.Label
	cwDotImage    *walk.ImageView
	cwBtnConnect  *walk.PushButton
	cwProtocolCB  *walk.ComboBox
	cwStopTicker  chan struct{}
)

// ShowConnectionWindow displays the main connection/status window.
func ShowConnectionWindow() {
	connWindowMu.Lock()
	if connWindow != nil {
		win := connWindow
		connWindowMu.Unlock()
		win.Synchronize(func() {
			win.Show()
		})
		return
	}
	connWindowMu.Unlock()

	windowIcon := createWindowIcon()
	appCfg := loadAppConfig()

	var mw *walk.MainWindow

	if err := (MainWindow{
		AssignTo: &mw,
		Title:    "VPN Client",
		MinSize:  Size{Width: 300, Height: 420},
		Size:     Size{Width: 340, Height: 480},
		MaxSize:  Size{Width: 420, Height: 580},
		Layout:   VBox{Margins: Margins{Left: 16, Top: 12, Right: 16, Bottom: 8}},
		Children: []Widget{
			// Top row: settings gear button + connmon
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					PushButton{
						Text:    "⚙ Настройки",
						MaxSize: Size{Width: 110, Height: 28},
						OnClicked: func() {
							openSettings()
						},
					},
					HSpacer{},
					PushButton{
						Text:    "🔍 Маршруты",
						MaxSize: Size{Width: 110, Height: 28},
						OnClicked: func() {
							go openConnMon()
						},
					},
				},
			},

			VSpacer{Size: 10},

			// Connection info group
			GroupBox{
				Title:  "Информация о подключении",
				Layout: VBox{Spacing: 2},
				Children: []Widget{
					Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
						Label{Text: "Сервер:", Font: Font{Bold: true}, MinSize: Size{Width: 120}, MaxSize: Size{Width: 120}},
						Label{AssignTo: &cwLblServer, Text: "—"},
						HSpacer{},
					}},
					Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
						Label{Text: "Протокол:", Font: Font{Bold: true}, MinSize: Size{Width: 120}, MaxSize: Size{Width: 120}},
						Label{AssignTo: &cwLblProtocol, Text: "—"},
						HSpacer{},
					}},
					Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
						Label{Text: "Время работы:", Font: Font{Bold: true}, MinSize: Size{Width: 120}, MaxSize: Size{Width: 120}},
						Label{AssignTo: &cwLblUptime, Text: "00:00:00"},
						HSpacer{},
					}},
					Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
						Label{Text: "Передано:", Font: Font{Bold: true}, MinSize: Size{Width: 120}, MaxSize: Size{Width: 120}},
						Label{AssignTo: &cwLblSent, Text: "0 B"},
						HSpacer{},
					}},
					Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
						Label{Text: "Получено:", Font: Font{Bold: true}, MinSize: Size{Width: 120}, MaxSize: Size{Width: 120}},
						Label{AssignTo: &cwLblReceived, Text: "0 B"},
						HSpacer{},
					}},
					Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
						Label{Text: "IP-адрес:", Font: Font{Bold: true}, MinSize: Size{Width: 120}, MaxSize: Size{Width: 120}},
						Label{AssignTo: &cwLblLocalIP, Text: "—"},
						HSpacer{},
					}},
				},
			},

			VSpacer{Size: 8},

			// Protocol selector
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					Label{Text: "Протокол:", MinSize: Size{Width: 80}},
					ComboBox{
						AssignTo: &cwProtocolCB,
						Model:    []string{"wireguard", "openvpn", "ssh"},
					},
				},
			},

			VSpacer{Size: 12},

			// Connect / Disconnect button
			PushButton{
				AssignTo: &cwBtnConnect,
				Text:     "Подключиться",
				MinSize:  Size{Height: 48},
				Font:     Font{PointSize: 11, Bold: true},
				OnClicked: func() {
					if currentState == "connected" || currentState == "connecting" {
						go doDisconnect()
					} else {
						protocol := cwProtocolCB.Text()
						go cwDoConnect(protocol)
					}
				},
			},

			VSpacer{},

			// Status bar
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 4},
				Children: []Widget{
					ImageView{
						AssignTo: &cwDotImage,
						MinSize:  Size{Width: 12, Height: 12},
						MaxSize:  Size{Width: 12, Height: 12},
					},
					Label{AssignTo: &cwLblStatus, Text: "Отключён"},
					HSpacer{},
					Label{Text: appVersion, Font: Font{PointSize: 8}},
				},
			},
		},
	}).Create(); err != nil {
		logger.Error("Failed to create connection window: %v", err)
		return
	}

	connWindowMu.Lock()
	connWindow = mw
	connWindowMu.Unlock()

	// Set protocol from config
	for i, p := range []string{"wireguard", "openvpn", "ssh"} {
		if p == appCfg.Protocol {
			cwProtocolCB.SetCurrentIndex(i)
			break
		}
	}

	if windowIcon != nil {
		mw.SetIcon(windowIcon)
	}

	// Set initial status dot
	cwSetDot("disconnected")

	// Apply initial status
	if service != nil {
		cwRefreshUI(service.GetStatusPayload())
	}

	// Start ticker for live uptime/traffic updates
	cwStopTicker = make(chan struct{})
	go cwTickerLoop()

	mw.Run()

	// Window closed — clean up
	cwStopTickerSafe()

	connWindowMu.Lock()
	connWindow = nil
	connWindowMu.Unlock()
}

// cwDoConnect saves the selected protocol and initiates connection.
func cwDoConnect(protocol string) {
	if protocol != "" {
		cfg := loadAppConfig()
		cfg.Protocol = protocol
		saveAppConfig(cfg)

		if service != nil {
			service.ReloadConfig()
		}
	}
	doConnect()
}

func cwStopTickerSafe() {
	if cwStopTicker != nil {
		close(cwStopTicker)
		cwStopTicker = nil
	}
}

func cwTickerLoop() {
	defer logger.Recover("cwTickerLoop")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cwStopTicker:
			return
		case <-ticker.C:
			connWindowMu.Lock()
			win := connWindow
			connWindowMu.Unlock()
			if win == nil {
				return
			}
			if service != nil {
				status := service.GetStatusPayload()
				win.Synchronize(func() {
					cwRefreshUI(status)
				})
			}
		}
	}
}
