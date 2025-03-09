package estellm

import (
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/mashiike/estellm/metadata"
	"github.com/stretchr/testify/require"
)

func TestConfigClone(t *testing.T) {
	vm := jsonnet.MakeVM()
	original := &Config{
		Raw:              "raw data",
		PromptPath:       "path/to/prompt",
		Name:             "test_name",
		Type:             "test_type",
		DependsOn:        []string{"dep1", "dep2"},
		PayloadSchema:    map[string]any{"key": "value"},
		rawMap:           map[string]any{"raw_key": "raw_value"},
		vm:               vm,
		Enabled:          ptr(true),
		dependents:       []string{"dep3", "dep4"},
		Tools:            []string{"tool1", "tool2"},
		RequestMetadata:  metadata.Metadata{"req_key": "req_value"},
		ResponseMetadata: metadata.Metadata{"res_key": "res_value"},
	}

	clone := original.Clone()

	require.EqualValues(t, original, clone)
}
