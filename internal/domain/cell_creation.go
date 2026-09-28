package domain

type CellCreation struct {
	Status      CreationStatus
	Stage       CreationStage
	FailedStage CreationStage
	LastError   string
}

func NewCellCreation() CellCreation {
	return CellCreation{Status: CreationCreating, Stage: CreationStageSource}
}
