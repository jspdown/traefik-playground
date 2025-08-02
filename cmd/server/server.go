package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jspdown/traefik-playground/app"
	"github.com/jspdown/traefik-playground/db/migrations"
	"github.com/jspdown/traefik-playground/internal/command"
	"github.com/jspdown/traefik-playground/internal/experiment"
	"github.com/rs/zerolog/log"
)

// Config holds the Server configuration.
type Config struct {
	Addr               string
	DatabaseConnString string

	// SecretKey is the key used to sign experiment responses.
	SecretKey string

	// TesterTimeout defines how long an experiment is allowed to run.
	TesterTimeout time.Duration

	// MaxPendingCommands defines the size of the spawner command queue.
	MaxPendingCommands int
	// MaxProcesses defines the number of simultaneous processes executing spawner commands.
	MaxProcesses int
}

// Server serves the traefik-playground service.
type Server struct {
	config Config
}

// New creates a new Server.
func New(config Config) (*Server, error) {
	if config.MaxPendingCommands < config.MaxProcesses {
		return nil, errors.New("max-pending-commands must be greater or equal to max-processes")
	}
	if config.TesterTimeout < time.Second {
		return nil, errors.New("tester-timeout must be at least 1s")
	}

	return &Server{
		config: config,
	}, nil
}

// Start starts the server.
func (s *Server) Start(ctx context.Context) error {
	// Initialize the database.
	db, err := sql.Open("postgres", s.config.DatabaseConnString)
	if err != nil {
		return fmt.Errorf("opening database connection: %w", err)
	}

	defer func() { _ = db.Close() }()

	if err = migrations.Migrate(db); err != nil {
		return fmt.Errorf("migrating database: %w", err)
	}

	// Initialize handlers.
	store := experiment.NewStore(db)
	pool := command.NewWorkerPool(s.config.MaxProcesses, s.config.MaxPendingCommands)
	traefikRunner := experiment.NewTraefik(pool, s.config.TesterTimeout)
	controller := experiment.NewController(store, traefikRunner)

	appHandler, err := app.New(controller, s.config.SecretKey)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	appHandler.MountOn(mux)

	// Start the server.
	server := &http.Server{
		Addr:         s.config.Addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		Handler:      mux,
	}

	ctx, stopAll := context.WithCancel(ctx)
	defer stopAll()

	serverDoneCh := make(chan struct{})
	go func() {
		log.Info().Msgf("Starting server on %s...", s.config.Addr)
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			log.Error().Err(listenErr).Msg("Failed to start server")
		}

		close(serverDoneCh)
	}()

	// Handle graceful server shutdown.
	select {
	case <-ctx.Done():
		// Attempt to gracefully shut down the server.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		log.Info().Msg("Shutting down server...")

		//nolint:contextcheck // context not inherited to give enough time for the shutdown.
		if err = server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Server forced to shutdown")

			if err = server.Close(); err != nil {
				return fmt.Errorf("forcing shutdown: %w", err)
			}
		}

		log.Info().Msg("Successfully shutdown server...")
	case <-serverDoneCh:
		return errors.New("server stopped")
	}

	return nil
}

func healthHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK)
}
