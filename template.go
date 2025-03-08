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

var builtinTemplateFuncs = template.FuncMap{
	"toXml":           toXml,
	"toXmlWithPrefix": toXmlWithPrefix,
	"ref": func(name string) (*Response, error) {
		return nil, nil
	},
	"config": func() map[string]any {
		return make(map[string]any, 0)
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
	baseRef := ret["ref"].(func(string) (*Response, error))
	ret["ref"] = func(name string) (*Response, error) {
		cfg.AppendDependsOn(name)
		return baseRef(name)
	}
	var confMap map[string]any
	if err := cfg.Decode(&confMap); err == nil {
		ret["config"] = func() map[string]any {
			return confMap
		}
	} else {
		ret["config"] = func() map[string]any {
			return nil
		}
	}
	return ret
}

func PromptExecutionPhaseTemplateFuncs(cfg *Config, req *Request) template.FuncMap {
	ret := PreRenderPhaseTemplateFuncs(cfg)
	ret["ref"] = func(name string) (*Response, error) {
		// TODO: convert response to string
		if req == nil {
			return nil, fmt.Errorf("request is nil")
		}
		if req.PreviousResults == nil {
			return nil, nil
		}
		if res, ok := req.PreviousResults[name]; ok {
			return res, nil
		}
		return nil, nil
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
