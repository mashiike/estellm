package estellm

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
)

type MessageDecoder struct {
	dec *xml.Decoder
}

func NewMessageDecoder(r io.Reader) *MessageDecoder {
	dec := xml.NewDecoder(r)
	return &MessageDecoder{
		dec: dec,
	}
}

type decodeState struct {
	textBuffer           *strings.Builder
	enc                  *xml.Encoder
	current              Message
	lastChangeRole       string
	lastRoleChangeOffset int64
	result               []Message
}

func newDecodeState() *decodeState {
	var textBuffer strings.Builder
	enc := xml.NewEncoder(&textBuffer)
	return &decodeState{
		textBuffer: &textBuffer,
		enc:        enc,
		current: Message{
			Parts: make([]ContentPart, 0),
		},
		result: make([]Message, 0),
	}
}

func (s *decodeState) fulashTextBuffer() {
	if s.textBuffer.Len() > 0 {
		text := strings.TrimSpace(s.textBuffer.String())
		if len(text) > 0 {
			s.current.Parts = append(s.current.Parts, ContentPart{
				Type: PartTypeText,
				Text: text,
			})
		}
		s.textBuffer.Reset()
	}
}

func (s *decodeState) changeRole(role string) {
	if s.current.Role != role && s.lastChangeRole != role {
		s.fulashTextBuffer()
		if len(s.current.Parts) > 0 {
			s.result = append(s.result, s.current)
		}
		s.lastChangeRole = role
		s.current = Message{
			Role:  role,
			Parts: make([]ContentPart, 0),
		}
	}
}

func (s *decodeState) decodeToken(t xml.Token, inputOffset int64) error {
	switch se := t.(type) {
	case xml.CharData:
		s.textBuffer.WriteString(string(se))
	case xml.StartElement:
		switch {
		case se.Name.Space == "role":
			if se.Name.Local != RoleUser && se.Name.Local != RoleAssistant {
				return fmt.Errorf("unsupported role: %s", se.Name.Local)
			}
			s.changeRole(se.Name.Local)
			s.lastRoleChangeOffset = inputOffset
		case se.Name.Local == "binary":
			var part ContentPart
			var name string
			for _, attr := range se.Attr {
				switch attr.Name.Local {
				case "src":
					var err error
					part, err = ParseSrcURL(attr.Value)
					if err != nil {
						return err
					}
				case "name":
					name = attr.Value
				}
			}
			if name != "" {
				part.Name = name
			}
			if part.Type != PartTypeBinary {
				return errors.New("invalid binary part")
			}
			s.fulashTextBuffer()
			s.current.Parts = append(s.current.Parts, part)
		default:
			s.enc.EncodeToken(t)
			s.enc.Flush()
		}
	case xml.EndElement:
		switch {
		case se.Name.Space == "role":
			if s.lastRoleChangeOffset != inputOffset {
				s.changeRole(RoleUser)
			}
		case se.Name.Local == "binary":
			// do nothing
		default:
			s.enc.EncodeToken(t)
			s.enc.Flush()
		}
	}
	return nil
}

func (s *decodeState) complete() (string, []Message, error) {
	s.fulashTextBuffer()
	if len(s.current.Parts) > 0 {
		s.result = append(s.result, s.current)
	}
	if len(s.result) == 0 {
		return "", nil, fmt.Errorf("no messages")
	}
	if len(s.result) == 1 {
		s.result[0].Role = RoleUser
		return "", s.result, nil
	}
	if s.result[1].Role == RoleAssistant {
		s.result[0].Role = RoleUser
		return "", s.result, nil
	}
	if slices.ContainsFunc(s.result[0].Parts, func(c ContentPart) bool {
		return c.Type == PartTypeBinary
	}) {
		s.result[0].Role = RoleUser
		if s.result[1].Role == RoleUser {
			// merge user messages
			s.result[0].Parts = append(s.result[0].Parts, s.result[1].Parts...)
			s.result[1] = s.result[0]
			s.result = s.result[1:]
		}
		return "", s.result, nil
	}
	onlyText := true
	var systemPromptBuffer strings.Builder
	for _, part := range s.result[0].Parts {
		if part.Type != PartTypeText {
			onlyText = false
			break
		}
		systemPromptBuffer.WriteString(part.Text)
	}
	if !onlyText {
		s.result[0].Role = RoleUser
		return "", s.result, nil
	}
	systemPrompt := strings.TrimSpace(systemPromptBuffer.String())
	if len(systemPrompt) == 0 {
		return "", s.result, nil
	}
	return systemPrompt, s.result[1:], nil
}

func (d *MessageDecoder) Decode() (string, []Message, error) {
	s := newDecodeState()
	for {
		t, err := d.dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}
		if err := s.decodeToken(t, d.dec.InputOffset()); err != nil {
			return "", nil, err
		}
	}
	return s.complete()
}

func ParseSrcURL(srcURL string) (ContentPart, error) {
	u, err := url.Parse(srcURL)
	if err != nil {
		return ContentPart{}, fmt.Errorf("parse data URL: %w", err)
	}
	switch u.Scheme {
	case "data":
		return parseDataURL(u)
	default:
		return ContentPart{}, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
}

func parseDataURL(u *url.URL) (ContentPart, error) {
	metaParts := strings.SplitN(u.Opaque, ",", 2)
	if len(metaParts) != 2 {
		return ContentPart{}, errors.New("invalid data-url: missing comma separator")
	}

	meta, rawData := metaParts[0], metaParts[1]
	mimeType := "text/plain"
	isBase64 := false
	if meta != "" {
		metaParts := strings.Split(meta, ";")
		mimeType = metaParts[0]
		if len(metaParts) > 1 && metaParts[len(metaParts)-1] == "base64" {
			isBase64 = true
		}
	}
	var decoded []byte
	if isBase64 {
		var err error
		decoded, err = base64.StdEncoding.DecodeString(rawData)
		if err != nil {
			return ContentPart{}, fmt.Errorf("failed to decode base64 data: %w", err)
		}
	} else {
		decoded = []byte(rawData)
	}
	return ContentPart{
		Type:     PartTypeBinary,
		MIMEType: mimeType,
		Data:     decoded,
	}, nil
}
