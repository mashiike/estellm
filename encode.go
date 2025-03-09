package estellm

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

type MessageEncoder struct {
	w             io.Writer
	inReasoning   bool
	skipReasoning bool
}

func NewMessageEncoder(w io.Writer) *MessageEncoder {
	return &MessageEncoder{
		w: w,
	}
}

func (e *MessageEncoder) SkipReasoning() {
	e.skipReasoning = true
}

func (e *MessageEncoder) Encode(system string, messages []Message) error {
	if str := strings.TrimSpace(system); str != "" {
		fmt.Fprintln(e.w, str)
	}
	for _, msg := range messages {
		if err := e.EncodeMessage(msg); err != nil {
			return err
		}
	}
	return nil
}

func (e *MessageEncoder) EncodeMessage(msg Message) error {
	if msg.Role != RoleUser && msg.Role != RoleAssistant {
		return fmt.Errorf("unsupported role: %s", msg.Role)
	}
	fmt.Fprintf(e.w, "<role:%s/>", msg.Role)
	for _, part := range msg.Parts {
		if err := e.EncodeContentPart(part); err != nil {
			return err
		}
		fmt.Fprintln(e.w)
	}
	return nil
}

func (e *MessageEncoder) EncodeContentPart(part ContentPart) error {
	switch part.Type {
	case PartTypeText:
		if e.inReasoning {
			fmt.Fprint(e.w, "</think>\n")
			e.inReasoning = false
		}
		fmt.Fprint(e.w, part.Text)
	case PartTypeBinary:
		if e.inReasoning {
			fmt.Fprint(e.w, "</think>\n")
			e.inReasoning = false
		}
		dataURL := fmt.Sprintf("data:%s;base64,%s", part.MIMEType, base64.StdEncoding.EncodeToString(part.Data))
		fmt.Fprintf(e.w, "<binary src=\"%s\"/>", dataURL)
	case PartTypeReasoning:
		if e.skipReasoning {
			return nil
		}
		if !e.inReasoning {
			fmt.Fprint(e.w, "<think>")
			e.inReasoning = true
		}
		fmt.Fprint(e.w, part.Text)
		return nil
	default:
		return fmt.Errorf("unsupported content type: %s", part.Type)
	}
	return nil
}
