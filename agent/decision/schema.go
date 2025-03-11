package decision

import (
	//for embedding output schema

	_ "embed"
	"encoding/json"
	"maps"

	"github.com/google/go-jsonnet"
)

//go:embed output_schema.jsonnet
var outputSchemaSnippet string
var outputSchema map[string]any

func init() {
	vm := jsonnet.MakeVM()
	jsonStr, err := vm.EvaluateAnonymousSnippet("output_schema.jsonnet", outputSchemaSnippet)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(jsonStr), &outputSchema); err != nil {
		panic(err)
	}
}

func newOutputSchema(agents []string) map[string]any {
	schema := make(map[string]any)
	maps.Copy(schema, outputSchema)
	properties, ok := schema["properties"].(map[string]any)
	if !ok || properties == nil {
		return schema
	}
	nextAgent, ok := properties["next_agent"].(map[string]any)
	if !ok || nextAgent == nil {
		return schema
	}
	nextAgent["enum"] = agents
	properties["next_agent"] = nextAgent
	schema["properties"] = properties
	return schema
}
