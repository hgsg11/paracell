package domain

import "fmt"

type CreationStage string

const (
	CreationStageSource     CreationStage = "source"
	CreationStageContainers CreationStage = "containers"
	CreationStageSession    CreationStage = "session"
)

func NewCreationStage(value string) (CreationStage, error) {
	stage := CreationStage(value)
	switch stage {
	case CreationStageSource, CreationStageContainers, CreationStageSession:
		return stage, nil
	default:
		return stage, fmt.Errorf("invalid creation stage %q", stage)
	}
}
