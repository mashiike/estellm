package estellm

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"text/template"

	"github.com/xeipuuv/gojsonschema"
)

type Prompt struct {
	cfg            *Config
	tmpl           *template.Template
	preRendered    string
	relatedPrompts map[string]*Prompt
}

func (p *Prompt) Name() string {
	return p.cfg.Name
}

func (p *Prompt) Config() *Config {
	return p.cfg.Clone()
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
	tmpl = tmpl.Funcs(PromptExecutionPhaseTemplateFuncs(p, req))
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

func (p *Prompt) SetRelatedPrompts(prompts map[string]*Prompt) {
	p.relatedPrompts = prompts
}
