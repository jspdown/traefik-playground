package traefik

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhoami(t *testing.T) {
	t.Parallel()

	server := NewWhoami()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusTeapot, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, "GET / HTTP/1.1\r\n"+
		fmt.Sprintf("Host: %s\r\n", server.Listener.Addr().String())+
		"User-Agent: Go-http-client/1.1\r\n"+
		"Accept-Encoding: gzip\r\n"+
		"\r\n", string(bodyBytes))
}
