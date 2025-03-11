package remote

import (
	"encoding/json"
	"fmt"

	"github.com/mashiike/estellm"
)

type ToolResult struct {
	Content []ToolResultContent `json:"content,omitempty"`
	Status  string              `json:"status,omitempty"`
}

func (tr ToolResult) MarshalParts() ([]estellm.ContentPart, error) {
	content := make([]estellm.ContentPart, 0, len(tr.Content))
	for _, c := range tr.Content {
		tc, err := c.MarshalPart()
		if err != nil {
			return nil, err
		}
		content = append(content, tc)
	}
	return content, nil
}

func (tr *ToolResult) UnmarshalParts(parts []estellm.ContentPart) error {
	tr.Content = make([]ToolResultContent, 0, len(parts))
	for _, part := range parts {
		var tc ToolResultContent
		if err := tc.UnmarshalPart(part); err != nil {
			return err
		}
		tr.Content = append(tr.Content, tc)
	}
	return nil
}

type ToolResultContent struct {
	Type   string `json:"type"`
	Text   string `json:"text,omitempty"`
	Format string `json:"format,omitempty"`
	JSON   string `json:"json,omitempty"`
	Name   string `json:"name,omitempty"`
	Source []byte `json:"source,omitempty"`
}

func (trc ToolResultContent) MarshalPart() (estellm.ContentPart, error) {
	switch trc.Type {
	case "text":
		return estellm.TextPart(trc.Text), nil
	case "json":
		return estellm.TextPart(trc.JSON), nil
	case "document":
		mimeType := trc.Format
		switch mimeType {
		case "pdf":
			mimeType = "application/" + mimeType
		case "csv", "html":
			mimeType = "text/" + mimeType
		case "doc", "docx":
			mimeType = "application/msword"
		case "xls", "xlsx":
			mimeType = "application/vnd.ms-excel"
		case "txt":
			mimeType = "text/plain"
		case "md":
			mimeType = "text/markdown"
		}
		part := estellm.BinaryPart(mimeType, trc.Source)
		part.Name = trc.Name
		return part, nil
	case "image":
		return estellm.BinaryPart("image/"+trc.Format, trc.Source), nil
	default:
		return estellm.ContentPart{}, fmt.Errorf("unsupported content type: %s", trc.Type)
	}
}

func (trc *ToolResultContent) UnmarshalPart(part estellm.ContentPart) error {
	switch part.Type {
	case estellm.PartTypeText:
		if json.Valid([]byte(part.Text)) {
			trc.Type = "json"
			trc.JSON = part.Text
		} else {
			trc.Type = "text"
			trc.Text = part.Text
		}
	case estellm.PartTypeBinary:
		if part.Name != "" {
			trc.Name = part.Name
		}
		switch {
		case part.MIMEType == "application/pdf":
			trc.Type = "document"
			trc.Format = "pdf"
		case part.MIMEType == "text/csv":
			trc.Type = "document"
			trc.Format = "csv"
		case part.MIMEType == "text/html":
			trc.Type = "document"
			trc.Format = "html"
		case part.MIMEType == "application/msword":
			trc.Type = "document"
			trc.Format = "doc"
		case part.MIMEType == "application/vnd.ms-excel":
			trc.Type = "document"
			trc.Format = "xls"
		case part.MIMEType == "text/plain":
			trc.Type = "document"
			trc.Format = "txt"
		case part.MIMEType == "text/markdown":
			trc.Type = "document"
			trc.Format = "md"
		case part.MIMEType == "image/jpeg":
			trc.Type = "image"
			trc.Format = "jpeg"
		case part.MIMEType == "image/png":
			trc.Type = "image"
			trc.Format = "png"
		case part.MIMEType == "image/gif":
			trc.Type = "image"
			trc.Format = "gif"
		case part.MIMEType == "image/webp":
			trc.Type = "image"
			trc.Format = "webp"
		default:
			return fmt.Errorf("unsupported binary type: %s", part.MIMEType)
		}
		trc.Source = part.Data
	default:
		return fmt.Errorf("unsupported content type: %s", part.Type)
	}
	return nil
}
