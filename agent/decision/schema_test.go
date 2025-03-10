package decision

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtructFirstJSON(t *testing.T) {
	cases := []struct {
		name     string
		bs       []byte
		expected map[string]any
		hasError bool
	}{
		{
			name: "normal",
			bs:   []byte(`{"key": "value"}`),
			expected: map[string]any{
				"key": "value",
			},
			hasError: false,
		},
		{
			name:     "invalid json",
			bs:       []byte(`{"key": "value"`),
			hasError: true,
		},
		{
			name: "before non json text",
			bs:   []byte(`<hoge>{"key": "value"}`),
			expected: map[string]any{
				"key": "value",
			},
			hasError: false,
		},
		{
			name: "after non json text",
			bs:   []byte(`{"key": "value"}<hoge>`),
			expected: map[string]any{
				"key": "value",
			},
			hasError: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v map[string]any
			err := extructFirstJSON(tc.bs, &v)
			if tc.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, v)
			}
		})
	}
}
