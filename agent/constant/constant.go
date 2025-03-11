package constant

import (
	"context"
	"fmt"

	"github.com/mashiike/estellm"
)

const (
	AgentName = "constant"
)

func init() {
	err := estellm.RegisterAgent(AgentName, NewAgent)
	if err != nil {
		panic(fmt.Sprintf("failed to register agent %s: %v", AgentName, err))
	}
}

type Agent struct {
	p *estellm.Prompt
}

func NewAgent(_ context.Context, p *estellm.Prompt) (estellm.Agent, error) {
	return &Agent{p: p}, nil
}

func (a *Agent) Execute(ctx context.Context, req *estellm.Request, w estellm.ResponseWriter) error {
	content, err := a.p.Render(ctx, req)
	if err != nil {
		return fmt.Errorf("render prompt: %w", err)
	}
	fmt.Fprint(estellm.ResponseWriterToWriter(w), content)
	w.Finish(estellm.FinishReasonEndTurn, "write content")
	return nil
}
