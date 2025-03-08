package estellm

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindDAGs(t *testing.T) {
	tests := []struct {
		name       string
		dependents map[string][]string
		expected   []map[string][]string
	}{
		{
			name: "Single DAG",
			dependents: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"a"},
			},
			expected: []map[string][]string{
				{
					"a": {"b"},
					"b": {"c"},
					"c": {},
					"d": {"a"},
				},
			},
		},
		{
			name: "Independent DAGs",
			dependents: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"e"},
				"e": {"f"},
				"f": {},
			},
			expected: []map[string][]string{
				{
					"a": {"b"},
					"b": {"c"},
					"c": {},
				},
				{
					"d": {"e"},
					"e": {"f"},
					"f": {},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findDAGs(tt.dependents)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestPickupDAG(t *testing.T) {
	tests := []struct {
		name       string
		dependents map[string][]string
		targetNode string
		expected   map[string][]string
		ok         bool
	}{
		{
			name: "Pickup DAG(single)",
			dependents: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"a"},
			},
			targetNode: "b",
			expected: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"a"},
			},
			ok: true,
		},
		{
			name: "Pickup DAG(multiple)",
			dependents: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"e"},
				"e": {"f"},
				"f": {},
			},
			targetNode: "e",
			expected:   map[string][]string{"d": {"e"}, "e": {"f"}, "f": {}},
			ok:         true,
		},
		{
			name: "Pickup DAG(not found)",
			dependents: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"a"},
			},
			targetNode: "x",
			expected:   nil,
			ok:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, ok := pickupDAG(tt.targetNode, tt.dependents)
			if ok != tt.ok || !reflect.DeepEqual(found, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, found)
			}
		})
	}
}

func TestExtractSubgraph(t *testing.T) {
	tests := []struct {
		name     string
		graph    map[string][]string
		start    string
		expected map[string][]string
	}{
		{
			name: "Extract Subgraph",
			graph: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"a"},
			},
			start: "a",
			expected: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSubgraph(tt.graph, tt.start)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name     string
		graph    map[string][]string
		expected [][]string
		hasError bool
	}{
		{
			name: "straight line",
			graph: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"a"},
			},
			expected: [][]string{{"d"}, {"a"}, {"b"}, {"c"}},
		},
		{
			name: "branch",
			graph: map[string][]string{
				"a": {"b", "c"},
				"b": {"d"},
				"c": {"d"},
				"d": {},
			},
			expected: [][]string{{"a"}, {"b", "c"}, {"d"}},
		},
		{
			name: "cycle",
			graph: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {"a"},
			},
			hasError: true,
		},
		{
			name: "multiple dag",
			graph: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"e"},
				"e": {"f"},
				"f": {},
			},
			expected: [][]string{
				{"a", "d"},
				{"b", "e"},
				{"c", "f"},
			},
		},
		{
			name: "multiple source, sink",
			graph: map[string][]string{
				"a": {"b"},
				"b": {"e"},
				"c": {"d"},
				"d": {"e"},
				"e": {"f", "g"},
				"f": {"h"},
				"g": {},
				"h": {},
			},
			expected: [][]string{
				{"a", "c"},
				{"b", "d"},
				{"e"},
				{"f", "g"},
				{"h"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := topologicalSort(tt.graph)
			if tt.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestFindSinkNodes(t *testing.T) {
	tests := []struct {
		name     string
		graph    map[string][]string
		expected []string
	}{
		{
			name: "Find Sink Nodes",
			graph: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"a"},
			},
			expected: []string{"c"},
		},
		{
			name: "Find Sink Nodes(Branch)",
			graph: map[string][]string{
				"a": {"b", "c"},
				"b": {"d"},
				"c": {"d"},
				"d": {},
			},
			expected: []string{"d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSinkNodes(tt.graph)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestFindSourceNodes(t *testing.T) {
	tests := []struct {
		name     string
		graph    map[string][]string
		expected []string
	}{
		{
			name: "Find Source Nodes",
			graph: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"a"},
			},
			expected: []string{"d"},
		},
		{
			name: "Find Source Nodes with Multiple Sources",
			graph: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {},
				"d": {"e"},
				"e": {"f"},
				"f": {},
			},
			expected: []string{"d", "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSourceNodes(tt.graph)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}
