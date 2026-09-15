package domain

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrVersionConflict = errors.New("cell version conflict")
)
