package decision

import (
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/mashiike/estellm"
	"github.com/mashiike/estellm/jsonutil"
)

const (
	AgentName = "decision"
)

func init() {
	err := estellm.RegisterAgent(AgentName, NewAgent)
	if err != nil {
		panic(fmt.Sprintf("failed to register agent %s: %v", AgentName, err))
	}
	err = estellm.SetAgentTemplateFuncs(AgentName, template.FuncMap{
		"decisionSchema": func(agents []string) map[string]any {
			return newOutputSchema(agents)
		},
	})
	if err != nil {
		panic(fmt.Sprintf("failed to set template funcs for agent %s: %v", AgentName, err))
	}
	err = estellm.SetAgentMarmaidNodeWrapper(AgentName, func(s string) string {
		return fmt.Sprintf("{%s}", s)
	})
	if err != nil {
		panic(fmt.Sprintf("failed to set marmaid node wrapper for agent %s: %v", AgentName, err))
	}
}

type Config struct {
	ModelProvider      string         `json:"model_provider"`
	ModelID            string         `json:"model_id"`
	ModelParams        map[string]any `json:"model_params"`
	FallbackAgent      string         `json:"fallback_agent"`
	FallbackThreshould *float64       `json:"fallback_threshould"`
}

type Agent struct {
	p             *estellm.Prompt
	cfg           *Config
	modelProvider estellm.ModelProvider
}

type Output struct {
	NextAgent  string  `json:"next_agent"`
	Reasoning  string  `json:"reasoning"`
	Confidence float64 `json:"confidence"`
}

func NewAgent(ctx context.Context, p *estellm.Prompt) (estellm.Agent, error) {
	var cfg Config
	if err := p.Config().Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode `generate_text` agent config: %w", err)
	}
	if cfg.ModelProvider == "" {
		return nil, fmt.Errorf("model_provider is required")
	}
	if cfg.ModelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	modelProvider, err := estellm.GetModelProvider(ctx, cfg.ModelProvider)
	if err != nil {
		return nil, fmt.Errorf("model_provider `%s`: %w", cfg.ModelProvider, err)
	}
	return &Agent{
		p:             p,
		cfg:           &cfg,
		modelProvider: modelProvider,
	}, nil
}

func (a *Agent) Execute(ctx context.Context, req *estellm.Request, w estellm.ResponseWriter) error {
	system, msgs, err := a.p.Decode(ctx, req)
	if err != nil {
		return fmt.Errorf("decode prompt: %w", err)
	}
	modelReq := &estellm.GenerateTextRequest{
		ModelID:     a.cfg.ModelID,
		ModelParams: a.cfg.ModelParams,
		System:      system,
		Messages:    msgs,
		Tools:       req.Tools,
		Metadata:    req.Metadata,
	}
	batch := estellm.NewBatchResponseWriter()
	if err := a.modelProvider.GenerateText(ctx, modelReq, batch); err != nil {
		return fmt.Errorf("generate text: %w", err)
	}
	resp := batch.Response()
	var output Output
	if err := jsonutil.UnmarshalFirstJSON([]byte(resp.String()), &output); err != nil {
		return fmt.Errorf("extruct output: %w", err)
	}
	if output.Reasoning != "" {
		w.Metadata().SetString("Next-Agents-Reasoning", output.Reasoning)
		w.Metadata().SetFloat64("Next-Agents-Confidence", output.Confidence)
		w.WritePart(estellm.ReasoningPart(output.Reasoning))
	}
	nextAgent := output.NextAgent
	if a.cfg.FallbackThreshould != nil && output.Confidence < *a.cfg.FallbackThreshould {
		nextAgent = a.cfg.FallbackAgent
	}
	if nextAgent == "" {
		nextAgent = a.cfg.FallbackAgent
	}
	if nextAgent == "" {
		return errors.New("next_agent is empty")
	}
	estellm.SetNextAgents(w, nextAgent)
	w.Finish(estellm.FinishReasonEndTurn, "select next agent")
	return nil
}
