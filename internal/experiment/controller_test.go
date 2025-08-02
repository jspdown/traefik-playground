package experiment_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jspdown/traefik-playground/internal/experiment"
	"github.com/jspdown/traefik-playground/internal/traefik"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStore implements a simple in-memory store for testing.
type fakeStore struct {
	experiments map[string]storedExperiment
	nextID      string
}

type storedExperiment struct {
	exp      experiment.Experiment
	res      experiment.Result
	clientIP string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		experiments: make(map[string]storedExperiment),
		nextID:      "test-id",
	}
}

func (s *fakeStore) Save(_ context.Context, exp experiment.Experiment, res experiment.Result, clientIP string) (string, error) {
	s.experiments[s.nextID] = storedExperiment{exp, res, clientIP}

	return s.nextID, nil
}

func (s *fakeStore) Get(_ context.Context, id string) (experiment.Experiment, experiment.Result, error) {
	if stored, ok := s.experiments[id]; ok {
		return stored.exp, stored.res, nil
	}

	return experiment.Experiment{}, experiment.Result{}, errors.New("not found")
}

// fakeTraefik implements a test double for the traefikRunner interface.
type fakeTraefik func(ctx context.Context, dynamicConfig string, req *http.Request) (*http.Response, []traefik.Log, error)

func (f fakeTraefik) Run(ctx context.Context, dynamicConfig string, req *http.Request) (*http.Response, []traefik.Log, error) {
	return f(ctx, dynamicConfig, req)
}

func TestController_Run(t *testing.T) {
	t.Parallel()

	dynamicConfig := `{
		"http": {
			"routers": {
				"api": {
					"rule": "PathPrefix(` + "`/foo`" + `)",
					"entryPoints": ["web"],
					"service": "whoami@playground"
				}
			}
		}
	}`

	fakeTraefik := fakeTraefik(func(_ context.Context, config string, req *http.Request) (*http.Response, []traefik.Log, error) {
		if config != dynamicConfig {
			return nil, nil, errors.New("unexpected dynamic config")
		}

		if strings.HasPrefix(req.URL.Path, "/foo") {
			return &http.Response{
				Proto:      "HTTP/1.1",
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("response")),
				Header:     http.Header{"X-Foo": {"Value"}},
			}, []traefik.Log{{Message: "found"}}, nil
		}

		return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil, nil
	})

	controller := experiment.NewController(newFakeStore(), fakeTraefik)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := controller.Run(ctx, experiment.Experiment{
		DynamicConfig: dynamicConfig,
		Request: experiment.HTTPRequest{
			Method: "GET",
			URL:    "https://example.com/foo/bar",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, experiment.Result{
		Response: experiment.HTTPResponse{
			Proto:      "HTTP/1.1",
			StatusCode: http.StatusOK,
			Headers:    map[string][]string{"X-Foo": {"Value"}},
			Body:       []byte("response"),
		},
		Logs: []traefik.Log{{Message: "found"}},
	}, result)
}

func TestController_Run_ContextCanceled(t *testing.T) {
	t.Parallel()

	traefik := fakeTraefik(func(ctx context.Context, dynamicConfig string, req *http.Request) (*http.Response, []traefik.Log, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}, nil, ctx.Err()
	})

	controller := experiment.NewController(newFakeStore(), traefik)

	// Create a context and immediately cancel it.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := controller.Run(ctx, experiment.Experiment{
		DynamicConfig: "{}",
		Request: experiment.HTTPRequest{
			Method: "GET",
			URL:    "http://example.com/foo/bar",
		},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, experiment.ErrRunTimeout)
}

func TestController_Run_Timeout(t *testing.T) {
	t.Parallel()

	traefik := fakeTraefik(func(ctx context.Context, dynamicConfig string, req *http.Request) (*http.Response, []traefik.Log, error) {
		// Simulate slow response.
		select {
		case <-time.After(time.Second):
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}, nil, nil
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	})

	controller := experiment.NewController(newFakeStore(), traefik)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := controller.Run(ctx, experiment.Experiment{
		DynamicConfig: "{}",
		Request: experiment.HTTPRequest{
			Method: "GET",
			URL:    "http://example.com/foo/bar",
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, experiment.ErrRunTimeout)
}

func TestController_Share(t *testing.T) {
	t.Parallel()

	controller := experiment.NewController(newFakeStore(), nil)

	exp := experiment.Experiment{
		Request: experiment.HTTPRequest{
			Method: "GET",
			URL:    "http://example.com",
		},
	}
	res := experiment.Result{
		Response: experiment.HTTPResponse{
			StatusCode: http.StatusOK,
		},
	}

	id, err := controller.Share(context.Background(), exp, res, "127.0.0.1")
	require.NoError(t, err)

	storedExp, storedRes, err := controller.Shared(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, exp, storedExp)
	assert.Equal(t, res, storedRes)
}
