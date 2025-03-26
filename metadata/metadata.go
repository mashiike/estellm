package metadata

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

func (m Metadata) Set(key string, value any) {
	switch v := value.(type) {
	case string:
		m.SetString(key, v)
	case int64:
		m.SetInt64(key, v)
	case float64:
		m.SetFloat64(key, v)
	case bool:
		m.SetBool(key, v)
	case []string:
		m.SetStrings(key, v)
	case []byte:
		m.SetBytes(key, v)
	default:
		slog.Warn("unsupported value type", "type", fmt.Sprintf("%T", value))
	}
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

func (m Metadata) Merge(other Metadata) Metadata {
	merged := m.Clone()
	for key, value := range other {
		merged[key] = value
	}
	return merged
}

func (m Metadata) MergeInPlace(other Metadata) {
	for key, value := range other {
		m[key] = value
	}
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

func (m Metadata) GetStrings(key string) []string {
	value := m[textproto.CanonicalMIMEHeaderKey(key)]
	switch v := value.(type) {
	case int64:
		return []string{strconv.FormatInt(v, 10)}
	case float64:
		return []string{strconv.FormatFloat(v, 'f', -1, 64)}
	case []string:
		return v
	case bool:
		return []string{strconv.FormatBool(v)}
	case []byte:
		return []string{base64.StdEncoding.EncodeToString(v)}
	default:
		return []string{}
	}
}

func (m Metadata) GetString(key string) string {
	strs := m.GetStrings(key)
	if len(strs) > 0 {
		return strs[0]
	}
	return ""
}

func (m Metadata) AddString(key string, value string) {
	strs, ok := m[textproto.CanonicalMIMEHeaderKey(key)].([]string)
	if !ok {
		m.SetString(key, value)
		return
	}
	m[textproto.CanonicalMIMEHeaderKey(key)] = append(strs, value)
}

func (m Metadata) SetString(key string, value string) {
	m[textproto.CanonicalMIMEHeaderKey(key)] = []string{value}
}

func (m Metadata) SetStrings(key string, values []string) {
	m[textproto.CanonicalMIMEHeaderKey(key)] = values
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
		sb.WriteString(strings.Join(m.GetStrings(key), ", "))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m *Metadata) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if *m == nil {
		*m = make(Metadata, len(raw))
	}
	for key, value := range raw {
		switch v := value.(type) {
		case string:
			if bs, err := base64.StdEncoding.DecodeString(v); err == nil {
				(*m).SetBytes(key, bs)
			} else {
				(*m).SetString(key, v)
			}
		case float64:
			if float64(int64(v)) == v {
				(*m).SetInt64(key, int64(v))
			} else {
				(*m).SetFloat64(key, v)
			}
		case bool:
			(*m).SetBool(key, v)
		case int:
			(*m).SetInt64(key, int64(v))
		case []any:
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					return errors.New("unsupported value type")
				}
				m.AddString(key, s)
			}
		default:
			return errors.New("unsupported value type")
		}
	}
	return nil
}

func SetInputTokens(metadata Metadata, tokens int64) {
	metadata.SetInt64("Usage-Input-Tokens", tokens)
}

func SetOutputTokens(metadata Metadata, tokens int64) {
	metadata.SetInt64("Usage-Output-Tokens", tokens)
}

func SetTotalTokens(metadata Metadata, tokens int64) {
	metadata.SetInt64("Usage-Total-Tokens", tokens)
}

func GetInputTokens(metadata Metadata) (int64, bool) {
	return metadata.GetInt64("Usage-Input-Tokens")
}

func GetOutputTokens(metadata Metadata) (int64, bool) {
	return metadata.GetInt64("Usage-Output-Tokens")
}

func GetTotalTokens(metadata Metadata) (int64, bool) {
	return metadata.GetInt64("Usage-Total-Tokens")
}
