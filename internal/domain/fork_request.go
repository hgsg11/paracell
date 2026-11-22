package domain

import "fmt"

// ForkRequest holds the fork arguments that paracell uses itself when a queued
// entry starts. An empty Prefix means the default prefix and an empty Note means
// no note.
type ForkRequest struct {
	Template string
	Prefix   string
	Command  string
	Note     string
}

// NewForkRequest requires a template and normalizes a non-empty note.
func NewForkRequest(template, prefix, command, note string) (ForkRequest, error) {
	if template == "" {
		return ForkRequest{}, fmt.Errorf("fork request template is required")
	}
	if note != "" {
		normalized, err := NormalizeCellNote(note)
		if err != nil {
			return ForkRequest{}, err
		}
		note = normalized
	}
	return ForkRequest{Template: template, Prefix: prefix, Command: command, Note: note}, nil
}
