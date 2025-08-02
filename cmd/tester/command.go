package tester

import (
	"bufio"
	"context"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ettle/strcase"
	"github.com/jspdown/traefik-playground/internal/traefik"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"github.com/traefik/traefik/v3/pkg/logs"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

const (
	flagLogLevel = "log-level"
	flagRequest  = "request"
)

// NewCommand creates the tester CLI command.
func NewCommand() *cli.Command {
	return &cli.Command{
		Name:  "tester",
		Usage: "Test a request against a dynamic configuration received on the standard input",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  flagLogLevel,
				Usage: "Log level (debug, info, error)",
				Value: "INFO",
			},
			&cli.StringFlag{
				Name:     flagRequest,
				Usage:    "HTTP request to pass to the handler",
				Sources:  cli.EnvVars(strcase.ToSNAKE(flagRequest)),
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := initializeTraefikLogger(cmd.String(flagLogLevel)); err != nil {
				return err
			}

			var dynamicConfig dynamic.Configuration
			if err := yaml.NewDecoder(os.Stdin).Decode(&dynamicConfig); err != nil {
				return fmt.Errorf("decoding dynamic configuration: %w", err)
			}

			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			rawRequest := cmd.String(flagRequest)

			req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(rawRequest)))
			if err != nil {
				return fmt.Errorf("reading request: %w", err)
			}

			req = req.WithContext(ctx)

			instance, err := traefik.NewTraefik(&dynamicConfig)
			if err != nil {
				return fmt.Errorf("initializing Traefik instance: %w", err)
			}

			errCh := make(chan error)
			instance.OnReady(func() {
				res, sendErr := instance.Send(req)
				if sendErr != nil {
					errCh <- sendErr

					return
				}

				defer func() { _ = res.Body.Close() }()

				errCh <- res.Write(os.Stdout)
			})

			if err = instance.Start(ctx); err != nil {
				return fmt.Errorf("starting Traefik instance: %w", err)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case err = <-errCh:
				return err
			}
		},
	}
}

func initializeTraefikLogger(logLevel string) error {
	logCtx := zerolog.New(os.Stderr).With().Timestamp()

	level, err := zerolog.ParseLevel(strings.ToLower(logLevel))
	if err != nil {
		return fmt.Errorf("parsing log level: %w", err)
	}

	log.Logger = logCtx.Logger().Level(level)
	zerolog.DefaultContextLogger = &log.Logger
	zerolog.SetGlobalLevel(level)

	// Configure default standard log.
	stdlog.SetFlags(stdlog.Lshortfile | stdlog.LstdFlags)
	stdlog.SetOutput(logs.NoLevel(log.Logger, zerolog.DebugLevel))

	return nil
}
