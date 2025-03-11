package estellm_test

import (
	"context"
	"fmt"
	"math/rand/v2"
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
		{
			name:     "cycle3",
			includes: "testdata/cycle3/includes",
			prompts:  "testdata/cycle3/prompts",
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
	seed := [32]byte{1}
	randReader := rand.New(rand.NewChaCha8(seed))
	gen := estellm.NewSchemaValueGenerator(randReader)
	var executionHistory strings.Builder
	reg := estellm.NewRegistry()
	reg.Register("test_agent", estellm.NewAgentFunc(func(ctx context.Context, p *estellm.Prompt) (estellm.Agent, error) {
		return estellm.AgentFunc(func(ctx context.Context, req *estellm.Request, rw estellm.ResponseWriter) error {
			for _, tool := range req.Tools {
				fmt.Fprintf(&executionHistory, "call tool `%s` \n", tool.Name())
				toolPayload, err := gen.Generate(tool.InputSchema())
				require.NoError(t, err)
				err = tool.Call(ctx, toolPayload, rw)
				require.NoError(t, err)
			}
			w := estellm.ResponseWriterToWriter(rw)
			fmt.Fprintf(w, "execute %s \n", p.Name())
			fmt.Fprintf(&executionHistory, "execute %s \n", p.Name())
			return nil
		}), nil
	}))
	reg.Register("test_decision", estellm.NewAgentFunc(func(ctx context.Context, p *estellm.Prompt) (estellm.Agent, error) {
		return estellm.AgentFunc(func(ctx context.Context, req *estellm.Request, rw estellm.ResponseWriter) error {
			deps := p.Config().Dependents()
			w := estellm.ResponseWriterToWriter(rw)
			fmt.Fprintf(w, "execute %s \n", p.Name())
			if len(deps) > 1 {
				index := randReader.IntN(len(deps))
				dep := deps[index]
				fmt.Fprintf(&executionHistory, "decision %s -> %s \n", p.Name(), dep)
				estellm.SetNextAgents(rw, dep)
				rw.WritePart(estellm.ReasoningPart(fmt.Sprintf("decision %s -> %s", p.Name(), dep)))
			}
			fmt.Fprintf(&executionHistory, "execute %s \n", p.Name())
			return nil
		}), nil
	}))
	reg.SetMarmaidNodeWrapper("test_decision", func(name string) string {
		return fmt.Sprintf("{%s}", name)
	})
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
		name            string
		includes        string
		prompts         string
		start           string
		payload         any
		includeUpstream bool
		skipStructure   bool
		middleware      []func(next estellm.Agent) estellm.Agent
	}{
		{
			name:     "simple",
			includes: "testdata/simple/includes",
			prompts:  "testdata/simple/prompts",
			start:    "start",
		},
		{
			name:          "simple_start_task_b",
			includes:      "testdata/simple/includes",
			prompts:       "testdata/simple/prompts",
			start:         "task_b",
			skipStructure: true,
		},
		{
			name:            "simple_include_upstream",
			includes:        "testdata/simple/includes",
			prompts:         "testdata/simple/prompts",
			start:           "task_b",
			includeUpstream: true,
			skipStructure:   true,
		},
		{
			name:     "tools",
			includes: "testdata/toolcall/includes",
			prompts:  "testdata/toolcall/prompts",
			start:    "main",
		},
		{
			name:     "decision",
			includes: "testdata/decision/includes",
			prompts:  "testdata/decision/prompts",
			start:    "main",
		},
		{
			name:          "decision_middlewares",
			includes:      "testdata/decision/includes",
			prompts:       "testdata/decision/prompts",
			start:         "main",
			skipStructure: true,
			middleware: []func(next estellm.Agent) estellm.Agent{
				func(next estellm.Agent) estellm.Agent {
					return estellm.AgentFunc(func(ctx context.Context, req *estellm.Request, rw estellm.ResponseWriter) error {
						w := estellm.ResponseWriterToWriter(rw)
						fmt.Fprintf(&executionHistory, "middleware1 \n")
						fmt.Fprintf(w, "middleware1 \n")
						return next.Execute(ctx, req, rw)
					})
				},
			},
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
			mux.Use(c.middleware...)
			if !c.skipStructure {
				g.Assert(t, c.name, []byte(mux.ToMarkdown()))
			}
			err = mux.Validate()
			require.NoError(t, err)
			executionHistory.Reset()
			req, err := estellm.NewRequest(c.start, c.payload)
			require.NoError(t, err)
			req.IncludeUpstream = c.includeUpstream
			w := estellm.NewBatchResponseWriter()
			err = mux.Execute(context.Background(), req, w)
			require.NoError(t, err)
			g2.Assert(t, c.name, []byte(executionHistory.String()))
			g3.AssertJson(t, c.name, w.Response())
		})
	}
}
