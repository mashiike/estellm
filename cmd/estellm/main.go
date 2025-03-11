package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/mashiike/estellm/cli"

	//builtin agents import
	_ "github.com/mashiike/estellm/agent/constant"
	_ "github.com/mashiike/estellm/agent/decision"
	_ "github.com/mashiike/estellm/agent/genimage"
	_ "github.com/mashiike/estellm/agent/gentext"

	//builtin providers import
	_ "github.com/mashiike/estellm/provider/bedrock"
	_ "github.com/mashiike/estellm/provider/openai"
)

func main() {
	if code := run(context.Background()); code != 0 {
		os.Exit(code)
	}
}

func run(ctx context.Context) int {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	var c cli.CLI
	return c.Run(ctx)
}
