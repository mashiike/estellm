package estellm

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type MessageEncoder struct {
	w             io.Writer
	e             *xml.Encoder
	inReasoning   bool
	skipReasoning bool
	noRole        bool
	textOnly      bool
}

func NewMessageEncoder(w io.Writer) *MessageEncoder {
	return &MessageEncoder{
		w: w,
		e: xml.NewEncoder(w),
	}
}

func (e *MessageEncoder) SkipReasoning() {
	e.skipReasoning = true
}

func (e *MessageEncoder) NoRole() {
	e.noRole = true
}

func (e *MessageEncoder) TextOnly() {
	e.textOnly = true
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
	if err := e.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}

func (e *MessageEncoder) EncodeMessage(msg Message) error {
	if msg.Role != RoleUser && msg.Role != RoleAssistant {
		return fmt.Errorf("unsupported role: %s", msg.Role)
	}
	if !e.noRole {
		fmt.Fprintf(e.w, "<role:%s/>", msg.Role)
	}
	for _, part := range msg.Parts {
		if err := e.EncodeContentPart(part); err != nil {
			return err
		}
	}
	if err := e.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}

func (e *MessageEncoder) EncodeContentPart(part ContentPart) error {
	switch part.Type {
	case PartTypeText:
		if err := e.Flush(); err != nil {
			return fmt.Errorf("flush on text part: %w", err)
		}
		fmt.Fprint(e.w, part.Text)
		fmt.Fprintln(e.w)
	case PartTypeBinary:
		if err := e.Flush(); err != nil {
			return fmt.Errorf("flush on binary part: %w", err)
		}
		if !e.textOnly {
			dataURL := fmt.Sprintf("data:%s;base64,%s", part.MIMEType, base64.StdEncoding.EncodeToString(part.Data))
			fmt.Fprintf(e.w, "<binary src=\"%s\"/>", dataURL)
			fmt.Fprintln(e.w)
		}
	case PartTypeReasoning:
		if e.skipReasoning {
			return nil
		}
		if !e.inReasoning {
			if err := e.e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "think"}}); err != nil {
				return fmt.Errorf("encode start element: %w", err)
			}
			e.inReasoning = true
		}
		e.e.EncodeToken(xml.CharData(part.Text))
		return nil
	default:
		return fmt.Errorf("unsupported content type: %s", part.Type)
	}
	return nil
}

func (e *MessageEncoder) Flush() error {
	if e.inReasoning {
		if err := e.e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "think"}}); err != nil {
			return fmt.Errorf("encode end element: %w", err)
		}
		if err := e.e.Flush(); err != nil {
			return fmt.Errorf("flush: %w", err)
		}
		e.inReasoning = false
	}
	return nil
}
