package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/andrebq/vandrare/cmd/vandrare/ssh"
	"github.com/urfave/cli/v2"
)

func Instance() *cli.App {
	loglevel := "info"
	return &cli.App{
		Commands: []*cli.Command{
			ssh.Cmd(),
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "Verbosity of log, valid values are: debug, info, warn, error",
				EnvVars:     []string{"VANDRARE_LOG_LEVEL"},
				Hidden:      false,
				Destination: &loglevel,
				Value:       loglevel,
			},
		},
		Before: func(ctx *cli.Context) error {
			level := slog.LevelInfo
			switch strings.ToLower(loglevel) {
			case "debug":
				level = slog.LevelDebug
			case "warn":
				level = slog.LevelWarn
			case "error":
				level = slog.LevelError
			}
			logger := slog.New(slog.NewTextHandler(ctx.App.ErrWriter, &slog.HandlerOptions{
				Level: level,
			}))
			slog.SetDefault(logger)
			return nil
		},
	}
}

func Run(ctx context.Context, args []string) error {
	app := Instance()
	return app.RunContext(ctx, args)
}
