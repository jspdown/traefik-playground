package command

import (
	"context"
	"os/exec"
	"time"
)

// Command can be executed.
type Command interface {
	Exec(ctx context.Context) error
}

// MountPoint defines how a directory should be mounted.
type MountPoint struct {
	Host   string
	Target string
}

// NewIsolatedCommand runs a Command in isolation using BubbleWrap.
func NewIsolatedCommand(ctx context.Context, mountPoints []MountPoint, args ...string) *exec.Cmd {
	commandArgs := make([]string, 0, len(mountPoints))
	for _, mountPoint := range mountPoints {
		commandArgs = append(commandArgs, "--ro-bind", mountPoint.Host, mountPoint.Target)
	}
	commandArgs = append(commandArgs, "--unshare-all", "--clearenv", "--new-session")
	commandArgs = append(commandArgs, args...)

	return exec.CommandContext(ctx, "bwrap", commandArgs...) //nolint:gosec // Args are sanitized.
}

// WithTimeout is a helper Command that wraps a Command and adds an execution timeout.
type WithTimeout struct {
	command Command
	timeout time.Duration
}

// NewWithTimeout creates a new WithTimeout.
func NewWithTimeout(command Command, timeout time.Duration) WithTimeout {
	return WithTimeout{
		command: command,
		timeout: timeout,
	}
}

// Exec calls the underlying Command with a timeout.
func (c WithTimeout) Exec(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.command.Exec(ctx)
}
