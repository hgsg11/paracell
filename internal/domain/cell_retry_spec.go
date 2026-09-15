package domain

type CellRetrySpec struct {
	ID          string
	Issue       string
	Name        string
	Project     string
	Template    string
	Command     string
	FailedStage CreationStage
}

func NewCellRetrySpec(id string, issue string, name string, project string, templateName string, command string, failedStage CreationStage) CellRetrySpec {
	return CellRetrySpec{ID: id, Issue: issue, Name: name, Project: project, Template: templateName, Command: command, FailedStage: failedStage}
}
