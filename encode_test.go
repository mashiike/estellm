package estellm_test

import (
	"bytes"
	"testing"

	"github.com/mashiike/estellm"
	"github.com/stretchr/testify/require"
)

func TestMessageEncoder__Encode(t *testing.T) {
	messages := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is user message."),
			},
		},
		{
			Role: estellm.RoleAssistant,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is assistant message."),
			},
		},
	}

	var buf bytes.Buffer
	enc := estellm.NewMessageEncoder(&buf)
	err := enc.Encode("", messages)
	require.NoError(t, err)

	expected := `<role:user/>This is user message.
<role:assistant/>This is assistant message.
`
	require.Equal(t, expected, buf.String())
}

func TestMessageEncoder__EncodeWithSystem(t *testing.T) {
	messages := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is user message."),
			},
		},
		{
			Role: estellm.RoleAssistant,
			Parts: []estellm.ContentPart{
				estellm.TextPart("This is assistant message."),
			},
		},
	}

	var buf bytes.Buffer
	enc := estellm.NewMessageEncoder(&buf)
	err := enc.Encode("system message", messages)
	require.NoError(t, err)

	expected := `system message
<role:user/>This is user message.
<role:assistant/>This is assistant message.
`
	require.Equal(t, expected, buf.String())
}

func TestMessageEncoder__EncodeBinary(t *testing.T) {
	messages := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				estellm.TextPart("explain this binary data."),
				estellm.BinaryPart("text/plain", []byte("Hello, World!")),
				estellm.TextPart("what is this?"),
			},
		},
	}

	var buf bytes.Buffer
	enc := estellm.NewMessageEncoder(&buf)
	err := enc.Encode("", messages)
	require.NoError(t, err)

	expected := `<role:user/>explain this binary data.
<binary src="data:text/plain;base64,SGVsbG8sIFdvcmxkIQ=="/>
what is this?
`
	require.Equal(t, expected, buf.String())
}

func TestMessageEncoder_TextOnly(t *testing.T) {
	var buf bytes.Buffer
	encoder := estellm.NewMessageEncoder(&buf)
	encoder.TextOnly()

	messages := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				{Type: estellm.PartTypeText, Text: "Hello"},
				{Type: estellm.PartTypeBinary, MIMEType: "image/png", Data: []byte{0x89, 0x50, 0x4E, 0x47}},
			},
		},
	}

	err := encoder.Encode("", messages)
	require.NoError(t, err)

	expected := "<role:user/>Hello\n"
	require.Equal(t, expected, buf.String())
}

func TestMessageEncoder_SkipReasoning(t *testing.T) {
	var buf bytes.Buffer
	encoder := estellm.NewMessageEncoder(&buf)
	encoder.SkipReasoning()

	messages := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				{Type: estellm.PartTypeText, Text: "Hello"},
				{Type: estellm.PartTypeReasoning, Text: "Thinking..."},
			},
		},
	}

	err := encoder.Encode("", messages)
	require.NoError(t, err)

	expected := "<role:user/>Hello\n"
	require.Equal(t, expected, buf.String())
}

func TestMessageEncoder_NoRole(t *testing.T) {
	var buf bytes.Buffer
	encoder := estellm.NewMessageEncoder(&buf)
	encoder.NoRole()

	messages := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				{Type: estellm.PartTypeText, Text: "Hello"},
			},
		},
	}

	err := encoder.Encode("", messages)
	require.NoError(t, err)

	expected := "Hello\n"
	require.Equal(t, expected, buf.String())
}

func TestMessageEncoder__EncodeReasoning(t *testing.T) {
	messages := []estellm.Message{
		{
			Role: estellm.RoleUser,
			Parts: []estellm.ContentPart{
				{Type: estellm.PartTypeText, Text: "What do you think?"},
				{Type: estellm.PartTypeReasoning, Text: "I think this is a good idea."},
				{Type: estellm.PartTypeReasoning, Text: " yes, I think so."},
			},
		},
	}

	var buf bytes.Buffer
	enc := estellm.NewMessageEncoder(&buf)
	err := enc.Encode("", messages)
	require.NoError(t, err)
	require.NoError(t, enc.Flush())

	expected := `<role:user/>What do you think?
<think>I think this is a good idea. yes, I think so.</think>`
	require.Equal(t, expected, buf.String())
}
