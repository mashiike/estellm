package estellm

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/mashiike/estellm/metadata"
)

type GenerateTextRequest struct {
	Metadata    metadata.Metadata `json:"metadata"`
	ModelID     string            `json:"model_id"`
	ModelParams map[string]any    `json:"model_params"`
	System      string            `json:"system"`
	Messages    []Message         `json:"messages"`
	Tools       ToolSet           `json:"tools"`
}

type GenerateImageRequest struct {
	Metadata    metadata.Metadata `json:"metadata"`
	ModelID     string            `json:"model_id"`
	ModelParams map[string]any    `json:"model_params"`
	System      string            `json:"system"`
	Messages    []Message         `json:"messages"`
}

type ModelProvider interface {
	GenerateText(ctx context.Context, req *GenerateTextRequest, w ResponseWriter) error
	GenerateImage(ctx context.Context, req *GenerateImageRequest, w ResponseWriter) error
}

type ModelProviderManager struct {
	mu          sync.RWMutex
	providers   map[string]ModelProvider
	middlewares []func(ModelProvider) ModelProvider
}

var (
	ErrModelProviderNameEmpty = errors.New("model provider name is empty")
	ErrModelNotFound          = errors.New("model not found")
)

func (m *ModelProviderManager) Register(name string, provider ModelProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if name == "" {
		return ErrModelProviderNameEmpty
	}
	m.providers[name] = provider
	return nil
}

func (m *ModelProviderManager) Get(name string) (ModelProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	provider, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("model provider %s not found", name)
	}
	return provider, nil
}

func (m *ModelProviderManager) Exists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.providers[name]
	return ok
}

func (m *ModelProviderManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

func (m *ModelProviderManager) Clone() *ModelProviderManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	clone := NewModelProviderManager()
	for name, provider := range m.providers {
		clone.providers[name] = provider
	}
	return clone
}

func (m *ModelProviderManager) Use(middlewares ...func(ModelProvider) ModelProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.middlewares = append(m.middlewares, middlewares...)
}

func NewModelProviderManager() *ModelProviderManager {
	return &ModelProviderManager{
		providers: make(map[string]ModelProvider),
	}
}

var globalModelProviderManager = NewModelProviderManager()

type contextKey string

var modelProviderManagerContextKey = contextKey("modelProviderManager")

func WithModelProviderManager(ctx context.Context) (context.Context, *ModelProviderManager) {
	var modelProvider *ModelProviderManager
	if m, ok := modelProviderManagerFromContext(ctx); ok {
		modelProvider = m.Clone()
	} else {
		modelProvider = globalModelProviderManager.Clone()
	}
	return context.WithValue(ctx, modelProviderManagerContextKey, modelProvider), modelProvider
}

func modelProviderManagerFromContext(ctx context.Context) (*ModelProviderManager, bool) {
	m, ok := ctx.Value(modelProviderManagerContextKey).(*ModelProviderManager)
	return m, ok
}

func RegisterModelProvider(name string, provider ModelProvider) error {
	return globalModelProviderManager.Register(name, provider)
}

func GetModelProvider(ctx context.Context, name string) (ModelProvider, error) {
	manager, ok := modelProviderManagerFromContext(ctx)
	if !ok {
		manager = globalModelProviderManager
	}
	modelProvider, err := manager.Get(name)
	if err != nil {
		return nil, fmt.Errorf("model provider `%s`: %w", name, err)
	}
	for _, middleware := range manager.middlewares {
		modelProvider = middleware(modelProvider)
	}
	return modelProvider, nil
}

func UserModelProviderMiddlewares(middlewares ...func(ModelProvider) ModelProvider) {
	globalModelProviderManager.Use(middlewares...)
}
