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
	b.WriteString("CELL\tTEMPLATE\tCREATION\tSTATUS\tPROGRESS\tPR\tFAILED_STAGE\tLAST_ERROR\n")
	for _, cell := range cells {
		status := statusReady
		if cell.Status == statusPending {
			status = statusPending
		}
		progress := cell.Progress
		if progress == "" {
			progress = "-"
		}
		pullRequests := "-"
		if len(cell.PullRequests) > 0 {
			pullRequests = "#" + strings.Join(cell.PullRequests, ",#")
		}
		failedStage := "-"
		lastError := "-"
		if cell.CreationStatus == creationFailed {
			failedStage = cell.FailedStage
			lastError = singleLine(cell.LastError, 120)
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			cell.Label, cell.Template, cell.CreationStatus, status, progress, pullRequests, failedStage, lastError)
	}
	return b.String()
}

// FormatUndecidedReviews warns about the Cells whose pull requests are partly
// merged and partly closed. A person decides whether each is merged or abandoned.
func FormatUndecidedReviews(cells []string) string {
	var b strings.Builder
	for _, cell := range cells {
		fmt.Fprintf(&b, "WARNING: cell %s has pull requests both merged and closed; run paracell merged %s or paracell abandon %s\n", cell, cell, cell)
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
