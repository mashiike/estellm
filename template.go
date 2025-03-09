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
		reference["result"] = resp
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
		reference["result"] = dummyResp
	}
	return reference
}

var builtinTemplateFuncs = template.FuncMap{
	"toXml":           toXml,
	"toXmlWithPrefix": toXmlWithPrefix,
	"ref": func(name string) (map[string]any, error) {
		return newReference(nil, nil), nil
	},
	"self": func() (map[string]any, error) {
		return newReference(nil, nil), nil
	},
}

// ConfigLoadPhaseTemplateFuncs returns the template functions for the config load phase.
func ConfigLoadPhaseTemplateFuncs() template.FuncMap {
	ret := sprig.TxtFuncMap()
	maps.Copy(ret, builtinTemplateFuncs)
	return ret
}

func PreRenderPhaseTemplateFuncs(cfg *Config) template.FuncMap {
	ret := ConfigLoadPhaseTemplateFuncs()
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
	ret := PreRenderPhaseTemplateFuncs(p.Config())
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
