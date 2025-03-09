package metadata

import (
	"encoding/base64"
	"net/textproto"
	"strconv"
	"strings"
)

type Metadata map[string]any

func (m Metadata) Get(key string) any {
	return m[textproto.CanonicalMIMEHeaderKey(key)]
}

func (m Metadata) Del(key string) {
	delete(m, textproto.CanonicalMIMEHeaderKey(key))
}

func (m Metadata) Has(key string) bool {
	_, ok := m[textproto.CanonicalMIMEHeaderKey(key)]
	return ok
}

func (m Metadata) Keys() []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func (m Metadata) Clone() Metadata {
	clone := make(Metadata, len(m))
	for key, value := range m {
		clone[key] = value
	}
	return clone
}

func (m Metadata) GetInt64(key string) (int64, bool) {
	value, ok := m[textproto.CanonicalMIMEHeaderKey(key)].(int64)
	return value, ok
}

func (m Metadata) SetInt64(key string, value int64) {
	m[textproto.CanonicalMIMEHeaderKey(key)] = value
}

func (m Metadata) GetFloat64(key string) (float64, bool) {
	value, ok := m[textproto.CanonicalMIMEHeaderKey(key)].(float64)
	return value, ok
}

func (m Metadata) SetFloat64(key string, value float64) {
	m[textproto.CanonicalMIMEHeaderKey(key)] = value
}

func (m Metadata) GetString(key string) string {
	value := m[textproto.CanonicalMIMEHeaderKey(key)]
	switch v := value.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case []byte:
		return base64.StdEncoding.EncodeToString(v)
	default:
		return ""
	}
}

func (m Metadata) SetString(key string, value string) {
	m[textproto.CanonicalMIMEHeaderKey(key)] = value
}

func (m Metadata) GetBool(key string) (bool, bool) {
	value, ok := m[textproto.CanonicalMIMEHeaderKey(key)].(bool)
	return value, ok
}

func (m Metadata) SetBool(key string, value bool) {
	m[textproto.CanonicalMIMEHeaderKey(key)] = value
}

func (m Metadata) GetBytes(key string) ([]byte, bool) {
	value, ok := m[textproto.CanonicalMIMEHeaderKey(key)].([]byte)
	return value, ok
}

func (m Metadata) SetBytes(key string, value []byte) {
	m[textproto.CanonicalMIMEHeaderKey(key)] = value
}

func (m Metadata) String() string {
	var sb strings.Builder
	for key := range m {
		sb.WriteString(key)
		sb.WriteString(": ")
		sb.WriteString(m.GetString(key))
		sb.WriteString("\n")
	}
	return sb.String()
}
