package server

import (
	"context"
	"time"

	"github.com/ettle/strcase"
	"github.com/jspdown/traefik-playground/internal/logger"
	"github.com/urfave/cli/v3"
)

const (
	flagAddr               = "addr"
	flagLogLevel           = "log-level"
	flagLogFormat          = "log-format"
	flagDatabaseConnString = "db"
	flagSecretKey          = "secret-key"
	flagTesterTimeout      = "tester-timeout"
	flagMaxProcesses       = "max-processes"
	flagMaxPendingCommands = "max-pending-commands"
)

// NewCommand creates the server CLI command.
func NewCommand() *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "Starts server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     flagAddr,
				Usage:    "Address to listen on",
				Sources:  cli.EnvVars(strcase.ToSNAKE(flagAddr)),
				Required: true,
			},
			&cli.StringFlag{
				Name:  flagLogLevel,
				Usage: "Log level (debug, info, error)",
				Value: "info",
			},
			&cli.StringFlag{
				Name:  flagLogFormat,
				Usage: "Log format (console, json)",
				Value: "json",
			},
			&cli.StringFlag{
				Name:     flagDatabaseConnString,
				Usage:    "Database connection string to a PostgreSQL database",
				Sources:  cli.EnvVars(strcase.ToSNAKE(flagDatabaseConnString)),
				Required: true,
			},
			&cli.StringFlag{
				Name:     flagSecretKey,
				Usage:    "Secret key to use for experiment response signing",
				Sources:  cli.EnvVars(strcase.ToSNAKE(flagSecretKey)),
				Required: true,
			},
			&cli.DurationFlag{
				Name:    flagTesterTimeout,
				Usage:   "Duration before the experiment is canceled",
				Sources: cli.EnvVars(strcase.ToSNAKE(flagTesterTimeout)),
				Value:   2 * time.Second,
			},
			&cli.IntFlag{
				Name:    flagMaxProcesses,
				Usage:   "Maximum number of concurrent test processes",
				Sources: cli.EnvVars(strcase.ToSNAKE(flagMaxProcesses)),
				Value:   100,
			},
			&cli.IntFlag{
				Name:    flagMaxPendingCommands,
				Usage:   "Maximum number commands that can be waiting to be executed",
				Sources: cli.EnvVars(strcase.ToSNAKE(flagMaxPendingCommands)),
				Value:   2000,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := logger.Configure(cmd.String(flagLogLevel), cmd.String(flagLogFormat)); err != nil {
				return err
			}

			s, err := New(Config{
				Addr:               cmd.String(flagAddr),
				DatabaseConnString: cmd.String(flagDatabaseConnString),
				SecretKey:          cmd.String(flagSecretKey),
				TesterTimeout:      cmd.Duration(flagTesterTimeout),
				MaxPendingCommands: cmd.Int(flagMaxPendingCommands),
				MaxProcesses:       cmd.Int(flagMaxProcesses),
			})
			if err != nil {
				return err
			}

			return s.Start(ctx)
		},
	}
}
