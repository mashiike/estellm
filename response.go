package estellm

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mashiike/estellm/interanal/jsonutil"
	"github.com/mashiike/estellm/metadata"
)

type Response struct {
	Metadata      metadata.Metadata `json:"metadata,omitempty"`
	Message       Message           `json:"message,omitempty"`
	FinishReason  FinishReason      `json:"finish_reason,omitempty"`
	FinishMessage string            `json:"finish_message,omitempty"`

	tmpl responseTemplateData
}

func (r *Response) Clone() *Response {
	clone := *r
	clone.Metadata = r.Metadata.Clone()
	clone.Message.Parts = slices.Clone(r.Message.Parts)
	return &clone
}

func (r *Response) String() string {
	if r == nil {
		return "[no response]"
	}
	var sb strings.Builder
	enc := NewMessageEncoder(&sb)
	for _, part := range r.Message.Parts {
		if err := enc.EncodeContentPart(part); err != nil {
			return "[error encoding response]"
		}
	}
	fmt.Fprintln(&sb)
	return sb.String()
}

func (r *Response) TemplateData() responseTemplateData {
	if r.tmpl == nil {
		r.tmpl = make(responseTemplateData)
	}
	str := r.String()
	if err := jsonutil.UnmarshalFirstJSON([]byte(str), &r.tmpl); err != nil {
		r.tmpl = make(responseTemplateData)
	}
	r.tmpl["_raw"] = str
	return r.tmpl
}

type responseTemplateData map[string]any

func (d responseTemplateData) String() string {
	if d == nil {
		return ""
	}
	resp, ok := d["_raw"].(string)
	if !ok {
		return ""
	}
	return resp
}
