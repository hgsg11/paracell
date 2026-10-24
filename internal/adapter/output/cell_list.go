package output

import (
	"fmt"
	"strings"

	"github.com/hgsg11/paracell/internal/query"
)

const (
	statusPending  = "pending"
	statusReady    = "ready"
	creationFailed = "failed"
)

func FormatCellList(cells []query.CellView) string {
	var b strings.Builder
	b.WriteString("CELL\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n")
	for _, cell := range cells {
		status := statusReady
		if cell.Status == statusPending {
			status = statusPending
		}
		failedStage := "-"
		lastError := "-"
		if cell.CreationStatus == creationFailed {
			failedStage = cell.FailedStage
			lastError = singleLine(cell.LastError, 120)
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%t\t%s\t%s\n",
			cell.Label, cell.Template, cell.CreationStatus, status, cell.Done, failedStage, lastError)
	}
	return b.String()
}

func singleLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "-"
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}
