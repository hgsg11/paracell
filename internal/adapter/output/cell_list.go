package output

import (
	"fmt"
	"strings"

	"github.com/hgsg11/paracell/internal/domain"
)

func FormatCellList(cells []domain.Cell) string {
	var b strings.Builder
	b.WriteString("CELL\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n")
	for _, cell := range cells {
		display := cell.Display()
		failedStage := "-"
		lastError := "-"
		if display.CreationStatus == domain.CreationFailed {
			failedStage = string(display.FailedStage)
			lastError = singleLine(display.LastError, 120)
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%t\t%s\t%s\n",
			display.Label, display.Template, display.CreationStatus, display.Status, display.Done, failedStage, lastError)
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
