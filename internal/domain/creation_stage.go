package domain

import "fmt"

type CreationStage string

const (
	CreationStageSource     CreationStage = "source"
	CreationStageContainers CreationStage = "containers"
	CreationStageWorkspace  CreationStage = "workspace"
)

func NewCreationStage(value string) (CreationStage, error) {
	stage := CreationStage(value)
	switch stage {
	case CreationStageSource, CreationStageContainers, CreationStageWorkspace:
		return stage, nil
	default:
		return stage, fmt.Errorf("invalid creation stage %q", stage)
	}
}
