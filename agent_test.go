package estellm_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mashiike/estellm"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

func TestNewAgentMux__Cycle(t *testing.T) {
	// simple cycle workflow
	reg := estellm.NewRegistry()
	reg.Register("test_agent", estellm.NewAgentFunc(func(ctx context.Context, p *estellm.Prompt) (estellm.Agent, error) {
		return estellm.AgentFunc(func(ctx context.Context, req *estellm.Request, rw estellm.ResponseWriter) error {
			w := estellm.ResponseWriterToWriter(rw)
			fmt.Fprintf(w, "execute %s \n", p.Name())
			return nil
		}), nil
	}))
	g := goldie.New(t,
		goldie.WithFixtureDir("testdata/fixtures/structure"),
		goldie.WithNameSuffix(".golden.md"),
	)
	cases := []struct {
		name     string
		includes string
		prompts  string
	}{
		{
			name:     "cycle1",
			includes: "testdata/cycle1/includes",
			prompts:  "testdata/cycle1/prompts",
		},
		{
			name:     "cycle2",
			includes: "testdata/cycle2/includes",
			prompts:  "testdata/cycle2/prompts",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mux, err := estellm.NewAgentMux(
				context.Background(),
				estellm.WithRegistry(reg),
				estellm.WithIncludesFS(os.DirFS(c.includes)),
				estellm.WithPromptsFS(os.DirFS(c.prompts)),
			)
			require.NoError(t, err)
			err = mux.Validate()
			require.Error(t, err)
			require.ErrorContains(t, err, "cycle detected")
			g.Assert(t, c.name, []byte(mux.ToMarkdown()))
		})
	}
}

func TestNewAgentMux__Execute(t *testing.T) {
	var executionHistory strings.Builder
	reg := estellm.NewRegistry()
	reg.Register("test_agent", estellm.NewAgentFunc(func(ctx context.Context, p *estellm.Prompt) (estellm.Agent, error) {
		return estellm.AgentFunc(func(ctx context.Context, req *estellm.Request, rw estellm.ResponseWriter) error {
			w := estellm.ResponseWriterToWriter(rw)
			fmt.Fprintf(w, "execute %s \n", p.Name())
			fmt.Fprintf(&executionHistory, "execute %s \n", p.Name())
			return nil
		}), nil
	}))
	g := goldie.New(t,
		goldie.WithFixtureDir("testdata/fixtures/structure"),
		goldie.WithNameSuffix(".golden.md"),
	)
	g2 := goldie.New(t,
		goldie.WithFixtureDir("testdata/fixtures/execution"),
		goldie.WithNameSuffix(".golden.md"),
	)
	g3 := goldie.New(t,
		goldie.WithFixtureDir("testdata/fixtures/response"),
		goldie.WithNameSuffix(".golden.json"),
	)
	cases := []struct {
		name          string
		includes      string
		prompts       string
		start         string
		payload       any
		includeDeps   bool
		skipStructure bool
	}{
		{
			name:     "simple",
			includes: "testdata/simple/includes",
			prompts:  "testdata/simple/prompts",
			start:    "start",
		},
		{
			name:     "simple_start_task_b",
			includes: "testdata/simple/includes",
			prompts:  "testdata/simple/prompts",
			start:    "task_b",
		},
		{
			name:        "simple_include_deps",
			includes:    "testdata/simple/includes",
			prompts:     "testdata/simple/prompts",
			start:       "task_b",
			includeDeps: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mux, err := estellm.NewAgentMux(
				context.Background(),
				estellm.WithRegistry(reg),
				estellm.WithIncludesFS(os.DirFS(c.includes)),
				estellm.WithPromptsFS(os.DirFS(c.prompts)),
			)
			require.NoError(t, err)
			err = mux.Validate()
			require.NoError(t, err)
			if !c.skipStructure {
				g.Assert(t, c.name, []byte(mux.ToMarkdown()))
			}
			executionHistory.Reset()
			req, err := estellm.NewRequest(c.start, c.payload)
			require.NoError(t, err)
			req.IncludeDeps = c.includeDeps
			w := estellm.NewBatchResponseWriter()
			err = mux.Execute(context.Background(), req, w)
			require.NoError(t, err)
			g2.Assert(t, c.name, []byte(executionHistory.String()))
			g3.AssertJson(t, c.name, w.Response())
		})
	}
}
