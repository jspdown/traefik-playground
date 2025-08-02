package traefik

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/jspdown/traefik-playground/internal/command"
	"github.com/rs/zerolog/log"
)

var _ command.Command = (*Command)(nil)

// Command spawns a fake Traefik instance using a given dynamic configuration and sends an HTTP request.
type Command struct {
	dynamicConfig string
	request       *http.Request

	stdout bytes.Buffer
	stderr bytes.Buffer
}

// NewCommand creates a new Command.
func NewCommand(dynamicConfig string, req *http.Request) (*Command, error) {
	return &Command{
		dynamicConfig: dynamicConfig,
		request:       req,
	}, nil
}

// Exec executes the command.
func (c *Command) Exec(ctx context.Context) error {
	logger := log.Ctx(ctx).With().Logger()

	reqBuffer := bytes.NewBuffer(nil)
	if err := c.request.Write(reqBuffer); err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	cmd := command.NewIsolatedCommand(ctx, []command.MountPoint{
		{Host: "/app", Target: "/app"},
	}, "/app/traefik-playground", "tester",
		"--request", reqBuffer.String(),
		"--log-level=debug",
	)
	cmd.Stdout = &c.stdout
	cmd.Stderr = &c.stderr

	commandIn, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("setting up child process stdin pipe: %w", err)
	}

	if err = cmd.Start(); err != nil {
		_ = commandIn.Close()

		return fmt.Errorf("starting command: %w", err)
	}

	if _, err = commandIn.Write([]byte(c.dynamicConfig)); err != nil {
		_ = commandIn.Close()

		return fmt.Errorf("writing request on child process stdin: %w", err)
	}

	if err = commandIn.Close(); err != nil {
		return fmt.Errorf("closing child process stdin: %w", err)
	}

	if err = cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			logger.Error().Err(err).
				Str("stderr", c.stderr.String()).
				Str("stdout", c.stdout.String()).
				Msg("Command has failed")

			c.stderr.Write([]byte(fmt.Sprintf("\n\ncommand failed with status %d", exitErr.ExitCode())))

			return nil
		}

		return fmt.Errorf("running command: %w", err)
	}

	return nil
}

// Result returns the HTTP response and logs of the previously run command.
func (c *Command) Result() (*http.Response, []Log, error) {
	res, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(c.stdout.Bytes())), c.request)
	if err != nil {
		return nil, nil, fmt.Errorf("reading response: %w", err)
	}

	logs := ParseRawLogs(c.stderr.String())

	return res, logs, nil
}
