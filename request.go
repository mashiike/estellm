package estellm

type Request struct {
	Name            string               `json:"name"`
	Payload         any                  `json:"payload"`
	Metadata        Metadata             `json:"metadata"`
	PreviousResults map[string]*Response `json:"previous_results,omitempty"`
	IncludeDeps     bool                 `json:"include_deps,omitempty"`
}

func NewRequest(name string, payload any) (*Request, error) {
	return &Request{
		Name:     name,
		Payload:  payload,
		Metadata: make(Metadata),
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
	return map[string]any{
		"name":             r.Name,
		"payload":          r.Payload,
		"metadata":         r.Metadata,
		"previous_results": r.PreviousResults,
		"include_deps":     r.IncludeDeps,
	}
}
