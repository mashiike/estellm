package estellm

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/go-jsonnet"
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
	vm               *jsonnet.VM       `json:"-"`
	rawMap           map[string]any    `json:"-"`
	dependents       []string          `json:"-"`
}

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
	if config.PayloadSchema == nil {
		config.PayloadSchema = make(map[string]any)
	}
	config.PromptPath = promptPath
	config.Raw = raw
	config.vm = vm
	var rawMap map[string]any
	if err := config.Decode(&rawMap); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	config.rawMap = rawMap
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
	return &cloned
}

func (cfg *Config) Dependents() []string {
	return slices.Clone(cfg.dependents)
}

func (cfg *Config) Decode(v any) error {
	vm := cfg.vm
	if vm == nil {
		vm = makeVM()
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
