package experiment_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jspdown/traefik-playground/internal/experiment"
	"github.com/jspdown/traefik-playground/internal/traefik"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeExperiment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		dynamicConfig string
		method        string
		url           string
		headers       string
		body          string

		wantErr error
	}{
		{
			name:          "valid experiment",
			dynamicConfig: `http: {}`,
			method:        http.MethodGet,
			url:           "http://example.com",
			headers:       "Content-Type: application/json",
			body:          "test body",
		},
		{
			name:          "invalid dynamic config",
			dynamicConfig: "invalid yaml",
			method:        http.MethodGet,
			url:           "http://example.com",
			wantErr:       errors.New("invalid dynamic configuration: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `invalid...` into dynamic.Configuration"),
		},
		{
			name:          "empty method",
			dynamicConfig: "http:",
			method:        "",
			url:           "http://example.com",
			wantErr:       errors.New("request: method is required"),
		},
		{
			name:          "dynamic config too long",
			dynamicConfig: string(make([]byte, 65537)), // 65KB + 1 byte
			method:        http.MethodGet,
			url:           "http://example.com",
			wantErr:       errors.New("dynamic config too long (max: 10240)"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := experiment.MakeExperiment(test.dynamicConfig, test.method, test.url, test.headers, test.body)
			if test.wantErr != nil {
				require.EqualError(t, err, test.wantErr.Error())
			} else {
				require.NoError(t, err)
			}

			if err == nil {
				assert.Equal(t, test.dynamicConfig, got.DynamicConfig)
				assert.Equal(t, test.method, got.Request.Method)
				assert.Equal(t, test.url, got.Request.URL)
				assert.Equal(t, test.body, got.Request.Body)
			}
		})
	}
}

func TestMakeHTTPRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		method  string
		url     string
		headers string
		body    string

		wantErr error
	}{
		{
			name:    "valid request",
			method:  http.MethodGet,
			url:     "http://example.com",
			headers: "Content-Type: application/json\nAccept: text/plain",
			body:    "test body",
		},
		{
			name:    "empty method",
			method:  "",
			url:     "http://example.com",
			wantErr: errors.New("method is required"),
		},
		{
			name:    "invalid method",
			method:  "INVALID",
			url:     "http://example.com",
			wantErr: errors.New("method INVALID not allowed"),
		},
		{
			name:    "empty url",
			method:  http.MethodGet,
			url:     "",
			wantErr: errors.New("url is required"),
		},
		{
			name:    "url too long",
			method:  http.MethodGet,
			url:     "http://" + string(make([]byte, 1024)),
			wantErr: errors.New("url is too long (max: 1024)"),
		},
		{
			name:    "body too long",
			method:  http.MethodGet,
			url:     "http://example.com",
			body:    string(make([]byte, 1025)),
			wantErr: errors.New("body is too long (max: 1024)"),
		},
		{
			name:    "invalid url",
			method:  http.MethodGet,
			url:     "not-a-url",
			wantErr: errors.New("url is invalid"),
		},
		{
			name:    "invalid header format",
			method:  http.MethodGet,
			url:     "http://example.com",
			headers: "Invalid-Header",
			wantErr: errors.New(`invalid header format, want "name: value", got: "Invalid-Header"`),
		},
		{
			name:    "empty header name",
			method:  http.MethodGet,
			url:     "http://example.com",
			headers: ": value",
			wantErr: errors.New(`missing header name on line: ": value"`),
		},
		{
			name:    "empty header value",
			method:  http.MethodGet,
			url:     "http://example.com",
			headers: "Name:",
			wantErr: errors.New(`missing header value on line: "Name:"`),
		},
		{
			name:    "header name too long",
			method:  http.MethodGet,
			url:     "http://example.com",
			headers: string(make([]byte, 101)) + ": value",
			wantErr: errors.New("header name is too long for \"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00...\" (max 100)"),
		},
		{
			name:    "header value too long",
			method:  http.MethodGet,
			url:     "http://example.com",
			headers: "Name: " + string(make([]byte, 201)),
			wantErr: errors.New(`header value is too long for "Name" (max 200)`),
		},
		{
			name:   "too many headers",
			method: http.MethodGet,
			url:    "http://example.com",
			headers: `
X-Header-1: value
X-Header-2: value
X-Header-3: value
X-Header-4: value
X-Header-5: value
X-Header-6: value
X-Header-7: value
X-Header-8: value
X-Header-9: value
X-Header-10: value
X-Header-11: value`,
			wantErr: errors.New("too many headers (max 10)"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			req, err := experiment.MakeHTTPRequest(test.method, test.url, test.headers, test.body)
			if test.wantErr != nil {
				require.EqualError(t, err, test.wantErr.Error())
			} else {
				require.NoError(t, err)
			}

			if err == nil {
				assert.Equal(t, test.method, req.Method)
				assert.Equal(t, test.url, req.URL)
				assert.Equal(t, test.body, req.Body)
			}
		})
	}
}

func TestResult_ValueAndScan(t *testing.T) {
	t.Parallel()

	original := &experiment.Result{
		Response: experiment.HTTPResponse{
			Proto:      "HTTP/1.1",
			StatusCode: 200,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
			Body:       []byte("test body"),
		},
		Logs: []traefik.Log{{Message: "test log"}},
	}

	// Test Value()
	value, err := original.Value()
	require.NoError(t, err)

	// Test Scan()
	scanned := &experiment.Result{}
	err = scanned.Scan(value)
	require.NoError(t, err)

	assert.Equal(t, original, scanned)
}

func TestHTTPRequest_ValueAndScan(t *testing.T) {
	t.Parallel()

	original := &experiment.HTTPRequest{
		Method:  http.MethodGet,
		URL:     "http://example.com",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    "test body",
	}

	// Test Value()
	value, err := original.Value()
	require.NoError(t, err)

	// Test Scan()
	scanned := &experiment.HTTPRequest{}
	err = scanned.Scan(value)
	require.NoError(t, err)

	assert.Equal(t, original, scanned)
}
