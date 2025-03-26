package jsonutil_test

import (
	"encoding/json"
	"math/rand/v2"
	"testing"

	"github.com/mashiike/estellm/jsonutil"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

func TestSchemaValueGenerator(t *testing.T) {
	seed := [32]byte{0}
	gen := jsonutil.NewSchemaValueGenerator(rand.New(rand.NewChaCha8(seed)))
	schema := `{
		"type": "object",
		"properties": {
			"name": { "type": "string", "example": "John Doe" },
			"age": { "type": "integer", "default": 30 },
			"gender": { "type": "string", "enum": ["male", "female", "other"] },
			"price": { "type": "number" },
			"is_active": { "type": "boolean" },
			"tags": {
				"type": "array",
				"items": { "type": "string", "example": "tag-example" }
			},
			"address": {
				"type": "object",
				"properties": {
					"city": { "type": "string", "example": "Tokyo" },
					"zip": { "type": "integer" }
				}
			}
		}
	}`

	var schemaMap map[string]interface{}
	err := json.Unmarshal([]byte(schema), &schemaMap)
	require.NoError(t, err)

	data, err := gen.Generate(schemaMap)
	require.NoError(t, err)
	g := goldie.New(
		t,
		goldie.WithFixtureDir("testdata/fixtures"),
		goldie.WithNameSuffix(".golden.json"),
	)
	g.AssertJson(t, "schema_value_generator", data)
}
