package estellm_test

import (
	"strings"
	"testing"

	"github.com/mashiike/estellm"
	"github.com/stretchr/testify/require"
)

func TestMessageDecoder__RoleChange(t *testing.T) {
	text := `
This is user message. <note> This is note message. </note>
<role:assistant> This is assistant message. </role:assistant>
This is user second message.
<role:assistant> This is assistant second message. </role:assistant>
	`
	dec := estellm.NewMessageDecoder(strings.NewReader(text))
	system, messages, err := dec.Decode()
	require.NoError(t, err)
	excepted := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is user message. <note> This is note message. </note>"),
			},
		},
		{
			Role: estellm.RoleAssistant,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is assistant message."),
			},
		},
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is user second message."),
			},
		},
		{
			Role: estellm.RoleAssistant,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is assistant second message."),
			},
		},
	}
	require.EqualValues(t, excepted, messages)
	require.Empty(t, system)
}

func TestMessageDecoder__SelfClosingRoleChange(t *testing.T) {
	text := `
this is system message
<role:user/>This is user message. <note> This is note message. </note>
<role:assistant/> This is assistant message.
<role:user/> This is user second message.
<role:assistant/> This is assistant second message.
	`
	dec := estellm.NewMessageDecoder(strings.NewReader(text))
	system, messages, err := dec.Decode()
	require.NoError(t, err)
	excepted := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is user message. <note> This is note message. </note>"),
			},
		},
		{
			Role: estellm.RoleAssistant,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is assistant message."),
			},
		},
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is user second message."),
			},
		},
		{
			Role: estellm.RoleAssistant,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is assistant second message."),
			},
		},
	}
	require.EqualValues(t, excepted, messages)
	require.EqualValues(t, "this is system message", system)
}

func TestMessageDecoder__BinaryTag(t *testing.T) {
	text := `
explain this binary data.
<binary src="data:text/plain;base64,SGVsbG8sIFdvcmxkIQ=="/>
what is this?
	`
	dec := estellm.NewMessageDecoder(strings.NewReader(text))
	system, messages, err := dec.Decode()
	require.NoError(t, err)
	excepted := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				estellm.TextPart("explain this binary data."),
				estellm.BinaryPart("text/plain", []byte("Hello, World!")),
				estellm.TextPart("what is this?"),
			},
		},
	}
	require.EqualValues(t, excepted, messages)
	require.EqualValues(t, "", system)
}

func TestMessageDecoder__OnlyText(t *testing.T) {
	text := `this is user message`
	dec := estellm.NewMessageDecoder(strings.NewReader(text))
	system, messages, err := dec.Decode()
	require.NoError(t, err)
	excepted := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				estellm.TextPart("this is user message"),
			},
		},
	}
	require.EqualValues(t, excepted, messages)
	require.Empty(t, system)
}

func TestParseSrcURL(t *testing.T) {
	tests := []struct {
		name      string
		dataURL   string
		expectErr bool
		expected  estellm.ContentPart
	}{
		{
			name:    "valid data URL",
			dataURL: "data:text/plain;base64,SGVsbG8sIFdvcmxkIQ==",
			expected: estellm.ContentPart{
				Type:     estellm.PartTypeBinary,
				MIMEType: "text/plain",
				Data:     []byte("Hello, World!"),
			},
		},
		{
			name:      "invalid scheme",
			dataURL:   "http:text/plain;base64,SGVsbG8sIFdvcmxkIQ==",
			expectErr: true,
		},
		{
			name:      "invalid base64 data",
			dataURL:   "data:text/plain;base64,SGVsbG8sIFdvcmxkIQ",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			part, err := estellm.ParseSrcURL(tt.dataURL)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, part)
			}
		})
	}
}
