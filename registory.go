package estellm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"text/template"
)

var defaultRegistory = NewRegistry()

type NewAgentFunc func(context.Context, *Prompt) (Agent, error)

// Registry is a registry of agents.
type Registry struct {
	mu                  sync.RWMutex
	newFuncs            map[string]NewAgentFunc
	templateFuncs       map[string]template.FuncMap
	marmaidNodeWrappers map[string]func(string) string
}

// NewRegistry creates a new registry.
func NewRegistry() *Registry {
	return &Registry{
		newFuncs: make(map[string]NewAgentFunc),
	}
}

// Errors returned by the registry.
var (
	ErrInvalidConfig          = errors.New("invalid config")
	ErrAgentTypeEmpty         = errors.New("agent type is empty")
	ErrAgentAlreadyRegistered = errors.New("agent already registered")
	ErrAgentTypeNotFound      = errors.New("agent type not found")
)

// Register registers a new agent.
func (r *Registry) Register(name string, f NewAgentFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" {
		return ErrAgentTypeEmpty
	}
	if _, ok := r.newFuncs[name]; ok {
		return ErrAgentAlreadyRegistered
	}
	r.newFuncs[name] = f
	return nil
}

func (r *Registry) SetTemplateFuncs(name string, funcs template.FuncMap) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" {
		return ErrAgentTypeEmpty
	}
	if _, ok := r.newFuncs[name]; !ok {
		return fmt.Errorf("type `%s`: %w", name, ErrAgentTypeNotFound)
	}
	if r.templateFuncs == nil {
		r.templateFuncs = make(map[string]template.FuncMap)
	}
	r.templateFuncs[name] = funcs
	if _, err := mergeFuncMaps(r.templateFuncs); err != nil {
		return err
	}
	return nil
}

func (r *Registry) SetMarmaidNodeWrapper(name string, f func(string) string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" {
		return ErrAgentTypeEmpty
	}
	if _, ok := r.newFuncs[name]; !ok {
		return fmt.Errorf("type `%s`: %w", name, ErrAgentTypeNotFound)
	}
	if r.marmaidNodeWrappers == nil {
		r.marmaidNodeWrappers = make(map[string]func(string) string)
	}
	r.marmaidNodeWrappers[name] = f
	return nil
}

func (r *Registry) getTemplateFuncs(name string) template.FuncMap {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if name == "" {
		return template.FuncMap{}
	}
	if r.templateFuncs == nil {
		return template.FuncMap{}
	}
	return r.templateFuncs[name]
}

func (r *Registry) getMergedTemplateFuncs() template.FuncMap {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.templateFuncs == nil {
		return template.FuncMap{}
	}
	if fmap, err := mergeFuncMaps(r.templateFuncs); err == nil {
		return fmap
	}
	return template.FuncMap{}
}

func (r *Registry) getMarmaidNodeWrapper(name string) func(string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if name == "" {
		return nil
	}
	if r.marmaidNodeWrappers == nil {
		return nil
	}
	return r.marmaidNodeWrappers[name]
}

// Exists returns true if the agent type is registered.
func (r *Registry) Exists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.newFuncs[name]
	return ok
}

// NewAgent creates a new agent.
func (r *Registry) NewAgent(ctx context.Context, p *Prompt) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg := p.Config()
	if cfg == nil {
		return nil, ErrInvalidConfig
	}
	if cfg.Type == "" {
		return nil, ErrAgentTypeEmpty
	}
	agentType := cfg.Type
	f, ok := r.newFuncs[agentType]
	if !ok {
		return nil, fmt.Errorf("type `%s`: %w", agentType, ErrAgentTypeNotFound)
	}
	return f(ctx, p)
}

// RegisterAgent registers a new agent. to the default registry.
func RegisterAgent(name string, f NewAgentFunc) error {
	return defaultRegistory.Register(name, f)
}

// SetAgentTemplateFuncs sets template functions for the agent type.
func SetAgentTemplateFuncs(name string, funcs template.FuncMap) error {
	return defaultRegistory.SetTemplateFuncs(name, funcs)
}

func SetAgentMarmaidNodeWrapper(name string, f func(string) string) error {
	return defaultRegistory.SetMarmaidNodeWrapper(name, f)
}

// DefaultRegistory returns the default registry.
func DefaultRegistory() *Registry {
	return defaultRegistory
}
