package compose_test

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jspdown/traefik-playground/internal/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tccompose "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
)

type httpExpectation struct {
	method     string
	path       string
	statusCode int
	contains   []string
}

func TestGenerate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputFile    string
		outputFile   string
		wantErr      bool
		expectations []httpExpectation
	}{
		{
			name:       "references whoami@playground",
			inputFile:  "whoami-playground.dynamic.yaml",
			outputFile: "whoami-playground.expected.yaml",
			expectations: []httpExpectation{
				{
					method:     "GET",
					path:       "/foo",
					statusCode: 200,
					contains:   []string{"Hostname:", "X-Request-Header: request"},
				},
			},
		},
		{
			name:       "custom service pointing to 10.10.10.10",
			inputFile:  "custom-service.dynamic.yaml",
			outputFile: "custom-service.expected.yaml",
			expectations: []httpExpectation{
				{
					method:     "GET",
					path:       "/foo",
					statusCode: 200,
					contains:   []string{"Hostname:"},
				},
			},
		},
		{
			name:       "empty",
			inputFile:  "empty.dynamic.yaml",
			outputFile: "empty.expected.yaml",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inputPath := filepath.Join("testdata", test.inputFile)
			dynamicConfig, err := os.ReadFile(inputPath)
			require.NoError(t, err)

			result := compose.Generate(string(dynamicConfig))
			assert.NotEmpty(t, result)

			expectedPath := filepath.Join("testdata", test.outputFile)
			expectedData, err := os.ReadFile(expectedPath)
			require.NoError(t, err)

			assert.Equal(t, string(expectedData), result)

			if len(test.expectations) == 0 {
				return
			}

			if testing.Short() {
				t.Skip("Skipping integration test in short mode")

				return
			}

			runIntegrationTest(t, result, test.expectations)
		})
	}
}

func runIntegrationTest(t *testing.T, dockerComposeContent string, expectations []httpExpectation) {
	t.Helper()

	containers := startDockerCompose(t, dockerComposeContent)
	defer containers.Cleanup()

	baseURL := "http://localhost:" + containers.WebPort
	client := &http.Client{Timeout: 5 * time.Second}

	for _, expectation := range expectations {
		t.Run(fmt.Sprintf("%s %s", expectation.method, expectation.path), func(t *testing.T) {
			url := baseURL + expectation.path

			resp, err := requestWithRetry(t.Context(), client, expectation.method, url, nil, 30*time.Second, 1*time.Second)
			if err != nil {
				t.Logf("All container logs:\n%s", containers.GetAllLogs())
				require.NoError(t, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if len(expectation.contains) > 0 {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)

				bodyStr := string(body)

				for _, contains := range expectation.contains {
					assert.Contains(t, bodyStr, contains, "Response body should contain: %s", contains)
				}
			}
		})
	}
}

func requestWithRetry(ctx context.Context, client *http.Client, method, url string, body io.Reader, timeout, retryInterval time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err == nil {
			return resp, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("request timed out after %v: %w", timeout, ctx.Err())
		case <-time.After(retryInterval):
			continue
		}
	}
}

type dockerComposeSetup struct {
	WebPort        string
	ComposeStack   tccompose.ComposeStack
	TraefikService testcontainers.Container
	Cleanup        func()
}

func startDockerCompose(t *testing.T, dockerComposeContent string) *dockerComposeSetup {
	t.Helper()

	// Generate a unique project name to avoid container naming conflicts.
	projectName := fmt.Sprintf("test-%d-%d", time.Now().UnixNano(), rand.Intn(10000))

	// Replace static port bindings with dynamic bindings.
	modifiedContent := strings.ReplaceAll(dockerComposeContent, `"80:80"`, `"80"`)
	modifiedContent = strings.ReplaceAll(modifiedContent, `"8080:8080"`, `"8080"`)

	ctx := context.Background()

	stack, err := tccompose.NewDockerComposeWith(
		tccompose.WithStackReaders(strings.NewReader(modifiedContent)),
		tccompose.StackIdentifier(projectName),
	)
	require.NoError(t, err)

	err = stack.
		WaitForService("traefik", wait.ForListeningPort("80/tcp")).
		Up(ctx, tccompose.Wait(true))
	require.NoError(t, err, "Failed to start docker compose")

	traefikContainer, err := stack.ServiceContainer(ctx, "traefik")
	require.NoError(t, err, "Failed to get Traefik container")

	// Get the mapped port for port 80.
	mappedPort, err := traefikContainer.MappedPort(ctx, "80/tcp")
	require.NoError(t, err, "Failed to get mapped port")
	webPort := mappedPort.Port()

	cleanup := func() {
		_ = stack.Down(ctx, tccompose.RemoveOrphans(true), tccompose.RemoveImagesLocal)
	}

	return &dockerComposeSetup{
		WebPort:        webPort,
		ComposeStack:   stack,
		TraefikService: traefikContainer,
		Cleanup:        cleanup,
	}
}

func (d *dockerComposeSetup) GetAllLogs() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get logs from all services in the compose stack
	services := d.ComposeStack.Services()
	allLogs := strings.Builder{}

	for _, serviceName := range services {
		container, err := d.ComposeStack.ServiceContainer(ctx, serviceName)
		if err != nil {
			allLogs.WriteString(fmt.Sprintf("Failed to get %s container: %v\n", serviceName, err))

			continue
		}

		logs, err := container.Logs(ctx)
		if err != nil {
			allLogs.WriteString(fmt.Sprintf("Failed to get %s logs: %v\n", serviceName, err))

			continue
		}

		logBytes, err := io.ReadAll(logs)
		_ = logs.Close()
		if err != nil {
			allLogs.WriteString(fmt.Sprintf("Failed to read %s logs: %v\n", serviceName, err))

			continue
		}

		allLogs.WriteString(fmt.Sprintf("=== %s logs ===\n", serviceName))
		allLogs.Write(logBytes)
		allLogs.WriteString("\n")
	}

	return allLogs.String()
}
