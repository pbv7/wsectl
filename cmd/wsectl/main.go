package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/pbv7/wsectl/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, os.Args[1:]); err != nil {
		os.Exit(app.ExitCode(err))
	}
}
