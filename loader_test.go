package estellm_test

import (
	"context"
	"maps"
	"math/rand/v2"
	"os"
	"slices"
	"testing"

	"github.com/mashiike/estellm"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

func TestLoader(t *testing.T) {
	loader := estellm.NewLoader()
	loader.Includes(os.DirFS("testdata/loadertest/includes"))
	seed := [32]byte{0}
	gen := estellm.NewSchemaValueGenerator(rand.New(rand.NewChaCha8(seed)))
	loader.ValueGenerator(gen)
	ctx := context.Background()

	prompts, _, err := loader.LoadFS(ctx, os.DirFS("testdata/loadertest/prompts"))
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"cot",
		"hoge",
		"before1",
		"before2",
		"before3",
	}, slices.Collect(maps.Keys(prompts)))
	t.Run("cot", func(t *testing.T) {
		p := prompts["cot"]
		cfg := p.Config()
		require.NotNil(t, cfg)
		require.Equal(t, "cot.md", cfg.PromptPath)
		require.Equal(t, "cot", p.Name())
		require.Equal(t, "test_agent", cfg.Type)
		require.ElementsMatch(t,
			[]string{"before1", "before2", "before3"},
			cfg.DependsOn,
		)
		g := goldie.New(
			t,
			goldie.WithFixtureDir("testdata/fixtures/cot"),
		)
		g.WithNameSuffix(".golden.jsonnet")
		g.Assert(t, "config", []byte(cfg.Raw))
		g.WithNameSuffix(".golden.md")
		g.Assert(t, "pre_render", []byte(p.PreRendered()))
		req, err := estellm.NewRequest("cot", map[string]interface{}{})
		require.NoError(t, err)
		_, err = p.Render(ctx, req)
		require.Error(t, err)
		var ve *estellm.DataValidateError
		require.ErrorAs(t, err, &ve)
		require.Len(t, ve.Result.Errors(), 1)
		data := map[string]interface{}{
			"numbers": []int{1, 2, 3, 4, 5},
		}
		req, err = estellm.NewRequest("cot", data)
		require.NoError(t, err)
		req.PreviousResults = map[string]*estellm.Response{
			"before1": {
				Message: estellm.Message{
					Role: estellm.RoleAssistant,
					Parts: []estellm.ContentPart{
						estellm.TextPart("This is before1 message."),
					},
				},
			},
			"before2": {
				Message: estellm.Message{
					Role: estellm.RoleAssistant,
					Parts: []estellm.ContentPart{
						estellm.TextPart("This is before2 message."),
					},
				},
			},
			"before3": {
				Message: estellm.Message{
					Role: estellm.RoleAssistant,
					Parts: []estellm.ContentPart{
						estellm.TextPart("This is before3 message."),
					},
				},
			},
		}
		renderd, err := p.Render(ctx, req)
		require.NoError(t, err)
		g.Assert(t, "rendered", []byte(renderd))
		system, messages, err := p.Decode(ctx, req)
		require.NoError(t, err)
		g.Assert(t, "decoded_system_prompt", []byte(system))
		g.WithNameSuffix(".golden.json")
		g.AssertJson(t, "decoded_messages", messages)
		var additionalConfig struct {
			ModelProvider string `json:"model_provider"`
			ModelID       string `json:"model_id"`
		}
		err = p.Config().Decode(&additionalConfig)
		require.NoError(t, err)
		require.Equal(t, "bedrock", additionalConfig.ModelProvider)
		require.Equal(t, "anthropic.claude-3-5-sonnet-20241022-v2:0", additionalConfig.ModelID)
		require.ElementsMatch(t, []string{}, p.Blocks())
	})
	t.Run("config_dump", func(t *testing.T) {
		p := prompts["hoge"]
		cfg := p.Config()
		require.NotNil(t, cfg)
		require.Equal(t, "nested/config_dump.md", cfg.PromptPath)
		require.Equal(t, "hoge", p.Name())
		require.Equal(t, "test_agent", cfg.Type)
		require.ElementsMatch(t,
			[]string{},
			cfg.DependsOn,
		)
		g := goldie.New(
			t,
			goldie.WithFixtureDir("testdata/fixtures/config_dump"),
		)
		g.WithNameSuffix(".golden.jsonnet")
		g.Assert(t, "config", []byte(cfg.Raw))
		g.WithNameSuffix(".golden.md")
		g.Assert(t, "pre_render", []byte(p.PreRendered()))
		req, err := estellm.NewRequest("hoge", map[string]interface{}{})
		require.NoError(t, err)
		renderd, err := p.Render(ctx, req)
		require.NoError(t, err)
		g.Assert(t, "rendered", []byte(renderd))
		require.ElementsMatch(t, []string{"dummy_block"}, p.Blocks())
	})
}
