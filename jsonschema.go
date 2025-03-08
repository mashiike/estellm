package estellm

import (
	"math/rand/v2"
	"slices"
)

type ValueGenerator interface {
	Generate(schema map[string]interface{}) (any, error)
}

type SchemaValueGenerator struct {
	r *rand.Rand
}

func NewSchemaValueGenerator(r *rand.Rand) *SchemaValueGenerator {
	return &SchemaValueGenerator{
		r: r,
	}
}

var defaultSchemaValueGenerator = NewSchemaValueGenerator(nil)

func (g *SchemaValueGenerator) intN(n int) int {
	if g.r == nil {
		return rand.IntN(n)
	}
	return g.r.IntN(n)
}

func (g *SchemaValueGenerator) float64() float64 {
	if g.r == nil {
		return rand.Float64()
	}
	return g.r.Float64()
}

func (g *SchemaValueGenerator) Generate(schema map[string]interface{}) (any, error) {
	if schemaType, ok := schema["type"].(string); ok {
		if enumValues, exists := schema["enum"].([]interface{}); exists && len(enumValues) > 0 {
			return enumValues[g.intN(len(enumValues))], nil
		}
		if example, exists := schema["example"]; exists {
			return example, nil
		}
		if defaultValue, exists := schema["default"]; exists {
			return defaultValue, nil
		}
		switch schemaType {
		case "string":
			return "example_string", nil
		case "number":
			return g.float64() * 100, nil
		case "integer":
			return g.intN(100), nil
		case "boolean":
			return g.intN(2) == 1, nil
		case "array":
			if items, exists := schema["items"].(map[string]interface{}); exists {
				arr := make([]interface{}, 3)
				for i := range arr {
					val, err := g.Generate(items)
					if err != nil {
						return nil, err
					}
					arr[i] = val
				}
				return arr, nil
			}
			return []interface{}{}, nil
		case "object":
			data := make(map[string]interface{})
			if properties, ok := schema["properties"].(map[string]interface{}); ok {
				keys := make([]string, 0, len(properties))
				for key := range properties {
					keys = append(keys, key)
				}
				slices.Sort(keys)
				for _, key := range keys {
					prop := properties[key]
					propMap, ok := prop.(map[string]interface{})
					if !ok {
						continue
					}
					if enumValues, exists := propMap["enum"].([]interface{}); exists && len(enumValues) > 0 {
						data[key] = enumValues[g.intN(len(enumValues))]
						continue
					}
					if example, exists := propMap["example"]; exists {
						data[key] = example
						continue
					}
					if defaultValue, exists := propMap["default"]; exists {
						data[key] = defaultValue
						continue
					}
					val, err := g.Generate(propMap)
					if err != nil {
						return nil, err
					}
					data[key] = val
				}
			}
			return data, nil
		default:
			return nil, nil
		}
	}
	return nil, nil
}
