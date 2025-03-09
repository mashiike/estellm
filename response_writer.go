package estellm

import (
	"fmt"
	"slices"
	"strings"
)

//go:generate go tool enumer -type=FinishReason -json -trimprefix=FinishReason  -transform=snake -output=finish_reason.gen.go
type FinishReason uint32

const (
	FinishReasonEndTurn FinishReason = iota
	FinishReasonMaxTokens
	FinishReasonStopSequence
	FinishReasonGuardrailIntervened
	FinishReasonContentFiltered
)

type ResponseWriter interface {
	Metadata() Metadata
	WritePart(parts ...ContentPart) error
	Finish(reason FinishReason, msg string) error
}

type responseWriterToWriter struct {
	w ResponseWriter
}

func (w *responseWriterToWriter) Write(p []byte) (n int, err error) {
	err = w.w.WritePart(TextPart(string(p)))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *responseWriterToWriter) WriteString(s string) (n int, err error) {
	err = w.w.WritePart(TextPart(s))
	if err != nil {
		return 0, err
	}
	return len(s), nil
}

func ResponseWriterToWriter(w ResponseWriter) *responseWriterToWriter {
	return &responseWriterToWriter{w: w}
}

type Response struct {
	Metadata      Metadata     `json:"metadata,omitempty"`
	Message       Message      `json:"message,omitempty"`
	FinishReason  FinishReason `json:"finish_reason,omitempty"`
	FinishMessage string       `json:"finish_message,omitempty"`
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

type BatchResponseWriter struct {
	metadata Metadata
	parts    []ContentPart
	reason   FinishReason
	message  string
}

func NewBatchResponseWriter() *BatchResponseWriter {
	return &BatchResponseWriter{
		metadata: make(Metadata),
	}
}

func (w *BatchResponseWriter) Metadata() Metadata {
	return w.metadata
}

func (w *BatchResponseWriter) WritePart(parts ...ContentPart) error {
	w.parts = append(w.parts, parts...)
	return nil
}

func (w *BatchResponseWriter) Finish(reason FinishReason, msg string) error {
	w.reason = reason
	w.message = msg
	return nil
}

func (w *BatchResponseWriter) Response() *Response {
	if len(w.parts) > 1 {
		compact := make([]ContentPart, 0, len(w.parts))
		buf := w.parts[0]
		for i := 1; i < len(w.parts); i++ {
			current := w.parts[i]
			if current.Type != PartTypeText {
				compact = append(compact, buf)
				buf = current
				continue
			}
			if buf.Type != current.Type {
				compact = append(compact, buf)
				buf = current
				continue
			}
			buf.Text += current.Text
		}
		compact = append(compact, buf)
		w.parts = compact
	}
	return &Response{
		Metadata: w.metadata,
		Message: Message{
			Role:  RoleAssistant,
			Parts: w.parts,
		},
		FinishReason:  w.reason,
		FinishMessage: w.message,
	}
}
