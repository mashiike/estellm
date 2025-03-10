package estellm_test

import (
	"context"
	"os"
	"testing"

	"github.com/mashiike/estellm"
	"github.com/stretchr/testify/require"
)

type mockAgent struct{}

func (m *mockAgent) Execute(ctx context.Context, req *estellm.Request, w estellm.ResponseWriter) error {
	return nil
}

func mockAgentFunc(ctx context.Context, p *estellm.Prompt) (estellm.Agent, error) {
	return &mockAgent{}, nil
}

func TestRegistry(t *testing.T) {
	registry := estellm.NewRegistry()
	ctx := context.Background()
	// Test Register
	err := registry.Register("test_agent", mockAgentFunc)
	require.NoError(t, err)

	// Test Register duplicate
	err = registry.Register("test_agent", mockAgentFunc)
	require.ErrorIs(t, err, estellm.ErrAgentAlreadyRegistered)

	// Test NewAgent
	l := estellm.NewLoader()
	p, err := l.Load(ctx, os.DirFS("testdata"), "exists.md")
	require.NoError(t, err)
	agent, err := registry.NewAgent(ctx, p)
	require.NoError(t, err)
	require.IsType(t, &mockAgent{}, agent)

	// Test NewAgent with non-existent type
	p, err = l.Load(ctx, os.DirFS("testdata"), "not_exists.md")
	require.NoError(t, err)
	agent, err = registry.NewAgent(ctx, p)
	require.ErrorIs(t, err, estellm.ErrAgentTypeNotFound)
	require.Nil(t, agent)
}
