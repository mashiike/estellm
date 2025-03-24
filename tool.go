package estellm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"github.com/mashiike/estellm/jsonutil"
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

var (
	toolUseIDContextKey = contextKey("tool_use_id")
	toolNameContextKey  = contextKey("tool_name")
)

func WithToolUseID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, toolUseIDContextKey, id)
}

func ToolUseIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(toolUseIDContextKey).(string)
	return id, ok
}

func WithToolName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, toolNameContextKey, name)
}

func ToolNameFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(toolNameContextKey).(string)
	return id, ok
}

func GenerateInputSchema[T any]() (map[string]any, error) {
	var v T
	r := jsonschema.Reflector{
		DoNotReference: true,
		ExpandedStruct: true,
	}
	schema := r.Reflect(v)
	bs, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bs, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w (schema=%q)", err, string(bs))
	}
	delete(m, "$schema")
	delete(m, "$id")
	return m, nil
}

type GenericTool[T any] struct {
	name        string
	description string
	inputSchema map[string]any
	caller      func(context.Context, T, ResponseWriter) error
}

func NewTool[T any](name, desc string, f func(context.Context, T, ResponseWriter) error) (*GenericTool[T], error) {
	inputSchema, err := GenerateInputSchema[T]()
	if err != nil {
		return nil, err
	}
	return &GenericTool[T]{
		name:        name,
		description: desc,
		inputSchema: inputSchema,
		caller:      f,
	}, nil
}

func (t *GenericTool[T]) Name() string {
	return t.name
}

func (t *GenericTool[T]) Description() string {
	return t.description
}

func (t *GenericTool[T]) InputSchema() map[string]any {
	return t.inputSchema
}

func (t *GenericTool[T]) Call(ctx context.Context, input any, w ResponseWriter) error {
	var value T
	if err := jsonutil.Remarshal(input, &value); err != nil {
		return err
	}
	return t.caller(ctx, value, w)
}
