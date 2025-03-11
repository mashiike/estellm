package estellm

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/stretchr/testify/require"
)

func TestSpecification_MarshalJSON(t *testing.T) {
	spec := Specification{
		Name:           "test-tool",
		Description:    "A tool for testing",
		InputSchema:    json.RawMessage(`{"type": "object", "properties": {"input": {"type": "string"}}}`),
		WorkerEndpoint: "http://localhost:8080/worker",
		Extra:          json.RawMessage(`{"extra_field_1": "extra_value_1", "extra_field_2": {"nested": "value"}}`),
	}

	data, err := spec.MarshalJSON()
	require.NoError(t, err)

	expectedJSON := `{
        "name": "test-tool",
        "description": "A tool for testing",
        "input_schema": {"type": "object", "properties": {"input": {"type": "string"}}},
        "worker_endpoint": "http://localhost:8080/worker",
        "extra_field_1": "extra_value_1",
        "extra_field_2": {"nested": "value"}
    }`

	var expected map[string]interface{}
	err = json.Unmarshal([]byte(expectedJSON), &expected)
	require.NoError(t, err)

	var actual map[string]interface{}
	err = json.Unmarshal(data, &actual)
	require.NoError(t, err)

	require.Equal(t, expected, actual)
}

func TestSpecification_UnmarshalJSON(t *testing.T) {
	jsonData := `{
        "name": "test-tool",
        "description": "A tool for testing",
        "input_schema": {"type": "object", "properties": {"input": {"type": "string"}}},
        "worker_endpoint": "http://localhost:8080/worker",
        "extra_field_1": "extra_value_1",
        "extra_field_2": {"nested": "value"}
    }`

	var spec Specification
	err := json.Unmarshal([]byte(jsonData), &spec)
	require.NoError(t, err)

	require.Equal(t, "test-tool", spec.Name)
	require.Equal(t, "A tool for testing", spec.Description)
	require.JSONEq(t, `{"type": "object", "properties": {"input": {"type": "string"}}}`, string(spec.InputSchema))
	require.Equal(t, "http://localhost:8080/worker", spec.WorkerEndpoint)
	require.JSONEq(t, `{"extra_field_1": "extra_value_1", "extra_field_2": {"nested": "value"}}`, string(spec.Extra))
}

func TestSpecification_EmptyExtra(t *testing.T) {
	spec := Specification{
		Name:           "test-tool",
		Description:    "A tool for testing",
		InputSchema:    json.RawMessage(`{"type": "object", "properties": {"input": {"type": "string"}}}`),
		WorkerEndpoint: "http://localhost:8080/worker",
	}

	data, err := spec.MarshalJSON()
	require.NoError(t, err)

	expectedJSON := `{
        "name": "test-tool",
        "description": "A tool for testing",
        "input_schema": {"type": "object", "properties": {"input": {"type": "string"}}},
        "worker_endpoint": "http://localhost:8080/worker"
    }`

	var expected map[string]interface{}
	err = json.Unmarshal([]byte(expectedJSON), &expected)
	require.NoError(t, err)

	var actual map[string]interface{}
	err = json.Unmarshal(data, &actual)
	require.NoError(t, err)

	require.Equal(t, expected, actual)
}

func TestSpecification_UnmarshalJSON_EmptyExtra(t *testing.T) {
	jsonData := `{
        "name": "test-tool",
        "description": "A tool for testing",
        "input_schema": {"type": "object", "properties": {"input": {"type": "string"}}},
        "worker_endpoint": "http://localhost:8080/worker"
    }`

	var spec Specification
	err := json.Unmarshal([]byte(jsonData), &spec)
	require.NoError(t, err)

	require.Equal(t, "test-tool", spec.Name)
	require.Equal(t, "A tool for testing", spec.Description)
	require.JSONEq(t, `{"type": "object", "properties": {"input": {"type": "string"}}}`, string(spec.InputSchema))
	require.Equal(t, "http://localhost:8080/worker", spec.WorkerEndpoint)
	require.Empty(t, spec.Extra)
}

func TestSpecificationCache(t *testing.T) {
	now := time.Now()
	restore := flextime.Fix(now.AddDate(0, 0, -1))
	defer restore()
	cache := NewSpecificationCache(1 * time.Hour)

	spec := Specification{Name: "test"}

	// Set the specification in the cache
	cache.Set("https://example.com/", spec)

	// Get the specification from the cache
	retrievedSpec, ok := cache.Get("https://example.com/")
	require.True(t, ok)
	require.Equal(t, spec, retrievedSpec)

	flextime.Fix(now)

	// Try to get the specification from the cache after expiration
	_, ok = cache.Get("https://example.com/")
	require.False(t, ok)
	// Set the specification in the cache again
	cache.Set("http://www.example.com/", spec)

	// Delete the specification from the cache
	cache.Delete("http://www.example.com/")

	// Try to get the specification from the cache after deletion
	_, ok = cache.Get("http://www.example.com/")
	require.False(t, ok)
}
