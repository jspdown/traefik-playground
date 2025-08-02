package command

import (
	"context"
	"fmt"
	"sync"
)

// WorkerPool is a pool of worker for executing commands limiting the maximum number
// of concurrent commands.
type WorkerPool struct {
	spawnSlots chan struct{}

	maxWaitQueueDepth int
	waitQueueDepth    int
	waitQueueDepthMu  sync.Mutex
}

// NewWorkerPool creates a new WorkerPool.
// When workers are not available, the Spawn method wait until one is available:
// - maxSlots controls the maximum number of concurrent workers.
// - maxWaitQueueDepth controls how many commands can wait for a worker to be available.
func NewWorkerPool(maxSlots int, maxWaitQueueDepth int) *WorkerPool {
	spawnSlots := make(chan struct{}, maxSlots)
	for range maxSlots {
		spawnSlots <- struct{}{}
	}

	return &WorkerPool{
		spawnSlots:        spawnSlots,
		maxWaitQueueDepth: maxWaitQueueDepth,
	}
}

// Spawn spawns a Command.
func (s *WorkerPool) Spawn(ctx context.Context, command Command) error {
	// Make sure it's worth trying to wait in the queue, otherwise abort immediately.
	s.waitQueueDepthMu.Lock()
	if s.waitQueueDepth >= s.maxWaitQueueDepth {
		s.waitQueueDepthMu.Unlock()

		return fmt.Errorf("too many commands in the queue: %w", context.DeadlineExceeded)
	}
	s.waitQueueDepth++
	s.waitQueueDepthMu.Unlock()

	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-s.spawnSlots:
	}

	s.waitQueueDepthMu.Lock()
	s.waitQueueDepth--
	s.waitQueueDepthMu.Unlock()

	if err != nil {
		return err
	}

	defer func() {
		s.spawnSlots <- struct{}{}
	}()

	return command.Exec(ctx)
}
