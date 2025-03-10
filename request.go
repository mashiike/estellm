package estellm

import (
	"encoding/json"

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
	payload := r.Payload
	if bs, err := json.Marshal(r.Payload); err == nil {
		var tmp any
		if err := json.Unmarshal(bs, &tmp); err == nil {
			payload = tmp
		}
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
