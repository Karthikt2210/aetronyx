package spec

// Spec is the root specification structure for an Aetronyx task.
type Spec struct {
	Version            string              `yaml:"spec_version"`
	Name               string              `yaml:"name"`
	Intent             string              `yaml:"intent"`
	Budget             Budget              `yaml:"budget"`
	AcceptanceCriteria []AcceptanceCriterion `yaml:"acceptance_criteria"`
	Invariants         []string            `yaml:"invariants"`
	OutOfScope         []string            `yaml:"out_of_scope"`
	Dependencies       Dependencies        `yaml:"dependencies"`
	TestContracts      []TestContract      `yaml:"test_contracts"`
	ApprovalGates      []ApprovalGate      `yaml:"approval_gates"`
	RoutingHint        RoutingHint         `yaml:"routing_hint"`
	Metadata           map[string]string   `yaml:"metadata"`
}

// Budget defines resource limits for a specification.
type Budget struct {
	MaxCostUSD        float64 `yaml:"max_cost_usd"`
	MaxIterations     int     `yaml:"max_iterations"`
	MaxWallTimeMins   int     `yaml:"max_wall_time_minutes"`
	MaxTokens         int     `yaml:"max_tokens"`
}

// AcceptanceCriterion defines a testable condition in Given-When-Then format.
type AcceptanceCriterion struct {
	Given string `yaml:"given"`
	When  string `yaml:"when"`
	Then  string `yaml:"then"`
}

// Dependencies lists files, services, and APIs the specification depends on.
type Dependencies struct {
	Files    []string `yaml:"files"`
	Services []string `yaml:"services"`
	APIs     []string `yaml:"apis"`
}

// TestContract links a test command to acceptance criteria or invariants.
type TestContract struct {
	Name    string   `yaml:"name"`
	Command string   `yaml:"command"`
	MapsTo  []string `yaml:"maps_to"`
}

// ApprovalGate defines a checkpoint for human approval during execution.
type ApprovalGate struct {
	After    string `yaml:"after"`
	Before   string `yaml:"before"`
	Required bool   `yaml:"required"`
}

// RoutingHint suggests which LLM models to use for planning and execution.
type RoutingHint struct {
	PlanningModel  string `yaml:"planning_model"`
	ExecutionModel string `yaml:"execution_model"`
}
