package main

import (
	"context"
	"os"

	"github.com/pbv7/wsectl/internal/app"
)

func main() {
	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		os.Exit(app.ExitCode(err))
	}
}
