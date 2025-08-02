package command

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCommand struct {
	Executed bool
	delay    time.Duration
	mu       sync.Mutex
}

func (m *mockCommand) Exec(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(m.delay):
		m.Executed = true

		return nil
	}
}

func TestWorkerPool_Spawn_singleCommand(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(1, 1)
	cmd := &mockCommand{}

	err := pool.Spawn(context.Background(), cmd)
	require.NoError(t, err)
	assert.True(t, cmd.Executed, "command should have been executed")
}

func TestWorkerPool_Spawn_concurrentCommands(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(2, 2)

	var wg sync.WaitGroup

	errs := make([]error, 0)
	var errsMu sync.Mutex

	commands := make([]*mockCommand, 4)
	for i := range 4 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			cmd := &mockCommand{delay: 50 * time.Millisecond}
			commands[i] = cmd

			if err := pool.Spawn(context.Background(), cmd); err != nil {
				errsMu.Lock()
				errs = append(errs, err)
				errsMu.Unlock()
			}
		}()
	}

	wg.Wait()

	assert.Empty(t, errs, "no errors should have occurred")
	for i, cmd := range commands {
		assert.True(t, cmd.Executed, "command %d should have been executed", i)
	}
}

func TestWorkerPool_Spawn_queueFull(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(1, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Block the only worker.
	longCmd := &mockCommand{delay: time.Second}
	go func() { _ = pool.Spawn(ctx, longCmd) }()

	// Wait a bit to ensure the first command is running.
	time.Sleep(50 * time.Millisecond)

	// Try to spawn more commands than queue can handle.
	cmd := &mockCommand{}
	err := pool.Spawn(context.Background(), cmd)
	require.Error(t, err, "should return error when queue is full")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestWorkerPool_Spawn_contextCancellation(t *testing.T) {
	t.Parallel()

	pool := NewWorkerPool(1, 1)

	// Block the only worker.
	longCmd := &mockCommand{delay: time.Second}
	go func() { _ = pool.Spawn(context.Background(), longCmd) }()

	// Wait a bit to ensure the first command is running.
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := &mockCommand{}
	err := pool.Spawn(ctx, cmd)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, cmd.Executed)
}
