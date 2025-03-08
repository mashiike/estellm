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

func (d *MessageDecoder) Decode() (string, []Message, error) {
	result := make([]Message, 0)
	current := Message{
		Parts: make([]ContentPart, 0),
	}
	var textBuffer strings.Builder
	enc := xml.NewEncoder(&textBuffer)
	fulashTextBuffer := func() {
		if textBuffer.Len() > 0 {
			text := strings.TrimSpace(textBuffer.String())
			if len(text) > 0 {
				current.Parts = append(current.Parts, ContentPart{
					Type: PartTypeText,
					Text: text,
				})
			}
			textBuffer.Reset()
		}
	}
	var lastChangeRole string
	changeRole := func(role string) {
		if current.Role != role && lastChangeRole != role {
			fulashTextBuffer()
			if len(current.Parts) > 0 {
				result = append(result, current)
			}
			lastChangeRole = role
			current = Message{
				Role:  role,
				Parts: make([]ContentPart, 0),
			}
		}
	}
	var lastRoleChangeOffset int64
	for {
		t, err := d.dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}
		switch se := t.(type) {
		case xml.CharData:
			textBuffer.WriteString(string(se))
		case xml.StartElement:
			switch {
			case se.Name.Space == "role":
				if se.Name.Local != RoleUser && se.Name.Local != RoleAssistant {
					return "", nil, fmt.Errorf("unsupported role: %s", se.Name.Local)
				}
				changeRole(se.Name.Local)
				lastRoleChangeOffset = d.dec.InputOffset()
			case se.Name.Local == "binary":
				var part ContentPart
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "src":
						var err error
						part, err = ParseSrcURL(attr.Value)
						if err != nil {
							return "", nil, err
						}
					}
				}
				if part.Type != PartTypeBinary {
					return "", nil, errors.New("invalid binary part")
				}
				fulashTextBuffer()
				current.Parts = append(current.Parts, part)
			default:
				enc.EncodeToken(t)
				enc.Flush()
			}
		case xml.EndElement:
			switch {
			case se.Name.Space == "role":
				if lastRoleChangeOffset != d.dec.InputOffset() {
					changeRole(RoleUser)
				}
			case se.Name.Local == "binary":
				// do nothing
			default:
				enc.EncodeToken(t)
				enc.Flush()
			}
		}
	}
	fulashTextBuffer()
	if len(current.Parts) > 0 {
		result = append(result, current)
	}
	if len(result) == 0 {
		return "", nil, fmt.Errorf("no messages")
	}
	if len(result) == 1 {
		result[0].Role = RoleUser
		return "", result, nil
	}
	if result[1].Role == RoleAssistant {
		result[0].Role = RoleUser
		return "", result, nil
	}
	if slices.ContainsFunc(result[0].Parts, func(c ContentPart) bool {
		return c.Type == PartTypeBinary
	}) {
		result[0].Role = RoleUser
		if result[1].Role == RoleUser {
			// merge user messages
			result[0].Parts = append(result[0].Parts, result[1].Parts...)
			result[1] = result[0]
			result = result[1:]
		}
		return "", result, nil
	}
	onlyText := true
	var systemPromptBuffer strings.Builder
	for _, part := range result[0].Parts {
		if part.Type != PartTypeText {
			onlyText = false
			break
		}
		systemPromptBuffer.WriteString(part.Text)
	}
	if !onlyText {
		result[0].Role = RoleUser
		return "", result, nil
	}
	systemPrompt := strings.TrimSpace(systemPromptBuffer.String())
	if len(systemPrompt) == 0 {
		return "", result, nil
	}
	return systemPrompt, result[1:], nil
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
