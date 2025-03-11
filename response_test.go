package estellm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponseString(t *testing.T) {
	resp := &Response{
		Message: Message{
			Role: RoleAssistant,
			Parts: []ContentPart{
				TextPart("part1"),
				TextPart("part2"),
			},
		},
	}

	expected := "part1part2\n"
	assert.Equal(t, expected, resp.String())
}

func TestResponseTemplateData(t *testing.T) {
	resp := &Response{
		Message: Message{
			Parts: []ContentPart{
				TextPart("part1"),
				TextPart(`{"key": "value"}`),
			},
		},
	}

	data := resp.TemplateData()
	assert.Equal(t, `part1{"key": "value"}`+"\n", data.String())
	assert.Equal(t, "value", data["key"])
}

func TestResponseTemplateDataString(t *testing.T) {
	data := responseTemplateData{
		"_raw": "raw response",
	}

	assert.Equal(t, "raw response", data.String())
}
