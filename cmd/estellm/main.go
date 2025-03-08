package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/fatih/color"
	"github.com/mashiike/estellm/cli"
	"github.com/mashiike/slogutils"
)

func main() {
	setupLogger(slog.LevelInfo)
	if code := run(context.Background()); code != 0 {
		os.Exit(code)
	}
}

func run(ctx context.Context) int {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	c, err := cli.New(setupLogger)
	if err != nil {
		slog.Error("failed to create estellm app", "details", err)
		return 1
	}
	if err := c.Run(ctx); err != nil {
		slog.Error("failed to run estellm app", "details", err)
		return 1
	}
	return 0
}

func setupLogger(level slog.Level) {
	middleware := slogutils.NewMiddleware(
		slog.NewJSONHandler,
		slogutils.MiddlewareOptions{
			ModifierFuncs: map[slog.Level]slogutils.ModifierFunc{
				slog.LevelDebug: slogutils.Color(color.FgBlack),
				slog.LevelInfo:  nil,
				slog.LevelWarn:  slogutils.Color(color.FgYellow),
				slog.LevelError: slogutils.Color(color.FgRed, color.Bold),
			},
			Writer: os.Stderr,
			HandlerOptions: &slog.HandlerOptions{
				Level: level,
			},
		},
	)
	logger := slog.New(middleware)
	slog.SetDefault(logger)
}
