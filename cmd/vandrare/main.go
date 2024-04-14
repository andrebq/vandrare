package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/andrebq/vandrare/cmd/vandrare/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := app.Run(ctx, os.Args)
	if err != nil {
		slog.Error("Application failed", "err", err)
		log.Fatal("abort")
	}

	<-ctx.Done()
}
