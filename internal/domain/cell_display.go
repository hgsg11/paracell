package domain

type CellDisplay struct {
	Label          string
	Template       string
	CreationStatus CreationStatus
	Status         CellStatus
	Done           bool
	FailedStage    CreationStage
	LastError      string
}

func NewCellDisplay(label string, templateName string, creationStatus CreationStatus, status CellStatus, done bool, failedStage CreationStage, lastError string) CellDisplay {
	return CellDisplay{Label: label, Template: templateName, CreationStatus: creationStatus, Status: status, Done: done, FailedStage: failedStage, LastError: lastError}
}
