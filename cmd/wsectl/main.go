package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/pbv7/wsectl/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := app.Run(ctx, os.Args[1:]); err != nil {
		os.Exit(app.ExitCode(err))
	}
}
