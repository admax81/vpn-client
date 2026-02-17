//go:build windows

package ui

import (
	"fmt"
	"net/netip"

	"github.com/lxn/walk"

	"github.com/user/vpn-client/internal/logger"
)

// --- Local routing file operations ---

// cmSaveDefaultRoute saves the default route setting to config.
func cmSaveDefaultRoute(mw *walk.MainWindow) {
	if service == nil || cmDefaultRouteCheck == nil {
		return
	}

	// Get current config
	cfg := loadAppConfig()
	isEnabled := cmDefaultRouteCheck.Checked()
	cfg.Routing.DefaultRoute = isEnabled

	// Save config
	if err := saveAppConfig(cfg); err != nil {
		logger.Warning("Failed to save default route setting: " + err.Error())
		walk.MsgBox(mw, "Ошибка", "Не удалось сохранить настройку:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}

	logger.Info("Default route setting saved: " + fmt.Sprintf("%v", isEnabled))

	// Check if VPN is connected
	if service.GetState() == "connected" {
		walk.MsgBox(mw, "Требуется переподключение",
			"Настройка сохранена.\n\nДля применения изменений необходимо переподключиться к VPN.",
			walk.MsgBoxIconInformation)
	}
}

// cmLoadDefaultRoute loads the default route setting from config.
func cmLoadDefaultRoute(mw *walk.MainWindow) {
	if cmDefaultRouteCheck == nil {
		return
	}

	cfg := loadAppConfig()
	mw.Synchronize(func() {
		cmDefaultRouteCheck.SetChecked(cfg.Routing.DefaultRoute)
	})
}

func cmRefreshRoutes(parent *walk.MainWindow) {
	if service == nil {
		return
	}
	routes, err := service.FetchRemoteRoutes()
	if err != nil {
		logger.Warning("Failed to read local routes: " + err.Error())
		walk.MsgBox(parent, "Ошибка", "Не удалось загрузить маршруты:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	cmRoutingModel.loadFromRemote(routes)
	// Also refresh the local routes cache for journal highlighting
	refreshLocalRoutesCache()
}

func cmAddRoute(parent *walk.MainWindow) {
	if service == nil || cmRouteEntry == nil || cmRoutingReadOnly {
		return
	}
	value := cmRouteEntry.Text()
	if value == "" {
		return
	}
	routeType := "IP/CIDR"
	if cmRouteType != nil {
		routeType = cmRouteType.Text()
	}

	if err := service.AddRemoteRoute(value, routeType); err != nil {
		logger.Warning("Failed to add route: " + err.Error())
		walk.MsgBox(parent, "Ошибка", "Не удалось добавить маршрут:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}

	cmRouteEntry.SetText("")
	cmRefreshRoutes(parent)
}

func cmDeleteRoute(parent *walk.MainWindow) {
	if service == nil || cmRoutingTable == nil || cmRoutingReadOnly {
		return
	}
	idx := cmRoutingTable.CurrentIndex()
	if idx < 0 || idx >= len(cmRoutingModel.items) {
		return
	}
	item := cmRoutingModel.items[idx]

	if err := service.RemoveRemoteRoute(item.Value, item.Type); err != nil {
		logger.Warning("Failed to delete route: " + err.Error())
		walk.MsgBox(parent, "Ошибка", "Не удалось удалить маршрут:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}

	cmRefreshRoutes(parent)
}

// --- Journal context menu: dynamic route actions ---

// cmSetupJournalContextMenu sets up a dynamic context menu on the journal table.
// The menu is rebuilt on each right-click based on whether the entry is in routes.
func cmSetupJournalContextMenu(mw *walk.MainWindow) {
	if cmJournalTable == nil {
		return
	}

	menu, err := walk.NewMenu()
	if err != nil {
		logger.Error("Failed to create journal context menu: %v", err)
		return
	}
	cmJournalTable.SetContextMenu(menu)

	cmJournalTable.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.RightButton {
			cmRebuildJournalContextMenu(mw)
		}
	})
}

// cmRebuildJournalContextMenu clears and rebuilds the journal table context menu.
func cmRebuildJournalContextMenu(mw *walk.MainWindow) {
	if cmJournalTable == nil || cmJournalModel == nil {
		return
	}

	menu := cmJournalTable.ContextMenu()
	if menu == nil {
		return
	}

	// Clear existing actions
	for menu.Actions().Len() > 0 {
		menu.Actions().RemoveAt(0)
	}

	idx := cmJournalTable.CurrentIndex()
	if idx < 0 || idx >= len(cmJournalModel.items) {
		return
	}
	entry := cmJournalModel.items[idx]

	// Always add "Копировать IP"
	copyIPAction := walk.NewAction()
	copyIPAction.SetText("Копировать IP")
	copyIPAction.Triggered().Attach(func() {
		cmCopyJournalIP()
	})
	menu.Actions().Add(copyIPAction)

	// Check if entry is in routes
	matchingEntries := findMatchingRouteEntries(entry)

	if len(matchingEntries) > 0 {
		// Entry IS in routes — show "Удалить из маршрутов"
		sep := walk.NewSeparatorAction()
		menu.Actions().Add(sep)

		for _, routeEntry := range matchingEntries {
			re := routeEntry // capture
			delAction := walk.NewAction()
			delAction.SetText("Удалить из маршрутов: " + re)
			delAction.Triggered().Attach(func() {
				cmJournalRemoveFromRoutes(mw, re)
			})
			menu.Actions().Add(delAction)
		}
	} else {
		// Entry is NOT in routes — show "Добавить в маршруты:" submenu
		if entry.RemoteIP != "" {
			sep := walk.NewSeparatorAction()
			menu.Actions().Add(sep)

			labelAction := walk.NewAction()
			labelAction.SetText("Добавить в маршруты:")
			labelAction.SetEnabled(false)
			menu.Actions().Add(labelAction)

			// IP address option
			addIPAction := walk.NewAction()
			addIPAction.SetText(entry.RemoteIP)
			addIPAction.Triggered().Attach(func() {
				cmJournalAddToRoutes(mw, entry.RemoteIP)
			})
			menu.Actions().Add(addIPAction)

			// CIDR option (if available)
			if addr, err := netip.ParseAddr(entry.RemoteIP); err == nil {
				cidr := ipToCIDR(addr)
				if cidr != "" && cidr != entry.RemoteIP {
					addCIDRAction := walk.NewAction()
					addCIDRAction.SetText(cidr)
					addCIDRAction.Triggered().Attach(func() {
						cmJournalAddToRoutes(mw, cidr)
					})
					menu.Actions().Add(addCIDRAction)
				}
			}
		}
	}
}

// cmJournalAddToRoutes adds an IP or CIDR from the journal to the routes file.
func cmJournalAddToRoutes(parent *walk.MainWindow, value string) {
	if service == nil {
		return
	}
	if err := service.AddRemoteRoute(value, "IP/CIDR"); err != nil {
		logger.Warning("Failed to add route from journal: " + err.Error())
		walk.MsgBox(parent, "Ошибка", "Не удалось добавить маршрут:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	logger.Info("Route added from journal: " + value)
	cmRefreshRoutes(parent)
}

// cmJournalRemoveFromRoutes removes a route entry that matches the journal entry.
func cmJournalRemoveFromRoutes(parent *walk.MainWindow, routeEntry string) {
	if service == nil {
		return
	}
	// Determine type from entry format
	routeType := "IP/CIDR"
	if _, err := netip.ParseAddr(routeEntry); err != nil {
		if _, err := netip.ParsePrefix(routeEntry); err != nil {
			routeType = "Domain"
		}
	}

	if err := service.RemoveRemoteRoute(routeEntry, routeType); err != nil {
		logger.Warning("Failed to remove route from journal: " + err.Error())
		walk.MsgBox(parent, "Ошибка", "Не удалось удалить маршрут:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	logger.Info("Route removed from journal: " + routeEntry)
	cmRefreshRoutes(parent)
}
