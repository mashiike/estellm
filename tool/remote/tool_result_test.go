package remote

import (
	"encoding/json"
	"testing"

	"github.com/mashiike/estellm"
	"github.com/stretchr/testify/require"
)

func TestToolResult__Marshal(t *testing.T) {
	parts := []estellm.ContentPart{
		estellm.TextPart("Hello, World!"),
		estellm.TextPart(`{"key":"value"}`),
		estellm.BinaryPartWithName("text/csv", "example", []byte("a,b,c\n1,2,3")),
		estellm.BinaryPart("image/png", []byte("image data")),
	}

	var tr ToolResult
	err := tr.UnmarshalParts(parts)
	require.NoError(t, err)
	tr.Status = "success"
	bs, err := json.MarshalIndent(tr, "", "  ")
	require.NoError(t, err)
	expected := `{
	"content": [
		{
			"type": "text",
			"text": "Hello, World!"
		},
		{
			"type": "json",
			"json": "{\"key\":\"value\"}"
		},
		{
			"type": "document",
			"format": "csv",
			"name": "example",
			"source": "YSxiLGMKMSwyLDM="
		},
		{
			"type": "image",
			"format": "png",
			"source": "aW1hZ2UgZGF0YQ=="
		}
	],
	"status": "success"
}
`
	t.Log(string(bs))
	require.JSONEq(t, expected, string(bs))
	var tr2 ToolResult
	err = json.Unmarshal(bs, &tr2)
	require.NoError(t, err)
	require.EqualValues(t, tr, tr2)
	acutal, err := tr2.MarshalParts()
	require.NoError(t, err)
	require.EqualValues(t, parts, acutal)
}
