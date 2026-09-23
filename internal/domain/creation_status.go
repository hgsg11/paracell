package domain

import "fmt"

type CreationStatus string

const (
	CreationCreating CreationStatus = "creating"
	CreationFailed   CreationStatus = "failed"
	CreationReady    CreationStatus = "ready"
)

func NewCreationStatus(value string) (CreationStatus, error) {
	status := CreationStatus(value)
	switch status {
	case CreationCreating, CreationFailed, CreationReady:
		return status, nil
	default:
		return status, fmt.Errorf("invalid creation status %q", status)
	}
}
