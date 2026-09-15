package domain

import "time"

type CellCreation struct {
	Status           CreationStatus
	Command          string
	CompletedStages  []CreationStage
	FailedStage      CreationStage
	LastError        string
	AttemptID        string
	LeaseStartedAt   *time.Time
	LeaseHeartbeatAt *time.Time
}

func NewCellCreation() CellCreation {
	return CellCreation{Status: CreationReady}
}
