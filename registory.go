package estellm

import (
	"context"
	"errors"
	"sync"
	"text/template"
)

var defaultRegistory = NewRegistry()

type NewAgentFunc func(context.Context, *Prompt) (Agent, error)

// Registry is a registry of agents.
type Registry struct {
	mu            sync.RWMutex
	newFuncs      map[string]NewAgentFunc
	templateFuncs map[string]template.FuncMap
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
	ErrAgentNotFound          = errors.New("agent not found")
)

type registorOptions struct {
	templateFuncs template.FuncMap
}

type RegistorOption func(*registorOptions)

func (r *Registry) Register(name string, f NewAgentFunc, opts ...RegistorOption) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" {
		return ErrAgentTypeEmpty
	}
	if _, ok := r.newFuncs[name]; ok {
		return ErrAgentAlreadyRegistered
	}
	r.newFuncs[name] = f
	var options registorOptions
	for _, o := range opts {
		o(&options)
	}
	if options.templateFuncs != nil {
		if r.templateFuncs == nil {
			r.templateFuncs = make(map[string]template.FuncMap)
		}
		r.templateFuncs[name] = options.templateFuncs
	}
	return nil
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
	exectorType := cfg.Type
	f, ok := r.newFuncs[exectorType]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return f(ctx, p)
}

// Register registers a new agent. to the default registry.
func Register(name string, f NewAgentFunc) error {
	return defaultRegistory.Register(name, f)
}

// DefaultRegistory returns the default registry.
func DefaultRegistory() *Registry {
	return defaultRegistory
}
