package output

import (
	"fmt"
	"strings"

	"github.com/hgsg11/paracell/internal/domain"
)

func FormatCellList(cells []domain.Cell) string {
	var b strings.Builder
	b.WriteString("NAME\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n")
	for _, cell := range cells {
		failedStage := "-"
		lastError := "-"
		if cell.CreationStatus() == domain.CreationFailed {
			failedStage = string(cell.Creation.FailedStage)
			lastError = singleLine(cell.Creation.LastError, 120)
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%t\t%s\t%s\n",
			cell.Name, cell.Template, cell.CreationStatus(), cell.Status(), cell.IsDone(), failedStage, lastError)
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
