//go:build windows

package ui

import (
	"fmt"
	"net/netip"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/user/vpn-client/internal/connmon"
	"github.com/user/vpn-client/internal/logger"
)

// showJournalEntryDetails displays a dialog with complete information about a journal entry.
func showJournalEntryDetails(parent walk.Form, entry connmon.JournalEntry) {
	var dlg *walk.Dialog
	var acceptPB *walk.PushButton

	// Parse CIDR if available
	var cidr string
	var addr netip.Addr
	if entry.RemoteIP != "" {
		if a, err := netip.ParseAddr(entry.RemoteIP); err == nil {
			addr = a
			cidr = ipToCIDR(addr)
		}
	}

	// Check if entry is in routes
	matchingRoutes := findMatchingRouteEntries(entry)
	isInRoutes := len(matchingRoutes) > 0

	// Format timestamp
	timestampStr := entry.Timestamp.Format("02.01.2006 15:04:05")

	// Format local address
	localAddrStr := "—"
	if entry.LocalAddr.IsValid() {
		localAddrStr = entry.LocalAddr.String()
	}

	// Format remote port
	remotePortStr := "—"
	if entry.RemoteAddr.Port() > 0 {
		remotePortStr = fmt.Sprintf("%d", entry.RemoteAddr.Port())
	}

	// Format domain/CIDR display
	domainCIDRStr := "—"
	if cidr != "" && entry.Domain != "" {
		domainCIDRStr = fmt.Sprintf("%s\n(%s)", cidr, entry.Domain)
	} else if cidr != "" {
		domainCIDRStr = cidr
	} else if entry.Domain != "" {
		domainCIDRStr = entry.Domain
	}

	// Process name
	processNameStr := "—"
	if entry.ProcessName != "" {
		processNameStr = entry.ProcessName
	}

	// Build route status text
	routeStatusStr := "❌ Не в маршрутах"
	if isInRoutes {
		routeStatusStr = "✅ В маршрутах: " + matchingRoutes[0]
		if len(matchingRoutes) > 1 {
			routeStatusStr += fmt.Sprintf(" (+%d)", len(matchingRoutes)-1)
		}
	}

	// Define widgets for buttons so we can enable/disable them
	var copyIPBtn *walk.PushButton
	var addIPBtn *walk.PushButton
	var addCIDRBtn *walk.PushButton
	var removeRouteBtn *walk.PushButton

	if err := (Dialog{
		AssignTo:      &dlg,
		Title:         "Подробности подключения",
		DefaultButton: &acceptPB,
		MinSize:       Size{Width: 480, Height: 420},
		MaxSize:       Size{Width: 600, Height: 600},
		Layout:        VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
		Children: []Widget{
			// Information section
			GroupBox{
				Title:  "Информация",
				Layout: Grid{Columns: 2, Spacing: 6},
				Children: []Widget{
					Label{Text: "Время:", Font: Font{Bold: true}},
					Label{Text: timestampStr},

					Label{Text: "Протокол:", Font: Font{Bold: true}},
					Label{Text: entry.Protocol},

					Label{Text: "Удалённый IP:", Font: Font{Bold: true}},
					Label{Text: entry.RemoteIP},

					Label{Text: "Порт:", Font: Font{Bold: true}},
					Label{Text: remotePortStr},

					Label{Text: "CIDR/Домен:", Font: Font{Bold: true}},
					Label{Text: domainCIDRStr},

					Label{Text: "Локальный адрес:", Font: Font{Bold: true}},
					Label{Text: localAddrStr},

					Label{Text: "Приложение:", Font: Font{Bold: true}},
					Label{Text: processNameStr},

					Label{Text: "Причина:", Font: Font{Bold: true}},
					Label{Text: entry.Reason},

					Label{Text: "Статус маршрута:", Font: Font{Bold: true}},
					Label{Text: routeStatusStr},
				},
			},

			// Actions section
			GroupBox{
				Title:  "Действия",
				Layout: VBox{Spacing: 4},
				Children: []Widget{
					PushButton{
						AssignTo: &copyIPBtn,
						Text:     "Копировать IP",
						OnClicked: func() {
							if entry.RemoteIP != "" {
								walk.Clipboard().SetText(entry.RemoteIP)
								logger.Info("IP copied to clipboard: " + entry.RemoteIP)
								walk.MsgBox(dlg, "Скопировано", "IP адрес скопирован в буфер обмена", walk.MsgBoxIconInformation)
							}
						},
					},
					PushButton{
						AssignTo: &addIPBtn,
						Text:     fmt.Sprintf("Добавить IP в маршруты (%s)", entry.RemoteIP),
						Enabled:  !isInRoutes && entry.RemoteIP != "",
						OnClicked: func() {
							if service == nil {
								walk.MsgBox(dlg, "Ошибка", "Сервис недоступен", walk.MsgBoxIconError)
								return
							}
							if err := service.AddRemoteRoute(entry.RemoteIP, "IP/CIDR"); err != nil {
								logger.Warning("Failed to add IP route: " + err.Error())
								walk.MsgBox(dlg, "Ошибка", "Не удалось добавить маршрут:\n"+err.Error(), walk.MsgBoxIconError)
								return
							}
							logger.Info("Route added: " + entry.RemoteIP)
							walk.MsgBox(dlg, "Успешно", "IP добавлен в маршруты", walk.MsgBoxIconInformation)
							// Refresh routes
							if connMonWindow != nil {
								connMonWindow.Synchronize(func() {
									cmRefreshRoutes(connMonWindow)
								})
							}
							dlg.Close(walk.DlgCmdOK)
						},
					},
					PushButton{
						AssignTo: &addCIDRBtn,
						Text:     fmt.Sprintf("Добавить CIDR в маршруты (%s)", cidr),
						Enabled:  !isInRoutes && cidr != "" && cidr != entry.RemoteIP,
						OnClicked: func() {
							if service == nil {
								walk.MsgBox(dlg, "Ошибка", "Сервис недоступен", walk.MsgBoxIconError)
								return
							}
							if err := service.AddRemoteRoute(cidr, "IP/CIDR"); err != nil {
								logger.Warning("Failed to add CIDR route: " + err.Error())
								walk.MsgBox(dlg, "Ошибка", "Не удалось добавить маршрут:\n"+err.Error(), walk.MsgBoxIconError)
								return
							}
							logger.Info("Route added: " + cidr)
							walk.MsgBox(dlg, "Успешно", "CIDR добавлен в маршруты", walk.MsgBoxIconInformation)
							// Refresh routes
							if connMonWindow != nil {
								connMonWindow.Synchronize(func() {
									cmRefreshRoutes(connMonWindow)
								})
							}
							dlg.Close(walk.DlgCmdOK)
						},
					},
					PushButton{
						AssignTo: &removeRouteBtn,
						Text:     "Удалить из маршрутов",
						Enabled:  isInRoutes,
						OnClicked: func() {
							if service == nil {
								walk.MsgBox(dlg, "Ошибка", "Сервис недоступен", walk.MsgBoxIconError)
								return
							}
							if len(matchingRoutes) == 0 {
								walk.MsgBox(dlg, "Ошибка", "Запись не найдена в маршрутах", walk.MsgBoxIconWarning)
								return
							}
							// Remove first matching route
							routeEntry := matchingRoutes[0]
							routeType := "IP/CIDR"
							if _, err := netip.ParseAddr(routeEntry); err != nil {
								if _, err := netip.ParsePrefix(routeEntry); err != nil {
									routeType = "Domain"
								}
							}
							if err := service.RemoveRemoteRoute(routeEntry, routeType); err != nil {
								logger.Warning("Failed to remove route: " + err.Error())
								walk.MsgBox(dlg, "Ошибка", "Не удалось удалить маршрут:\n"+err.Error(), walk.MsgBoxIconError)
								return
							}
							logger.Info("Route removed: " + routeEntry)
							walk.MsgBox(dlg, "Успешно", "Маршрут удалён", walk.MsgBoxIconInformation)
							// Refresh routes
							if connMonWindow != nil {
								connMonWindow.Synchronize(func() {
									cmRefreshRoutes(connMonWindow)
								})
							}
							dlg.Close(walk.DlgCmdOK)
						},
					},
				},
			},

			// Spacer and Close button
			VSpacer{},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{
						AssignTo: &acceptPB,
						Text:     "Закрыть",
						OnClicked: func() {
							dlg.Close(walk.DlgCmdOK)
						},
					},
				},
			},
		},
	}.Create(parent)); err != nil {
		logger.Error("Failed to create journal details dialog: %v", err)
		walk.MsgBox(parent, "Ошибка", "Не удалось создать окно деталей", walk.MsgBoxIconError)
		return
	}

	dlg.Run()
}
