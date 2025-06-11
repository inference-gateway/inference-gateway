package adk

// AgentInfo represents agent metadata and capabilities
type AgentInfo struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	URL         string       `json:"url"`
	Version     string       `json:"version"`
	Skills      []AgentSkill `json:"skills"`
}

// AgentSkill represents a capability that an agent can perform
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputModes  []string `json:"inputModes"`
	OutputModes []string `json:"outputModes"`
}
