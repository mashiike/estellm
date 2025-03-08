package estellm

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

type Message struct {
	Role  string        `json:"role"`
	Parts []ContentPart `json:"parts"`
}

const (
	PartTypeText   = "text"
	PartTypeBinary = "binary"
)

type ContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Data     []byte `json:"data,omitempty"`
}

func TextPart(text string) ContentPart {
	return ContentPart{Type: PartTypeText, Text: text}
}

func BinaryPart(mimeType string, data []byte) ContentPart {
	return ContentPart{Type: PartTypeBinary, MIMEType: mimeType, Data: data}
}
