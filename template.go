package estellm

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"maps"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

func newReference(cfg *Config, resp *Response) map[string]any {
	reference := make(map[string]any, 0)
	if cfg != nil {
		reference["config"] = cfg.RawAsMap()
	} else {
		reference["config"] = map[string]any{
			"enabled": true,
			"name":    "dummy",
			"type":    "dummy",
		}
	}
	if resp != nil {
		reference["result"] = resp.TemplateData()
	} else {
		dummyResp := &Response{
			Message: Message{
				Role: RoleAssistant,
				Parts: []ContentPart{
					{
						Type: PartTypeText,
						Text: "[this is dummy result]",
					},
				},
			},
		}
		reference["result"] = dummyResp.TemplateData()
	}
	return reference
}

var builtinTemplateFuncs = template.FuncMap{
	"toXml":           toXml,
	"toXmlWithPrefix": toXmlWithPrefix,
	"resolve": func(name string) (map[string]any, error) {
		return newReference(nil, nil), nil
	},
	"ref": func(name string) (map[string]any, error) {
		return newReference(nil, nil), nil
	},
	"self": func() (map[string]any, error) {
		return newReference(nil, nil), nil
	},
	"dependents": func() map[string]any {
		return map[string]any{}
	},
	"dependentNames": func() []string {
		return []string{}
	},
}

// ConfigLoadPhaseTemplateFuncs returns the template functions for the config load phase.
func ConfigLoadPhaseTemplateFuncs(reg *Registry) template.FuncMap {
	ret := reg.getMergedTemplateFuncs()
	maps.Copy(ret, sprig.TxtFuncMap())
	maps.Copy(ret, builtinTemplateFuncs)
	return ret
}

func PreRenderPhaseTemplateFuncs(reg *Registry, cfg *Config) template.FuncMap {
	ret := ConfigLoadPhaseTemplateFuncs(reg)
	baseRef := ret["ref"].(func(string) (map[string]any, error))
	ret["ref"] = func(name string) (map[string]any, error) {
		cfg.AppendDependsOn(name)
		return baseRef(name)
	}
	ret["self"] = func() (map[string]any, error) {
		return newReference(cfg, nil), nil
	}
	return ret
}

func PromptExecutionPhaseTemplateFuncs(p *Prompt, req *Request) template.FuncMap {
	ret := PreRenderPhaseTemplateFuncs(p.reg, p.Config())
	maps.Copy(ret, p.reg.getTemplateFuncs(p.Name()))
	ret["ref"] = func(name string) (map[string]any, error) {
		if req == nil {
			return nil, fmt.Errorf("request is nil")
		}
		var resp *Response
		if r, ok := req.PreviousResults[name]; ok {
			resp = r
		}
		if p.relatedPrompts == nil {
			return newReference(nil, resp), nil
		}
		var relatedCfg *Config
		if relatedPrompt, ok := p.relatedPrompts[name]; ok {
			relatedCfg = relatedPrompt.Config()
		}
		return newReference(relatedCfg, resp), nil
	}
	ret["resolve"] = func(name string) (map[string]any, error) {
		var resp *Response
		if r, ok := req.PreviousResults[name]; ok {
			resp = r
		}
		if p.relatedPrompts == nil {
			return newReference(nil, resp), nil
		}
		var relatedCfg *Config
		if relatedPrompt, ok := p.relatedPrompts[name]; ok {
			relatedCfg = relatedPrompt.Config()
		}
		return newReference(relatedCfg, resp), nil
	}
	ret["dependents"] = func() map[string]any {
		if p.relatedPrompts == nil {
			return map[string]any{}
		}
		cfg := p.Config()
		if cfg == nil {
			return map[string]any{}
		}
		deps := make(map[string]any, 0)
		for _, dep := range cfg.dependents {
			if relatedPrompt, ok := p.relatedPrompts[dep]; ok {
				deps[dep] = newReference(relatedPrompt.Config(), nil)
			}
		}
		return deps
	}
	ret["dependentNames"] = func() []string {
		if p.relatedPrompts == nil {
			return []string{}
		}
		cfg := p.Config()
		if cfg == nil {
			return []string{}
		}
		return cfg.dependents
	}
	return ret
}

func toXml(tag string, v any) (string, error) {
	buf := new(bytes.Buffer)
	enc := xml.NewEncoder(buf)
	enc.Indent("", "  ")
	if err := enc.EncodeElement(v, xml.StartElement{Name: xml.Name{Local: tag}}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func toXmlWithPrefix(tag string, prefix string, v any) (string, error) {
	buf := new(bytes.Buffer)
	enc := xml.NewEncoder(buf)
	enc.Indent(prefix, "  ")
	if err := enc.EncodeElement(v, xml.StartElement{Name: xml.Name{Local: tag}}); err != nil {
		return "", err
	}
	return strings.TrimPrefix(buf.String(), prefix), nil
}
