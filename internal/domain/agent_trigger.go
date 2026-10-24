package domain

import "fmt"

// AgentTrigger is a point in an agent's work, named independently of any agent.
type AgentTrigger string

const (
	// TurnStarted is the point where the agent starts working on a user's prompt.
	TurnStarted AgentTrigger = "turn-started"
	// TurnFinished is the point where the agent has finished its response.
	TurnFinished AgentTrigger = "turn-finished"
)

func NewAgentTrigger(value string) (AgentTrigger, error) {
	trigger := AgentTrigger(value)
	switch trigger {
	case TurnStarted, TurnFinished:
		return trigger, nil
	default:
		return trigger, fmt.Errorf("unsupported agent trigger %q", trigger)
	}
}
