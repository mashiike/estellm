package estellm

import (
	"net/textproto"
)

type Metadata map[string]any

func (m Metadata) Get(key string) any {
	return m[textproto.CanonicalMIMEHeaderKey(key)]
}

func (m Metadata) Set(key string, value any) {
	m[textproto.CanonicalMIMEHeaderKey(key)] = value
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
