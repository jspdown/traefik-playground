package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/jspdown/traefik-playground/cmd/server"
	"github.com/jspdown/traefik-playground/cmd/tester"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "traefik-playground",
		Usage: "Playground for Traefik configuration",
		Commands: []*cli.Command{
			server.NewCommand(),
			tester.NewCommand(),
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	if err := app.Run(ctx, os.Args); err != nil {
		stop()
		log.Fatal().Err(err).Send()
	}

	stop()
}
