//go:build windows

package ui

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/user/vpn-client/internal/connmon"
	"github.com/user/vpn-client/internal/logger"
	"github.com/user/vpn-client/internal/routing"
)

var (
	connMonWindow   *walk.MainWindow
	connMonWindowMu sync.Mutex
	connMonitor     *connmon.Monitor

	cmConnTable    *walk.TableView
	cmConnModel    *connMonConnModel
	cmJournalTable *walk.TableView
	cmJournalModel *connMonJournalModel
	cmStopTicker   chan struct{}
	cmStatusLabel  *walk.Label
	cmCountLabel   *walk.Label

	// Routing tab
	cmRoutingTable      *walk.TableView
	cmRoutingModel      *cmRoutingTableModel
	cmRouteEntry        *walk.LineEdit
	cmRouteType         *walk.ComboBox
	cmAddRouteBtn       *walk.PushButton
	cmDeleteRouteBtn    *walk.PushButton
	cmDefaultRouteCheck *walk.CheckBox
	cmRoutingReadOnly   bool
)

// ShowConnMonWindow displays the connection monitor window.
func ShowConnMonWindow() {
	connMonWindowMu.Lock()
	if connMonWindow != nil {
		win := connMonWindow
		connMonWindowMu.Unlock()
		win.Synchronize(func() {
			win.Show()
		})
		return
	}
	connMonWindowMu.Unlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	windowIcon := createWindowIcon()

	cmConnModel = &connMonConnModel{}
	cmJournalModel = &connMonJournalModel{}
	cmRoutingModel = &cmRoutingTableModel{}

	var mw *walk.MainWindow

	if err := (MainWindow{
		AssignTo: &mw,
		Title:    "Монитор соединений — Обнаружение маршрутов",
		MinSize:  Size{Width: 750, Height: 450},
		Size:     Size{Width: 880, Height: 550},
		Layout:   VBox{MarginsZero: true},
		Children: []Widget{
			TabWidget{
				Pages: []TabPage{
					// Tab 1: Remote routing management
					{
						Title:  "Маршрутизация",
						Layout: VBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}},
						Children: []Widget{
							CheckBox{
								AssignTo: &cmDefaultRouteCheck,
								Text:     "Весь трафик через VPN (маршрут по умолчанию)",
								OnCheckStateChanged: func() {
									cmSaveDefaultRoute(mw)
								},
							},
							VSeparator{},
							Label{Text: "Маршруты (локальный файл routes.txt):", Font: Font{Bold: true}},
							Composite{
								Layout: HBox{MarginsZero: true, Spacing: 4},
								Children: []Widget{
									ComboBox{
										AssignTo: &cmRouteType,
										Model:    []string{"IP/CIDR", "Domain"},
										MaxSize:  Size{Width: 90, Height: 0},
									},
									LineEdit{
										AssignTo:    &cmRouteEntry,
										ToolTipText: "напр., 10.0.0.0/8 или internal.company.com",
									},
									PushButton{
										AssignTo: &cmAddRouteBtn,
										Text:     "Добавить",
										MaxSize:  Size{Width: 80, Height: 28},
										OnClicked: func() {
											cmAddRoute(mw)
										},
									},
									PushButton{
										AssignTo: &cmDeleteRouteBtn,
										Text:     "Удалить",
										MaxSize:  Size{Width: 80, Height: 28},
										OnClicked: func() {
											cmDeleteRoute(mw)
										},
									},
									PushButton{
										Text:    "Обновить",
										MaxSize: Size{Width: 80, Height: 28},
										OnClicked: func() {
											cmRefreshRoutes(mw)
										},
									},
								},
							},
							TableView{
								AssignTo:         &cmRoutingTable,
								AlternatingRowBG: true,
								ColumnsOrderable: true,
								Model:            cmRoutingModel,
								Columns: []TableViewColumn{
									{Title: "Тип", Width: 90},
									{Title: "Значение", Width: 400},
								},
							},
						},
					},
					// Tab 2: Active connections
					{
						Title:  "Текущие соединения",
						Layout: VBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}},
						Children: []Widget{
							Composite{
								Layout: HBox{MarginsZero: true, Spacing: 8},
								Children: []Widget{
									Label{AssignTo: &cmCountLabel, Text: "Соединений: 0"},
									HSpacer{},
									PushButton{
										Text:    "Обновить",
										MaxSize: Size{Width: 80, Height: 28},
										OnClicked: func() {
											cmRefresh()
										},
									},
								},
							},
							TableView{
								AssignTo:         &cmConnTable,
								AlternatingRowBG: true,
								ColumnsOrderable: true,
								Model:            cmConnModel,
								Columns: []TableViewColumn{
									{Title: "Протокол", Width: 65},
									{Title: "Удалённый IP", Width: 130},
									{Title: "Порт", Width: 55},
									{Title: "CIDR", Width: 200},
									{Title: "Состояние", Width: 100},
									{Title: "Локальный адрес", Width: 140},
									{Title: "Приложение", Width: 120},
								},
							},
						},
					},
					// Tab 3: Connections journal
					{
						Title:  "Журнал подключений",
						Layout: VBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}},
						Children: []Widget{
							Composite{
								Layout: HBox{MarginsZero: true, Spacing: 8},
								Children: []Widget{
									Label{Text: "Журнал успешных и неудачных подключений:"},
									HSpacer{},
									PushButton{
										Text:    "Очистить",
										MaxSize: Size{Width: 80, Height: 28},
										OnClicked: func() {
											if connMonitor != nil {
												connMonitor.ClearJournal()
												cmJournalModel.update(nil)
											}
										},
									},
								},
							},
							TableView{
								AssignTo:         &cmJournalTable,
								AlternatingRowBG: true,
								ColumnsOrderable: true,
								Model:            cmJournalModel,
								StyleCell: func(style *walk.CellStyle) {
									if cmJournalModel == nil {
										return
									}
									row := style.Row()
									if row < 0 || row >= len(cmJournalModel.items) {
										return
									}
									if style.Col() == -1 {
										// Row-level pass: highlight entire row green if IP/domain is in routes.txt
										if isJournalEntryInRoutes(cmJournalModel.items[row]) {
											style.BackgroundColor = walk.RGB(200, 240, 200)
										}
									}
								},
								OnItemActivated: func() {
									// Double-click handler
									if cmJournalTable == nil || cmJournalModel == nil {
										return
									}
									idx := cmJournalTable.CurrentIndex()
									if idx < 0 || idx >= len(cmJournalModel.items) {
										return
									}
									entry := cmJournalModel.items[idx]
									showJournalEntryDetails(mw, entry)
								},
								Columns: []TableViewColumn{
									{Title: "Время", Width: 75},
									{Title: "Протокол", Width: 65},
									{Title: "Удалённый IP", Width: 130},
									{Title: "Порт", Width: 55},
									{Title: "CIDR", Width: 180},
									{Title: "Причина", Width: 100},
									{Title: "Приложение", Width: 120},
								}},
						},
					},
				},
			},
			// Status bar
			Composite{
				Layout: HBox{Margins: Margins{Left: 10, Top: 2, Right: 10, Bottom: 6}, Spacing: 8},
				Children: []Widget{
					Label{AssignTo: &cmStatusLabel, Text: "Мониторинг активен", Font: Font{PointSize: 8}},
					HSpacer{},
					Label{Text: "Обновление каждые 2 сек.", Font: Font{PointSize: 8}},
				},
			},
		},
	}).Create(); err != nil {
		logger.Error("Failed to create connmon window: %v", err)
		return
	}

	if windowIcon != nil {
		mw.SetIcon(windowIcon)
	}

	connMonWindowMu.Lock()
	connMonWindow = mw
	connMonWindowMu.Unlock()

	// Start monitor
	if connMonitor == nil {
		connMonitor = connmon.NewMonitor()
	}
	connMonitor.Start()

	// Load local routes on open
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error(fmt.Sprintf("Panic in routing tab initialization: %v", r))
			}
		}()

		// Always load the local routes cache for journal highlighting
		routes, err := routing.ReadLocalRoutesFile()
		if err != nil {
			logger.Warning("Failed to load routes on open: " + err.Error())
		} else {
			mw.Synchronize(func() {
				cmRoutingModel.loadFromRemote(routes)
				refreshLocalRoutesCache()
			})
		}

		if service == nil {
			return
		}

		// Check write permissions
		if err := service.CheckRoutingFileWritable(); err != nil {
			logger.Warning("Routing file is not writable: " + err.Error())
			cmRoutingReadOnly = true
			mw.Synchronize(func() {
				if cmAddRouteBtn != nil {
					cmAddRouteBtn.SetEnabled(false)
				}
				if cmDeleteRouteBtn != nil {
					cmDeleteRouteBtn.SetEnabled(false)
				}
				if cmRouteEntry != nil {
					cmRouteEntry.SetReadOnly(true)
				}
			})
		} else {
			cmRoutingReadOnly = false
		}
	}()

	// Set up dynamic context menu for journal table
	cmSetupJournalContextMenu(mw)

	// Load route type combo default
	if cmRouteType != nil {
		cmRouteType.SetCurrentIndex(0)
	}

	// Load default route checkbox state
	cmLoadDefaultRoute(mw)

	// Start periodic UI refresh
	cmStopTicker = make(chan struct{})
	go cmTickerLoop()

	mw.Run()

	// Window closed — clean up
	if cmStopTicker != nil {
		close(cmStopTicker)
		cmStopTicker = nil
	}
	if connMonitor != nil {
		connMonitor.Stop()
	}

	connMonWindowMu.Lock()
	connMonWindow = nil
	connMonWindowMu.Unlock()
}

func cmTickerLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cmStopTicker:
			return
		case <-ticker.C:
			connMonWindowMu.Lock()
			win := connMonWindow
			connMonWindowMu.Unlock()
			if win == nil {
				return
			}
			win.Synchronize(func() {
				cmRefresh()
			})
		}
	}
}

// cmCopyJournalIP copies the selected journal entry IP to clipboard.
func cmCopyJournalIP() {
	if cmJournalTable == nil || cmJournalModel == nil {
		return
	}
	idx := cmJournalTable.CurrentIndex()
	if idx < 0 || idx >= len(cmJournalModel.items) {
		return
	}
	text := cmJournalModel.items[idx].RemoteIP
	if text != "" {
		walk.Clipboard().SetText(text)
	}
}
