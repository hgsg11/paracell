package domain

type CellCreation struct {
	Status      CreationStatus
	FailedStage CreationStage
	LastError   string
}

func NewCellCreation() CellCreation {
	return CellCreation{Status: CreationReady}
}
