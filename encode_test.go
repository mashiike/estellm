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
