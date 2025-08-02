package traefik

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
)

func TestTraefik(t *testing.T) {
	t.Parallel()

	dynamicConfigFile, err := os.Open("testdata/dynamic.config.json")
	require.NoError(t, err)

	var dynamicConfig dynamic.Configuration
	err = json.NewDecoder(dynamicConfigFile).Decode(&dynamicConfig)
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "https://example.com/foo", strings.NewReader(`{"foo": "bar"}`))
	request.Header.Set("X-Header", "Value")

	traefik, err := NewTraefik(&dynamicConfig)
	require.NoError(t, err)

	readyCh := make(chan struct{})
	traefik.OnReady(func() {
		close(readyCh)
	})

	require.NoError(t, traefik.Start(t.Context()))

	select {
	case <-readyCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Traefik to be ready")
	}

	res, err := traefik.Send(request)
	require.NoError(t, err)

	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusTeapot, res.StatusCode)
	assert.Equal(t, "response", res.Header.Get("X-Response-Header"))

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	assert.Equal(t,
		"POST /foo HTTP/1.1\r\n"+
			"Host: example.com\r\n"+
			"User-Agent: Go-http-client/1.1\r\n"+
			"Content-Length: 14\r\n"+
			"Accept-Encoding: gzip\r\n"+
			"X-Forwarded-For: 192.0.2.1\r\n"+
			"X-Header: Value\r\n"+
			"X-Request-Header: request\r\n"+
			"\r\n"+
			`{"foo": "bar"}`, string(body))
}
