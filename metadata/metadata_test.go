package metadata

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	m := Metadata{}

	// Test SetString and GetString
	m.SetString("key1", "value1")
	require.Equal(t, "value1", m.GetString("key1"))

	// Test SetInt64 and GetInt64
	m.SetInt64("key2", 12345)
	value, ok := m.GetInt64("key2")
	require.True(t, ok)
	require.Equal(t, int64(12345), value)

	// Test SetFloat64 and GetFloat64
	m.SetFloat64("key3", 123.45)
	valueFloat, ok := m.GetFloat64("key3")
	require.True(t, ok)
	require.Equal(t, 123.45, valueFloat)

	// Test SetBool and GetBool
	m.SetBool("key4", true)
	valueBool, ok := m.GetBool("key4")
	require.True(t, ok)
	require.True(t, valueBool)

	// Test SetBytes and GetBytes
	bytesValue := []byte("test bytes")
	m.SetBytes("key5", bytesValue)
	valueBytes, ok := m.GetBytes("key5")
	require.True(t, ok)
	require.Equal(t, bytesValue, valueBytes)

	// Test GetString for different types
	require.Equal(t, "12345", m.GetString("key2"))
	require.Equal(t, "123.45", m.GetString("key3"))
	require.Equal(t, "true", m.GetString("key4"))
	require.Equal(t, base64.StdEncoding.EncodeToString(bytesValue), m.GetString("key5"))

	// Test Del and Has
	m.Del("key1")
	require.False(t, m.Has("key1"))

	// Test Keys
	expectedKeys := []string{"Key2", "Key3", "Key4", "Key5"}
	keys := m.Keys()
	for _, key := range expectedKeys {
		require.Contains(t, keys, key)
	}

	// Test Clone
	clone := m.Clone()
	require.Equal(t, len(m), len(clone))
	for key, value := range m {
		require.Equal(t, value, clone[key])
	}
}

func TestUnmarshalJSON_Success(t *testing.T) {
	jsonData := `{
		"key1": "value1",
		"key2": 12345,
		"key3": 123.45,
		"key4": true,
		"key5": "dGVzdCBieXRlcw==",
		"key6": ["value1","value2"]
	}`

	var m Metadata
	err := json.Unmarshal([]byte(jsonData), &m)
	require.NoError(t, err)

	require.Equal(t, "value1", m.GetString("key1"))
	valueInt64, ok := m.GetInt64("key2")
	require.True(t, ok)
	require.Equal(t, int64(12345), valueInt64)
	valueFloat64, ok := m.GetFloat64("key3")
	require.True(t, ok)
	require.Equal(t, 123.45, valueFloat64)
	valueBool, ok := m.GetBool("key4")
	require.True(t, ok)
	require.Equal(t, true, valueBool)
	valueBytes, ok := m.GetBytes("key5")
	require.True(t, ok)
	require.Equal(t, []byte("test bytes"), valueBytes)
	valueStrings := m.GetStrings("key6")
	require.Equal(t, []string{"value1", "value2"}, valueStrings)
}

func TestUnmarshalJSON_Failure(t *testing.T) {
	jsonData := `{
		"key1": "value1",
		"key2": {"nested": "object"}
	}`

	var m Metadata
	err := json.Unmarshal([]byte(jsonData), &m)
	require.Error(t, err)
	require.Equal(t, "unsupported value type", err.Error())
}

func TestMerge(t *testing.T) {
	m1 := Metadata{
		"Key1": "Value1",
		"Key2": 42,
	}
	m2 := Metadata{
		"Key2": 100,
		"Key3": true,
	}

	expected := Metadata{
		"Key1": "Value1",
		"Key2": 100,
		"Key3": true,
	}

	result := m1.Merge(m2)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestMergeInPlace(t *testing.T) {
	m1 := Metadata{
		"Key1": "Value1",
		"Key2": 42,
	}
	m2 := Metadata{
		"Key2": 100,
		"Key3": true,
	}

	expected := Metadata{
		"Key1": "Value1",
		"Key2": 100,
		"Key3": true,
	}

	m1.MergeInPlace(m2)

	if !reflect.DeepEqual(m1, expected) {
		t.Errorf("expected %v, got %v", expected, m1)
	}
}
