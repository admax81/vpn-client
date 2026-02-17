//go:build windows

package ui

import (
	"strconv"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// ShowSettingsWindow displays the settings window (Windows — lxn/walk GUI).
func ShowSettingsWindow() {
	config := loadAppConfig()

	var mw *walk.MainWindow

	// Widget references
	var protocolCombo *walk.ComboBox
	var autostartCheck *walk.CheckBox

	// Protocol settings containers
	var wgContainer, ovpnContainer, sshContainer *walk.Composite

	// WireGuard widgets
	var wgPrivateKey, wgAddress, wgPeerPubKey, wgEndpoint, wgKeepalive *walk.LineEdit

	// OpenVPN widgets
	var ovpnConfig, ovpnUsername, ovpnPassword *walk.LineEdit

	// SSH widgets
	var sshHost, sshPort, sshUser, sshKeyPath, sshRemoteTun *walk.LineEdit

	// Other widgets
	var dnsServers, dnsDomains *walk.LineEdit
	var splitDNSCheck *walk.CheckBox
	var ifaceName, ifaceMTU, ifaceMetric *walk.LineEdit
	var killswitchCheck, allowLANCheck *walk.CheckBox

	// Logs widget
	var logsTextEdit *walk.TextEdit

	// Generate window icon (teal shield for settings)
	windowIcon := createWindowIcon()

	// Function to show/hide protocol settings
	updateProtocolVisibility := func() {
		protocol := protocolCombo.Text()
		wgContainer.SetVisible(protocol == "wireguard")
		ovpnContainer.SetVisible(protocol == "openvpn")
		sshContainer.SetVisible(protocol == "ssh")
	}

	MainWindow{
		AssignTo: &mw,
		Title:    "VPN Client — Открыть тунель",
		MinSize:  Size{Width: 550, Height: 500},
		Size:     Size{Width: 600, Height: 550},
		Layout:   VBox{MarginsZero: true},
		Children: []Widget{
			TabWidget{
				Pages: []TabPage{
					// Connection tab
					{
						Title:  "Подключение",
						Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
						Children: []Widget{
							Composite{
								Layout: Grid{Columns: 2},
								Children: []Widget{
									Label{Text: "Протокол:"},
									ComboBox{
										AssignTo: &protocolCombo,
										Model:    []string{"wireguard", "openvpn", "ssh"},
										OnCurrentIndexChanged: func() {
											updateProtocolVisibility()
										},
									},
									Label{Text: ""},
									CheckBox{
										AssignTo: &autostartCheck,
										Text:     "Авто-подключение при запуске",
									},
								},
							},
							VSeparator{},
							// WireGuard
							Composite{
								AssignTo: &wgContainer,
								Layout:   VBox{MarginsZero: true},
								Children: []Widget{
									Label{Text: "Настройки WireGuard", Font: Font{Bold: true}},
									Composite{
										Layout: Grid{Columns: 2},
										Children: []Widget{
											Label{Text: "Приватный ключ:"},
											LineEdit{AssignTo: &wgPrivateKey, PasswordMode: true},
											Label{Text: "Локальный адрес:"},
											LineEdit{AssignTo: &wgAddress, ToolTipText: "напр., 10.255.0.2/24"},
											Label{Text: "Публичный ключ пира:"},
											LineEdit{AssignTo: &wgPeerPubKey},
											Label{Text: "Эндпоинт пира:"},
											LineEdit{AssignTo: &wgEndpoint, ToolTipText: "напр., vpn.example.com:51820"},
											Label{Text: "Keepalive (сек):"},
											LineEdit{AssignTo: &wgKeepalive},
										},
									},
								},
							},
							// OpenVPN
							Composite{
								AssignTo: &ovpnContainer,
								Visible:  false,
								Layout:   VBox{MarginsZero: true},
								Children: []Widget{
									Label{Text: "OpenVPN Settings", Font: Font{Bold: true}},
									Composite{
										Layout: Grid{Columns: 2},
										Children: []Widget{
											Label{Text: "Config File (.ovpn):"},
											LineEdit{AssignTo: &ovpnConfig},
											Label{Text: "Username:"},
											LineEdit{AssignTo: &ovpnUsername, ToolTipText: "Optional, if required by config"},
											Label{Text: "Password:"},
											LineEdit{AssignTo: &ovpnPassword, PasswordMode: true, ToolTipText: "Optional"},
										},
									},
								},
							},
							// SSH
							Composite{
								AssignTo: &sshContainer,
								Visible:  false,
								Layout:   VBox{MarginsZero: true},
								Children: []Widget{
									Label{Text: "SSH Tunnel Settings", Font: Font{Bold: true}},
									Composite{
										Layout: Grid{Columns: 2},
										Children: []Widget{
											Label{Text: "Host:"},
											LineEdit{AssignTo: &sshHost},
											Label{Text: "Port:"},
											LineEdit{AssignTo: &sshPort},
											Label{Text: "Username:"},
											LineEdit{AssignTo: &sshUser},
											Label{Text: "Private Key File:"},
											LineEdit{AssignTo: &sshKeyPath},
											Label{Text: "Remote TUN Address:"},
											LineEdit{AssignTo: &sshRemoteTun, ToolTipText: "e.g., 10.255.0.1/24"},
										},
									},
								},
							},
							VSpacer{},
						},
					},
					// DNS
					{
						Title:  "DNS",
						Layout: Grid{Columns: 2, Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
						Children: []Widget{
							Label{Text: "DNS-серверы:"},
							LineEdit{AssignTo: &dnsServers, ToolTipText: "Через запятую: 10.255.0.1, 8.8.8.8"},
							Label{Text: ""},
							CheckBox{AssignTo: &splitDNSCheck, Text: "Раздельный DNS"},
							Label{Text: "Домены раздельного DNS:"},
							LineEdit{AssignTo: &dnsDomains, ToolTipText: "Через запятую: company.com, internal.local"},
						},
					},
					// Advanced
					{
						Title:  "Дополнительно",
						Layout: Grid{Columns: 2, Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
						Children: []Widget{
							Label{Text: "Имя интерфейса:"},
							LineEdit{AssignTo: &ifaceName},
							Label{Text: "MTU:"},
							LineEdit{AssignTo: &ifaceMTU},
							Label{Text: "Метрика маршрута:"},
							LineEdit{AssignTo: &ifaceMetric},
							HSpacer{},
							HSpacer{},
							Label{Text: "Kill Switch:"},
							CheckBox{AssignTo: &killswitchCheck, Text: "Блокировать трафик при отключении VPN"},
							Label{Text: ""},
							CheckBox{AssignTo: &allowLANCheck, Text: "Разрешить доступ к локальной сети"},
						},
					},
					// Logs
					{
						Title:  "Логи",
						Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
						Children: []Widget{
							Composite{
								Layout: HBox{MarginsZero: true},
								Children: []Widget{
									Label{Text: "Логи VPN-соединения"},
									HSpacer{},
									PushButton{
										Text:    "Обновить",
										MaxSize: Size{Width: 70, Height: 0},
										OnClicked: func() {
											loadLogs(logsTextEdit)
										},
									},
									PushButton{
										Text:    "Очистить",
										MaxSize: Size{Width: 60, Height: 0},
										OnClicked: func() {
											logsTextEdit.SetText("")
											clearLogFile()
										},
									},
									PushButton{
										Text:    "Открыть файл",
										MaxSize: Size{Width: 90, Height: 0},
										OnClicked: func() {
											openLogFile()
										},
									},
								},
							},
							TextEdit{
								AssignTo: &logsTextEdit,
								ReadOnly: true,
								VScroll:  true,
								HScroll:  true,
								Font:     Font{Family: "Consolas", PointSize: 9},
							},
						},
					},
				},
			},
			Composite{
				Layout: HBox{Margins: Margins{Left: 10, Top: 5, Right: 10, Bottom: 10}},
				Children: []Widget{
					HSpacer{},
					PushButton{
						Text: "Отмена",
						OnClicked: func() {
							mw.Close()
						},
					},
					PushButton{
						Text: "Сохранить",
						OnClicked: func() {
							config.Protocol = protocolCombo.Text()
							config.Autostart = autostartCheck.Checked()

							config.WireGuard.PrivateKey = wgPrivateKey.Text()
							config.WireGuard.Address = wgAddress.Text()
							config.WireGuard.Peer.PublicKey = wgPeerPubKey.Text()
							config.WireGuard.Peer.Endpoint = wgEndpoint.Text()
							if k, err := strconv.Atoi(strings.TrimSpace(wgKeepalive.Text())); err == nil {
								config.WireGuard.Peer.PersistentKeepalive = k
							} else {
								config.WireGuard.Peer.PersistentKeepalive = 0
							}

							config.OpenVPN.ConfigPath = ovpnConfig.Text()
							config.OpenVPN.AuthUser = ovpnUsername.Text()
							config.OpenVPN.AuthPass = ovpnPassword.Text()

							config.SSH.Host = strings.TrimSpace(sshHost.Text())
							if p, err := strconv.Atoi(strings.TrimSpace(sshPort.Text())); err == nil {
								config.SSH.Port = p
							} else {
								config.SSH.Port = 0
							}
							config.SSH.User = strings.TrimSpace(sshUser.Text())
							config.SSH.KeyPath = strings.TrimSpace(sshKeyPath.Text())
							config.SSH.RemoteTunAddr = strings.TrimSpace(sshRemoteTun.Text())

							// DNS
							config.DNS.Servers = nil
							if strings.TrimSpace(dnsServers.Text()) != "" {
								for _, p := range strings.Split(dnsServers.Text(), ",") {
									if s := strings.TrimSpace(p); s != "" {
										config.DNS.Servers = append(config.DNS.Servers, s)
									}
								}
							}
							config.DNS.SplitDNS = splitDNSCheck.Checked()
							config.DNS.Domains = nil
							if strings.TrimSpace(dnsDomains.Text()) != "" {
								for _, p := range strings.Split(dnsDomains.Text(), ",") {
									if s := strings.TrimSpace(p); s != "" {
										config.DNS.Domains = append(config.DNS.Domains, s)
									}
								}
							}

							config.Interface.Name = ifaceName.Text()
							if m, err := strconv.Atoi(strings.TrimSpace(ifaceMTU.Text())); err == nil {
								config.Interface.MTU = m
							} else {
								config.Interface.MTU = 0
							}
							if m, err := strconv.Atoi(strings.TrimSpace(ifaceMetric.Text())); err == nil {
								config.Interface.Metric = m
							} else {
								config.Interface.Metric = 0
							}

							// Kill Switch
							config.Killswitch.Enabled = killswitchCheck.Checked()
							config.Killswitch.AllowLAN = allowLANCheck.Checked()

							if err := saveAppConfig(config); err != nil {
								walk.MsgBox(mw, "Ошибка", "Не удалось сохранить: "+err.Error(), walk.MsgBoxIconError)
							} else {
								// Reload config in the running service
								if service != nil {
									service.ReloadConfig()
								}
								walk.MsgBox(mw, "Готово", "Конфигурация сохранена!", walk.MsgBoxIconInformation)
							}
						},
					},
				},
			},
		},
	}.Create()

	// Populate fields
	for i, p := range []string{"wireguard", "openvpn", "ssh"} {
		if p == config.Protocol {
			protocolCombo.SetCurrentIndex(i)
			break
		}
	}
	autostartCheck.SetChecked(config.Autostart)

	wgPrivateKey.SetText(config.WireGuard.PrivateKey)
	wgAddress.SetText(config.WireGuard.Address)
	wgPeerPubKey.SetText(config.WireGuard.Peer.PublicKey)
	wgEndpoint.SetText(config.WireGuard.Peer.Endpoint)
	wgKeepalive.SetText(strconv.Itoa(config.WireGuard.Peer.PersistentKeepalive))

	ovpnConfig.SetText(config.OpenVPN.ConfigPath)
	ovpnUsername.SetText(config.OpenVPN.AuthUser)
	ovpnPassword.SetText(config.OpenVPN.AuthPass)

	sshHost.SetText(config.SSH.Host)
	sshPort.SetText(strconv.Itoa(config.SSH.Port))
	sshUser.SetText(config.SSH.User)
	sshKeyPath.SetText(config.SSH.KeyPath)
	sshRemoteTun.SetText(config.SSH.RemoteTunAddr)

	dnsServers.SetText(strings.Join(config.DNS.Servers, ", "))
	splitDNSCheck.SetChecked(config.DNS.SplitDNS)
	dnsDomains.SetText(strings.Join(config.DNS.Domains, ", "))

	ifaceName.SetText(config.Interface.Name)
	ifaceMTU.SetText(strconv.Itoa(config.Interface.MTU))
	ifaceMetric.SetText(strconv.Itoa(config.Interface.Metric))

	killswitchCheck.SetChecked(config.Killswitch.Enabled)
	allowLANCheck.SetChecked(config.Killswitch.AllowLAN)

	updateProtocolVisibility()

	if windowIcon != nil {
		mw.SetIcon(windowIcon)
	}

	loadLogs(logsTextEdit)

	mw.Run()
}
