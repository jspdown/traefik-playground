package experiment

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/jspdown/traefik-playground/db/migrations"
	"github.com/jspdown/traefik-playground/internal/traefik"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestStore_Save(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	s := NewStore(db)

	// Prepare test data.
	experiment := Experiment{
		DynamicConfig: "dynamicConfig",
		Request: HTTPRequest{
			Method:  http.MethodPost,
			URL:     "https://example.com/foo",
			Headers: http.Header{"Content-Type": []string{"application/json"}},
			Body:    "body",
		},
	}
	result := Result{
		Response: HTTPResponse{
			Proto:      "HTTP/1.1",
			StatusCode: http.StatusOK,
			Headers:    http.Header{"X-Key": []string{"value"}},
			Body:       []byte("value"),
		},
		Logs: []traefik.Log{
			{
				Timestamp: time.Now().String(),
				Level:     traefik.LogLevelInfo,
				Message:   "message",
				Error:     "error",
				Fields:    map[string]interface{}{"key": "value"},
			},
		},
	}
	ctx := context.Background()

	// Save the experiment for the first time.
	firstPublicID, err := s.Save(ctx, experiment, result, "127.0.0.1")
	require.NoError(t, err)
	assert.NotEmpty(t, firstPublicID)

	// Make sure it doesn't save a new entry of the content is similar.
	secondPublicID, err := s.Save(ctx, experiment, result, "127.0.0.2")
	require.NoError(t, err)
	assert.Equal(t, firstPublicID, secondPublicID)

	gotExp, gotRes, err := s.Get(ctx, firstPublicID)
	require.NoError(t, err)
	//nolint:testifylint // False positive.
	assert.Equal(t, experiment, gotExp)
	assert.Equal(t, result, gotRes)
}

// setupTestDB initializes a PostgreSQL test database inside a container.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	pgContainer, err := postgres.Run(context.Background(), "postgres:16",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),

		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatalf("failed to start PostgreSQL container: %v", err)
	}

	t.Cleanup(func() {
		require.NoError(t, pgContainer.Terminate(context.Background()))
	})

	dsn, err := pgContainer.ConnectionString(context.Background(), "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get database connection string: %v", err)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	require.NoError(t, migrations.Migrate(db))

	return db
}
