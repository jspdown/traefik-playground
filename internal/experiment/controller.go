package experiment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/jspdown/traefik-playground/internal/command"
	"github.com/jspdown/traefik-playground/internal/traefik"
)

// ErrRunTimeout indicates that the ran experiment has timed out.
var ErrRunTimeout = errors.New("timed out while waiting for response")

// TraefikRunner can run requests through a fake Traefik instance.
type TraefikRunner interface {
	Run(ctx context.Context, dynamicConfig string, req *http.Request) (*http.Response, []traefik.Log, error)
}

// Storer can store Experiments and Results.
type Storer interface {
	Get(ctx context.Context, id string) (Experiment, Result, error)
	Save(ctx context.Context, exp Experiment, res Result, clientIP string) (string, error)
}

// Controller controls Experiments.
type Controller struct {
	store   Storer
	traefik TraefikRunner
}

// NewController creates a new Controller.
func NewController(store Storer, traefik TraefikRunner) *Controller {
	return &Controller{
		store:   store,
		traefik: traefik,
	}
}

// Run runs the given experiment.
func (c *Controller) Run(ctx context.Context, exp Experiment) (Result, error) {
	testReq := httptest.NewRequestWithContext(ctx, exp.Request.Method, exp.Request.URL, strings.NewReader(exp.Request.Body))
	testReq.Header = exp.Request.Headers

	res, logs, err := c.traefik.Run(ctx, exp.DynamicConfig, testReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return Result{}, ErrRunTimeout
		}

		return Result{}, fmt.Errorf("running Traefik experiment: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return Result{}, fmt.Errorf("reading Traefik result response body: %w", err)
	}

	return Result{
		Response: HTTPResponse{
			Proto:      res.Proto,
			StatusCode: res.StatusCode,
			Headers:    res.Header,
			Body:       body,
		},
		Logs: logs,
	}, nil
}

// Share saves an experiment with its result to the store. The returned string is a unique
// ID that can be used to retrieve the experiment later with Shared.
func (c *Controller) Share(ctx context.Context, exp Experiment, res Result, clientIP string) (string, error) {
	return c.store.Save(ctx, exp, res, clientIP)
}

// Shared retrieves a previously shared experiment and its result from the store using the given ID.
func (c *Controller) Shared(ctx context.Context, id string) (exp Experiment, res Result, err error) {
	return c.store.Get(ctx, id)
}

// Traefik provides functionality to execute Traefik experiments by spawning commands
// to a fake Traefik instance and collecting the results.
type Traefik struct {
	workerPool *command.WorkerPool
	timeout    time.Duration
}

// NewTraefik creates a new Traefik runner.
// Timeout specifies how long to wait before canceling commands.
func NewTraefik(workerPool *command.WorkerPool, timeout time.Duration) *Traefik {
	return &Traefik{
		workerPool: workerPool,
		timeout:    timeout,
	}
}

// Run executes a request against a fakeTraefik with the provided configuration.
func (r *Traefik) Run(ctx context.Context, dynamicConfig string, req *http.Request) (*http.Response, []traefik.Log, error) {
	cmd, err := traefik.NewCommand(dynamicConfig, req)
	if err != nil {
		return nil, nil, fmt.Errorf("creating Traefik command: %w", err)
	}

	if err = r.workerPool.Spawn(ctx, command.NewWithTimeout(cmd, r.timeout)); err != nil {
		return nil, nil, err
	}

	res, logs, err := cmd.Result()
	if err != nil {
		return nil, nil, fmt.Errorf("getting Traefik result: %w", err)
	}

	return res, logs, nil
}
