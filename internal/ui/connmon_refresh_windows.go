//go:build windows

package ui

import (
	"fmt"

	"github.com/user/vpn-client/internal/logger"
)

func cmRefresh() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("cmRefresh panic: %v", r)
		}
	}()

	if connMonitor == nil {
		return
	}

	conns := connMonitor.GetConnections()
	cmConnModel.update(conns)
	if cmConnTable != nil {
		cmConnTable.Invalidate()
	}
	if cmCountLabel != nil {
		cmCountLabel.SetText(fmt.Sprintf("Соединений: %d", len(conns)))
	}

	journal := connMonitor.GetJournal()
	cmJournalModel.update(journal)
	if cmJournalTable != nil {
		cmJournalTable.Invalidate()
	}

	if cmStatusLabel != nil {
		cmStatusLabel.SetText(fmt.Sprintf("Мониторинг активен | Журнал: %d записей", len(journal)))
	}
}
