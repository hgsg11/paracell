package domain

type CellSummary struct {
	Version        CellVersion
	ID             string
	Issue          string
	Name           string
	DisplayLabel   string
	Template       string
	CreationStatus CreationStatus
	Status         CellStatus
	Done           bool
	FailedStage    CreationStage
	LastError      string
}

func NewCellSummary(version CellVersion, id string, issue string, name string, displayLabel string, templateName string, creationStatus CreationStatus, status CellStatus, done bool, failedStage CreationStage, lastError string) CellSummary {
	return CellSummary{Version: version, ID: id, Issue: issue, Name: name, DisplayLabel: displayLabel, Template: templateName, CreationStatus: creationStatus, Status: status, Done: done, FailedStage: failedStage, LastError: lastError}
}
