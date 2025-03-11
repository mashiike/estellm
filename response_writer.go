package estellm

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/mashiike/estellm/metadata"
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
	Metadata() metadata.Metadata
	WriteRole(role string) error
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

func ResponseWriterToWriter(w ResponseWriter) io.Writer {
	return &responseWriterToWriter{w: w}
}

type BatchResponseWriter struct {
	metadata metadata.Metadata
	parts    []ContentPart
	reason   FinishReason
	message  string
}

func NewBatchResponseWriter() *BatchResponseWriter {
	return &BatchResponseWriter{
		metadata: make(metadata.Metadata),
	}
}

func (w *BatchResponseWriter) Metadata() metadata.Metadata {
	return w.metadata
}

func (w *BatchResponseWriter) WriteRole(role string) error {
	switch role {
	case RoleAssistant, RoleUser:
		w.message = role
		return nil
	default:
		return fmt.Errorf("invalid role: %s", role)
	}
}

func (w *BatchResponseWriter) WritePart(parts ...ContentPart) error {
	if len(parts) == 0 {
		return nil
	}
	if len(w.parts) == 0 {
		w.parts = make([]ContentPart, 0, len(parts))
		w.parts = append(w.parts, parts[0])
		parts = parts[1:]
	}
	for _, part := range parts {
		if part.Type != PartTypeText && part.Type != PartTypeReasoning {
			w.parts = append(w.parts, part)
			continue
		}
		if w.parts[len(w.parts)-1].Type == part.Type {
			w.parts[len(w.parts)-1].Text += part.Text
			continue
		}
		w.parts = append(w.parts, part)
	}
	return nil
}

func (w *BatchResponseWriter) Finish(reason FinishReason, msg string) error {
	w.reason = reason
	w.message = msg
	slog.Debug("call finish", "reason", reason, "msg", msg, "metadata", w.metadata)
	return nil
}

func (w *BatchResponseWriter) Response() *Response {
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

type TextStreamingResponseWriter struct {
	w        io.Writer
	metadata metadata.Metadata
	enc      *MessageEncoder
}

func NewTextStreamingResponseWriter(w io.Writer) *TextStreamingResponseWriter {
	return &TextStreamingResponseWriter{
		w:        w,
		metadata: make(metadata.Metadata),
		enc:      NewMessageEncoder(w),
	}
}

func (w *TextStreamingResponseWriter) SkipReasoning() {
	w.enc.SkipReasoning()
}

func (w *TextStreamingResponseWriter) SetBinaryOutputDir(dir string) {
	w.enc.SetBinaryOutputDir(dir)
}

func (w *TextStreamingResponseWriter) Metadata() metadata.Metadata {
	return w.metadata
}

func (w *TextStreamingResponseWriter) WriteRole(_ string) error {
	// nothing to do
	return nil
}

func (w *TextStreamingResponseWriter) WritePart(parts ...ContentPart) error {
	for _, part := range parts {
		if err := w.enc.EncodeContentPart(part); err != nil {
			return err
		}
	}
	return nil
}

func (w *TextStreamingResponseWriter) Finish(reason FinishReason, msg string) error {
	w.metadata.SetString("Finish-Reason", reason.String())
	if msg != "" {
		w.metadata.SetString("Finish-Message", msg)
	}
	return nil
}

func (w *TextStreamingResponseWriter) DumpMetadata() {
	fmt.Fprintln(w.w)
	fmt.Fprintln(w.w, w.metadata)
}

const (
	metadataKeyNextAgents = "Next-Agents"
)

func SetNextAgents(w ResponseWriter, agents ...string) {
	w.Metadata().SetStrings(metadataKeyNextAgents, agents)
}

type ReasoningMirrorResponseWriter struct {
	ResponseWriter
	mirrors []ResponseWriter
}

func NewReasoningMirrorResponseWriter(w ResponseWriter, mirrors ...ResponseWriter) *ReasoningMirrorResponseWriter {
	return &ReasoningMirrorResponseWriter{
		ResponseWriter: w,
		mirrors:        mirrors,
	}
}

func (w *ReasoningMirrorResponseWriter) WritePart(parts ...ContentPart) error {
	if err := w.ResponseWriter.WritePart(parts...); err != nil {
		return err
	}
	mirrorParts := make([]ContentPart, 0, len(parts))
	for _, part := range parts {
		if part.Type == PartTypeReasoning {
			mirrorParts = append(mirrorParts, part)
		}
	}
	if len(mirrorParts) == 0 {
		return nil
	}
	for _, mirror := range w.mirrors {
		if err := mirror.WritePart(mirrorParts...); err != nil {
			return err
		}
	}
	return nil
}

func (w *ReasoningMirrorResponseWriter) Finish(reason FinishReason, msg string) error {
	if err := w.ResponseWriter.Finish(reason, msg); err != nil {
		return err
	}
	for k, v := range w.Metadata() {
		for _, mirror := range w.mirrors {
			mirror.Metadata()[k] = v
		}
	}
	return nil
}

type AsReasoningResponseWriter struct {
	ResponseWriter
}

func NewAsReasoningResponseWriter(w ResponseWriter) *AsReasoningResponseWriter {
	return &AsReasoningResponseWriter{
		ResponseWriter: w,
	}
}

func (w *AsReasoningResponseWriter) WritePart(parts ...ContentPart) error {
	rewrite := make([]ContentPart, 0, len(parts))
	for _, part := range parts {
		if part.Type == PartTypeText {
			part.Type = PartTypeReasoning
		}
		rewrite = append(rewrite, part)
	}
	return w.ResponseWriter.WritePart(rewrite...)
}
