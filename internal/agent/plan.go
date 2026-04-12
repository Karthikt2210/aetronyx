package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/spec"
)

// Step is a single unit of work in the agent's plan.
type Step struct {
	Goal          string   `json:"goal"`
	FilesTouched  []string `json:"files_touched"`
	VerifyCommand string   `json:"verify_command"`
}

// Plan holds the steps the agent will execute to satisfy the spec.
type Plan struct {
	Steps       []Step `json:"steps"`
	GeneratedAt int64  `json:"generated_at"`
	Model       string `json:"model"`
}

// reCodeFence matches a JSON code fence (```json ... ``` or ``` ... ```).
var reCodeFence = regexp.MustCompile("(?s)```(?:json)?\\s*(.+?)```")

// BuildPlanPrompt constructs the planning request sent to the planning model.
func BuildPlanPrompt(s *spec.Spec, blastSummary, agentsMD string) llm.Request {
	var sb strings.Builder
	if agentsMD != "" {
		sb.WriteString("# Agent Instructions\n")
		sb.WriteString(agentsMD)
		sb.WriteString("\n\n")
	}
	sb.WriteString("# Spec\n")
	fmt.Fprintf(&sb, "Name: %s\nIntent: %s\n", s.Name, s.Intent)
	if len(s.AcceptanceCriteria) > 0 {
		sb.WriteString("\nAcceptance criteria:\n")
		for i, ac := range s.AcceptanceCriteria {
			fmt.Fprintf(&sb, "  [%d] Given %s / When %s / Then %s\n", i, ac.Given, ac.When, ac.Then)
		}
	}
	if blastSummary != "" {
		sb.WriteString("\n# Blast Radius Summary\n")
		sb.WriteString(blastSummary)
	}
	sb.WriteString("\n\n# Task\n")
	sb.WriteString("Respond with a JSON array of steps. Each step: " +
		`{"goal":"...","files_touched":["..."],"verify_command":"..."}. ` +
		"Return only the JSON array, optionally wrapped in a ```json code fence.")

	return llm.Request{
		Model:     "",
		System:    "You are a software planning agent. Produce a concise, ordered plan.",
		Messages:  []llm.Message{{Role: "user", Content: []llm.ContentBlock{{Type: "text", Text: sb.String()}}}},
		MaxTokens: 2048,
	}
}

// ParsePlanResponse extracts a Plan from the model's text response.
// Returns a default single-step plan if the response is not valid JSON.
func ParsePlanResponse(content string) (*Plan, error) {
	raw := strings.TrimSpace(content)

	// Try extracting from code fence first.
	if m := reCodeFence.FindStringSubmatch(raw); len(m) == 2 {
		raw = strings.TrimSpace(m[1])
	}

	// Locate a JSON array.
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var steps []Step
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return nil, fmt.Errorf("ParsePlanResponse: %w", err)
	}

	return &Plan{
		Steps:       steps,
		GeneratedAt: time.Now().UTC().UnixMilli(),
	}, nil
}

// defaultPlan returns a single-step plan derived from the spec intent.
func defaultPlan(intent string) *Plan {
	return &Plan{
		Steps:       []Step{{Goal: intent}},
		GeneratedAt: time.Now().UTC().UnixMilli(),
	}
}
