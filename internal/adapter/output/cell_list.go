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
		label, templateName := cell.ListLabels()
		creationStatus := cell.CreationStatus()
		status := domain.Ready
		if cell.HasStatus(domain.Pending) {
			status = domain.Pending
		}
		done := cell.EnsureCanBeCleaned() == nil
		failedStage := "-"
		lastError := "-"
		if creationStatus == domain.CreationFailed {
			stage, message := cell.CreationFailure()
			failedStage = string(stage)
			lastError = singleLine(message, 120)
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%t\t%s\t%s\n",
			label, templateName, creationStatus, status, done, failedStage, lastError)
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
