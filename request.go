package estellm

import (
	"github.com/mashiike/estellm/interanal/jsonutil"
	"github.com/mashiike/estellm/metadata"
)

type Request struct {
	Name              string               `json:"name"`
	Payload           any                  `json:"payload"`
	Metadata          metadata.Metadata    `json:"metadata"`
	PreviousResults   map[string]*Response `json:"previous_results,omitempty"`
	IncludeUpstream   bool                 `json:"include_upstream,omitempty"`
	IncludeDownstream bool                 `json:"include_downstream,omitempty"`
	Tools             ToolSet              `json:"tools,omitempty"`
}

func NewRequest(name string, payload any) (*Request, error) {
	return &Request{
		Name:              name,
		Payload:           payload,
		Metadata:          make(metadata.Metadata),
		IncludeUpstream:   false,
		IncludeDownstream: true,
	}, nil
}

func (r *Request) Clone() *Request {
	clone := *r
	clone.Metadata = r.Metadata.Clone()
	clone.PreviousResults = make(map[string]*Response, len(r.PreviousResults))
	for key, value := range r.PreviousResults {
		clone.PreviousResults[key] = value.Clone()
	}
	return &clone
}

func (r *Request) TemplateData() map[string]any {
	var payload any
	if err := jsonutil.Remarshal(r.Payload, &payload); err != nil {
		payload = r.Payload
	}
	return map[string]any{
		"name":               r.Name,
		"payload":            payload,
		"metadata":           r.Metadata,
		"previous_results":   r.PreviousResults,
		"include_upstream":   r.IncludeUpstream,
		"include_downstream": r.IncludeDownstream,
	}
}

func (r *Request) AddTool(t Tool, tools ...Tool) {
	r.Tools = r.Tools.Append(t)
	if len(tools) > 0 {
		r.Tools = r.Tools.Append(tools...)
	}
}
