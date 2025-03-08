package estellm

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"text/template"

	"github.com/google/go-jsonnet"
	"github.com/xeipuuv/gojsonschema"
)

type Prompt struct {
	cfg         *Config
	tmpl        *template.Template
	preRendered string
}

type Config struct {
	Raw           string         `json:"-"`
	PromptPath    string         `json:"-"`
	Enabled       *bool          `json:"enabled"`
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	DependsOn     []string       `json:"depends_on"`
	PayloadSchema map[string]any `json:"payload_schema,omitempty"`
	vm            *jsonnet.VM    `json:"-"`
}

func (p *Prompt) Name() string {
	return p.cfg.Name
}

func (p *Prompt) Config() *Config {
	return &Config{
		Raw:           p.cfg.Raw,
		vm:            p.cfg.vm,
		PromptPath:    p.cfg.PromptPath,
		Name:          p.cfg.Name,
		Type:          p.cfg.Type,
		DependsOn:     slices.Clone(p.cfg.DependsOn),
		PayloadSchema: maps.Clone(p.cfg.PayloadSchema),
	}
}

func (cfg *Config) AppendDependsOn(dependsOn ...string) {
	cfg.DependsOn = append(cfg.DependsOn, dependsOn...)
	slices.Sort(cfg.DependsOn)
	cfg.DependsOn = slices.Compact(cfg.DependsOn)
}

func (cfg *Config) Decode(v any) error {
	vm := cfg.vm
	if vm == nil {
		vm = jsonnet.MakeVM()
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

func (p *Prompt) PreRendered() string {
	return p.preRendered
}

type DataValidateError struct {
	Result *gojsonschema.Result
}

func (e *DataValidateError) Error() string {
	return fmt.Sprintf("data validation error: %d issues", len(e.Result.Errors()))
}

func (p *Prompt) Render(ctx context.Context, req *Request) (string, error) {
	return p.RenderBlock(ctx, path.Base(p.cfg.PromptPath), req)
}

var (
	ErrTemplateBlockNotFound = fmt.Errorf("template block not found")
)

func (p *Prompt) Blocks() []string {
	var blocks []string
	for _, t := range p.tmpl.Templates() {
		if path.Base(p.cfg.PromptPath) != t.ParseName {
			continue
		}
		name := t.Name()
		if slices.Contains([]string{"config", t.ParseName, ""}, name) {
			continue
		}
		blocks = append(blocks, name)
	}
	return blocks
}

func (p *Prompt) RenderBlock(ctx context.Context, blockName string, req *Request) (string, error) {
	sl := gojsonschema.NewGoLoader(p.cfg.PayloadSchema)
	dl := gojsonschema.NewGoLoader(req.Payload)
	result, err := gojsonschema.Validate(sl, dl)
	if err != nil {
		return "", fmt.Errorf("validate data: %w", err)
	}
	if !result.Valid() {
		return "", &DataValidateError{Result: result}
	}
	tmpl := p.tmpl.Lookup(blockName)
	if tmpl == nil {
		return "", fmt.Errorf("block name `%s`: %w", blockName, ErrTemplateBlockNotFound)
	}
	tmpl = tmpl.Funcs(PromptExecutionPhaseTemplateFuncs(p.cfg, req))
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, blockName, req.TemplateData()); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func (p *Prompt) Decode(ctx context.Context, req *Request) (string, []Message, error) {
	prompt, err := p.Render(ctx, req)
	if err != nil {
		return "", nil, fmt.Errorf("render prompt: %w", err)
	}
	dec := NewMessageDecoder(strings.NewReader(prompt))
	return dec.Decode()
}

func (p *Prompt) DecodeBlock(ctx context.Context, blockName string, req *Request) (string, []Message, error) {
	prompt, err := p.RenderBlock(ctx, blockName, req)
	if err != nil {
		return "", nil, fmt.Errorf("render block: %w", err)
	}
	dec := NewMessageDecoder(strings.NewReader(prompt))
	return dec.Decode()
}
