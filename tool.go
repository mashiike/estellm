package estellm

import (
	"context"
	"encoding/json"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Call(ctx context.Context, input any, w ResponseWriter) error
}

type ToolSet []Tool

func (ts ToolSet) Append(tools ...Tool) ToolSet {
	// unique by name, if the name is the same, overwrite
	for _, t := range tools {
		var found bool
		for i, tt := range ts {
			if tt.Name() == t.Name() {
				ts[i] = t
				found = true
				break
			}
		}
		if !found {
			ts = append(ts, t)
		}
	}
	return ts
}

func (ts ToolSet) MarshalJSON() ([]byte, error) {
	var tools []map[string]any
	for _, t := range ts {
		tools = append(tools, map[string]any{
			"name":         t.Name(),
			"description":  t.Description(),
			"input_schema": t.InputSchema(),
		})
	}
	return json.Marshal(tools)
}

func (ts *ToolSet) UnmarshalJSON(_ []byte) error {
	//ignore
	if *ts == nil {
		*ts = make(ToolSet, 0)
	}
	return nil
}

type AgentTool struct {
	name        string
	description string
	inputSchema map[string]any
	agent       Agent
}

func NewAgentTool(name, description string, inputSchema map[string]any, agent Agent) *AgentTool {
	return &AgentTool{
		name:        name,
		description: description,
		inputSchema: inputSchema,
		agent:       agent,
	}
}

func (t *AgentTool) Name() string {
	return t.name
}

func (t *AgentTool) Description() string {
	return t.description
}

func (t *AgentTool) InputSchema() map[string]any {
	return t.inputSchema
}

func (t *AgentTool) Call(ctx context.Context, input any, w ResponseWriter) error {
	req, err := NewRequest(t.name, input)
	if err != nil {
		return err
	}
	if err := t.agent.Execute(ctx, req, w); err != nil {
		return err
	}
	return nil
}
