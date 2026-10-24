package domain

// AgentStatusHook sets a Cell to Status when the agent reaches Trigger.
type AgentStatusHook struct {
	Trigger AgentTrigger
	Status  CellStatus
}

// AgentStatusHooks returns the hooks Paracell installs into an agent:
// the Cell is pending while the agent works and ready once it has answered.
func AgentStatusHooks() []AgentStatusHook {
	return []AgentStatusHook{
		{Trigger: TurnStarted, Status: Pending},
		{Trigger: TurnFinished, Status: Ready},
	}
}
