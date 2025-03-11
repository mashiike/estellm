package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/mashiike/estellm"
	_ "github.com/mashiike/estellm/agent/gentext"
	_ "github.com/mashiike/estellm/provider/openai"
)

//go:embed prompts
var promptsFS embed.FS

func main() {
	ctx := context.Background()
	mux, err := estellm.NewAgentMux(ctx, estellm.WithPromptsFS(promptsFS))
	if err != nil {
		log.Fatalf("new agent mux: %v", err)
	}
	payload := map[string]interface{}{
		"numbers": []int{15, 5, 13, 7, 1},
	}
	req, err := estellm.NewRequest("simple", payload)
	if err != nil {
		log.Fatalf("new request: %v", err)
	}
	w := estellm.NewTextStreamingResponseWriter(os.Stdout)
	if err := mux.Execute(ctx, req, w); err != nil {
		log.Fatalf("execute: %v", err)
	}
}
