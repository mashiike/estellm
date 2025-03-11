package estellm

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Songmu/flextime"
)

type Specification struct {
	// Name of the tool.
	Name string `json:"name"`
	// Description of the tool.
	Description string `json:"description,omitempty"`
	// InputSchema of the tool.
	InputSchema json.RawMessage `json:"input_schema"`
	// WorkerEndpoint of the tool.
	WorkerEndpoint string `json:"worker_endpoint"`
	// Extra
	Extra json.RawMessage `json:"-,omitempty"`
}

var DefaultSpecificationPath = "/.well-known/estellm-tool-specification"

func (s *Specification) UnmarshalJSON(data []byte) error {
	type alias Specification
	if err := json.Unmarshal(data, (*alias)(s)); err != nil {
		return err
	}
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(data, &extra); err != nil {
		return err
	}
	delete(extra, "name")
	delete(extra, "description")
	delete(extra, "input_schema")
	delete(extra, "worker_endpoint")
	if len(extra) == 0 {
		return nil
	}
	extraJSON, err := json.Marshal(extra)
	if err != nil {
		return fmt.Errorf("failed to marshal extra fields; %w", err)
	}
	s.Extra = extraJSON
	return nil
}

func (s *Specification) MarshalJSON() ([]byte, error) {
	data := make(map[string]any, len(s.Extra)+4)
	if s.Extra != nil {
		var extra map[string]json.RawMessage
		if err := json.Unmarshal(s.Extra, &extra); err != nil {
			return nil, err
		}
		for k, v := range extra {
			data[k] = v
		}
	}
	data["name"] = s.Name
	data["description"] = s.Description
	data["input_schema"] = s.InputSchema
	data["worker_endpoint"] = s.WorkerEndpoint
	return json.Marshal(data)
}

type SpecificationCache struct {
	mu             sync.RWMutex
	cache          map[string]Specification
	cacheAt        map[string]time.Time
	expireDuration time.Duration
}

func NewSpecificationCache(expireDuration time.Duration) *SpecificationCache {
	return &SpecificationCache{
		cache:          make(map[string]Specification),
		cacheAt:        make(map[string]time.Time),
		expireDuration: expireDuration,
	}
}

func (sc *SpecificationCache) Get(name string) (Specification, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	spec, ok := sc.cache[name]
	if !ok {
		return Specification{}, false
	}
	at, ok := sc.cacheAt[name]
	if !ok {
		return Specification{}, false
	}
	if flextime.Since(at) > sc.expireDuration {
		sc.mu.RUnlock()
		sc.Delete(name)
		sc.mu.RLock()
		return Specification{}, false
	}
	return spec, ok
}

func (sc *SpecificationCache) Set(name string, spec Specification) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cache[name] = spec
	sc.cacheAt[name] = flextime.Now()
}

func (sc *SpecificationCache) Delete(name string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.cache, name)
	delete(sc.cacheAt, name)
}

var DefaultSpecificationCache = NewSpecificationCache(15 * time.Minute)
