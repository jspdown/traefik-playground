package experiment

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	stdurl "net/url"
	"slices"
	"strings"

	"github.com/jspdown/traefik-playground/internal/header"
	"github.com/jspdown/traefik-playground/internal/traefik"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"gopkg.in/yaml.v3"
)

const (
	maxDynamicConfigLength = 10 * 1024

	maxURLLength  = 1024
	maxBodyLength = 1024

	maxHeaders           = 10
	maxHeaderNameLength  = 100
	maxHeaderValueLength = 200
)

// Experiment is an experiment to run.
type Experiment struct {
	DynamicConfig string      `json:"dynamicConfig"`
	Request       HTTPRequest `json:"request"`
}

// MakeExperiment makes a valid Experiment.
func MakeExperiment(dynamicConfig, method, url, headers, body string) (Experiment, error) {
	if len(dynamicConfig) > maxDynamicConfigLength {
		return Experiment{}, fmt.Errorf("dynamic config too long (max: %d)", maxDynamicConfigLength)
	}

	var unmarshalledDynamicConfig dynamic.Configuration
	if err := yaml.Unmarshal([]byte(dynamicConfig), &unmarshalledDynamicConfig); err != nil {
		return Experiment{}, fmt.Errorf("invalid dynamic configuration: %w", err)
	}

	req, err := MakeHTTPRequest(method, url, headers, body)
	if err != nil {
		return Experiment{}, fmt.Errorf("request: %w", err)
	}

	return Experiment{
		DynamicConfig: dynamicConfig,
		Request:       req,
	}, nil
}

// Result is the result of a ran experiment.
type Result struct {
	Response HTTPResponse  `json:"response"`
	Logs     []traefik.Log `json:"logs"`
}

// Value implements driver.Valuer interface.
func (r *Result) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// Scan implements the sql.Scanner interface.
func (r *Result) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &r)
}

// HTTPRequest is an HTTP request to send as part of the Experiment.
type HTTPRequest struct {
	Method  string      `json:"method"`
	URL     string      `json:"url"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
}

// Value implements driver.Valuer interface.
func (r *HTTPRequest) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// Scan implements the sql.Scanner interface.
func (r *HTTPRequest) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &r)
}

// MakeHTTPRequest makes a valid HTTP request.
func MakeHTTPRequest(method, url, headers, body string) (HTTPRequest, error) {
	availableMethods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	switch {
	case method == "":
		return HTTPRequest{}, errors.New("method is required")
	case !slices.Contains(availableMethods, method):
		return HTTPRequest{}, fmt.Errorf("method %s not allowed", method)
	case url == "":
		return HTTPRequest{}, errors.New("url is required")
	case len(url) > maxURLLength:
		return HTTPRequest{}, fmt.Errorf("url is too long (max: %d)", maxURLLength)
	case len(body) > maxBodyLength:
		return HTTPRequest{}, fmt.Errorf("body is too long (max: %d)", maxBodyLength)
	}

	if _, err := stdurl.ParseRequestURI(url); err != nil {
		return HTTPRequest{}, errors.New("url is invalid")
	}

	parsedHeaders, err := parseHeaders(headers)
	if err != nil {
		return HTTPRequest{}, err
	}

	return HTTPRequest{
		Method:  method,
		URL:     url,
		Headers: parsedHeaders,
		Body:    body,
	}, nil
}

// HTTPResponse is the HTTP response obtained from a ran experiment.
type HTTPResponse struct {
	Proto      string      `json:"proto"`
	StatusCode int         `json:"statusCode"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
}

func parseHeaders(rawHeaders string) (http.Header, error) {
	headerLines := strings.Split(rawHeaders, "\n")

	headers := make(http.Header)
	for _, line := range headerLines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf(`invalid header format, want "name: value", got: %q`, line)
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if name == "" {
			return nil, fmt.Errorf("missing header name on line: %q", line)
		} else if value == "" {
			return nil, fmt.Errorf("missing header value on line: %q", line)
		}

		if len(name) > maxHeaderNameLength {
			return nil, fmt.Errorf(`header name is too long for "%s..." (max %d)`, name[:10], maxHeaderNameLength)
		}
		if len(value) > maxHeaderValueLength {
			return nil, fmt.Errorf("header value is too long for %q (max %d)", name, maxHeaderValueLength)
		}

		if !header.ValidHeaderField(name) {
			return nil, fmt.Errorf("invalid header name %q", name)
		}
		if !header.ValidHeaderValue(value) {
			return nil, fmt.Errorf("invalid header value for %q", name)
		}

		if len(headers) >= maxHeaders {
			return nil, fmt.Errorf("too many headers (max %d)", maxHeaders)
		}

		headers.Set(name, value)
	}

	return headers, nil
}
