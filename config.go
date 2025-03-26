package estellm

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/mashiike/estellm/jsonutil"
	"github.com/mashiike/estellm/metadata"
)

type Config struct {
	Raw              string            `json:"-"`
	PromptPath       string            `json:"-"`
	Enabled          *bool             `json:"enabled"`
	Default          bool              `json:"default"`
	Description      string            `json:"description"`
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	DependsOn        []string          `json:"depends_on"`
	PayloadSchema    map[string]any    `json:"payload_schema,omitempty"`
	Tools            []string          `json:"tools,omitempty"`
	RequestMetadata  metadata.Metadata `json:"request_metadata,omitempty"`
	ResponseMetadata metadata.Metadata `json:"response_metadata,omitempty"`
	AsReasoning      bool              `json:"as_reasoning,omitempty"`
	Publish          bool              `json:"publish,omitempty"`
	PublishTypes     []string          `json:"publish_types,omitempty"`
	Arguments        []ConfigArgument  `json:"arguments,omitempty"`
	vm               *jsonnet.VM       `json:"-"`
	rawMap           map[string]any    `json:"-"`
	dependents       []string          `json:"-"`
}

type ConfigArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

const (
	PublishTypeTool   = "tool"
	PublishTypePrompt = "prompt"
)

func newConfig(vm *jsonnet.VM, raw, promptPath string) (*Config, error) {
	jsonStr, err := vm.EvaluateAnonymousSnippet(promptPath, raw)
	if err != nil {
		return nil, fmt.Errorf("evaluate config: %w", err)
	}
	var config Config
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if config.Name == "" {
		config.Name = strings.TrimSuffix(filepath.Base(promptPath), filepath.Ext(promptPath))
	}
	if config.Type == "" {
		return nil, fmt.Errorf("prompt `%s`: type is empty", config.Name)
	}
	if config.Enabled == nil {
		config.Enabled = ptr(true)
	}
	if config.Arguments != nil && config.PayloadSchema != nil {
		return nil, fmt.Errorf("prompt `%s`: arguments and payload_schema are mutually exclusive", config.Name)
	}
	if config.Arguments != nil {
		required := make([]any, 0, len(config.Arguments))
		properties := make(map[string]any, len(config.Arguments))
		for i, arg := range config.Arguments {
			if arg.Name == "" {
				return nil, fmt.Errorf("prompt `%s`: argument[%d]: name is empty", config.Name, i)
			}
			propertiesSchema := map[string]any{
				"type": "string",
			}
			if arg.Description != "" {
				propertiesSchema["description"] = arg.Description
			}
			properties[arg.Name] = propertiesSchema
			if arg.Required {
				required = append(required, arg.Name)
			}
		}
		config.PayloadSchema = map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   required,
		}
		slog.Debug("generate payload schema", "prompt", config.Name, "schema", config.PayloadSchema)
	}
	if config.PayloadSchema == nil {
		config.PayloadSchema = make(map[string]any)
	}
	if config.PublishTypes == nil {
		config.PublishTypes = []string{PublishTypeTool}
	}
	slices.Sort(config.PublishTypes)
	config.PublishTypes = slices.Compact(config.PublishTypes)
	for i, publishType := range config.PublishTypes {
		config.PublishTypes[i] = strings.ToLower(publishType)
		if slices.Contains([]string{PublishTypeTool, PublishTypePrompt}, publishType) {
			continue
		}
		if publishType == PublishTypePrompt && config.Arguments == nil {
			return nil, fmt.Errorf("prompt `%s`: publish type `%s` requires arguments", config.Name, publishType)
		}
		return nil, fmt.Errorf("prompt `%s`: invalid publish type `%s`", config.Name, publishType)
	}
	config.PromptPath = promptPath
	config.Raw = raw
	config.vm = vm
	var rawMap map[string]any
	if err := config.Decode(&rawMap); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	config.rawMap = rawMap
	slog.Debug("parsed config", "config", config)
	return &config, nil
}

func (cfg *Config) AppendDependents(dependents ...string) {
	cfg.dependents = append(cfg.dependents, dependents...)
	slices.Sort(cfg.dependents)
	cfg.dependents = slices.Compact(cfg.dependents)
	if cfg.rawMap != nil {
		cfg.rawMap["dependents"] = cfg.dependents
	}
}

func (cfg *Config) AppendDependsOn(dependsOn ...string) {
	cfg.DependsOn = append(cfg.DependsOn, dependsOn...)
	slices.Sort(cfg.DependsOn)
	cfg.DependsOn = slices.Compact(cfg.DependsOn)
}

func (cfg *Config) Clone() *Config {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	cloned.rawMap = maps.Clone(cfg.rawMap)
	cloned.vm = cfg.vm
	cloned.Enabled = ptr(*cfg.Enabled)
	cloned.dependents = slices.Clone(cfg.dependents)
	cloned.Tools = slices.Clone(cfg.Tools)
	cloned.RequestMetadata = cfg.RequestMetadata.Clone()
	cloned.ResponseMetadata = cfg.ResponseMetadata.Clone()
	if cloned.Arguments != nil {
		cloned.Arguments = slices.Clone(cfg.Arguments)
	}
	cloned.PayloadSchema = maps.Clone(cfg.PayloadSchema)
	cloned.PublishTypes = slices.Clone(cfg.PublishTypes)
	return &cloned
}

func (cfg *Config) Dependents() []string {
	return slices.Clone(cfg.dependents)
}

func (cfg *Config) Decode(v any) error {
	vm := cfg.vm
	if vm == nil {
		vm = jsonutil.MakeVM()
	}
	jsonStr, err := vm.EvaluateAnonymousSnippet(cfg.PromptPath+".jsonnet", cfg.Raw)
	if err != nil {
		return fmt.Errorf("evaluate jsonnet: %w", err)
	}
	if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	return nil
}

func (cfg *Config) RawAsMap() map[string]any {
	return maps.Clone(cfg.rawMap)
}
