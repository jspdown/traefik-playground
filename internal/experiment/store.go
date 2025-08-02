package experiment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lithammer/shortuuid/v4"
)

var ErrNotFound = errors.New("not found")

// Store stores Experiments.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store.
func NewStore(db *sql.DB) *Store {
	return &Store{
		db: db,
	}
}

// Save saves the given Experiment, a unique public ID is returned.
func (s *Store) Save(ctx context.Context, exp Experiment, res Result, clientIP string) (string, error) {
	publicID := shortuuid.New()

	// Generate a hash of the experiment and result.
	// This hash is used to prevent saving multiple time the same thing.
	hashData, err := json.Marshal(struct {
		Experiment Experiment `json:"experiment"`
		Result     Result     `json:"result"`
	}{
		Experiment: exp,
		Result:     res,
	})
	if err != nil {
		return "", fmt.Errorf("marshaling hash data: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(hashData))

	query := `
		INSERT INTO shared_experiments (public_id,
		                         		hash,
		                         		dynamic_config,
		                         		request,
		                         		result,
		                         		client_ip)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(hash) DO UPDATE SET hash = shared_experiments.hash
		RETURNING public_id
	`
	err = s.db.QueryRowContext(ctx, query,
		publicID,
		hash,
		exp.DynamicConfig,
		&exp.Request,
		&res,
		clientIP,
	).Scan(&publicID)
	if err != nil {
		return "", fmt.Errorf("inserting experiment: %w", err)
	}

	return publicID, nil
}

// Get retrieves an Experiment from its public ID.
func (s *Store) Get(ctx context.Context, publicID string) (exp Experiment, res Result, err error) {
	query := `
		UPDATE shared_experiments SET last_retrieved_at = NOW()
        WHERE public_id = $1
        RETURNING dynamic_config, request, result
	`
	err = s.db.QueryRowContext(ctx, query, publicID).Scan(&exp.DynamicConfig, &exp.Request, &res)
	if errors.Is(err, sql.ErrNoRows) {
		return Experiment{}, Result{}, ErrNotFound
	} else if err != nil {
		return Experiment{}, Result{}, err
	}

	return
}
