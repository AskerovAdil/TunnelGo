package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tunnelgo/tunnelgo/internal/domain"
	"github.com/tunnelgo/tunnelgo/internal/domain/client"
)

type ClientRepo struct {
	db *sql.DB
}

func NewClientRepo(db *sql.DB) *ClientRepo {
	return &ClientRepo{db: db}
}

func (r *ClientRepo) Create(ctx context.Context, c *client.Client) error {
	const q = `
		INSERT INTO clients (id, name, private_key, public_key, address, created_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, q,
		c.ID.String(),
		c.Name,
		c.PrivateKey,
		c.PublicKey,
		c.Address,
		c.CreatedAt.UTC().Format(time.RFC3339Nano),
		nullableTime(c.RevokedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("insert client: %w", err)
	}

	return nil
}

func (r *ClientRepo) GetByID(ctx context.Context, id uuid.UUID) (*client.Client, error) {
	const q = `
		SELECT id, name, private_key, public_key, address, created_at, revoked_at
		FROM clients
		WHERE id = ?
	`

	row := r.db.QueryRowContext(ctx, q, id.String())
	c, err := scanClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select client by id: %w", err)
	}

	return c, nil
}

func (r *ClientRepo) GetByPublicKey(ctx context.Context, publicKey string) (*client.Client, error) {
	const q = `
		SELECT id, name, private_key, public_key, address, created_at, revoked_at
		FROM clients
		WHERE public_key = ?
	`

	row := r.db.QueryRowContext(ctx, q, publicKey)
	c, err := scanClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select client by public key: %w", err)
	}

	return c, nil
}

func (r *ClientRepo) List(ctx context.Context, includeRevoked bool) ([]*client.Client, error) {
	q := `
		SELECT id, name, private_key, public_key, address, created_at, revoked_at
		FROM clients
	`
	if !includeRevoked {
		q += ` WHERE revoked_at IS NULL`
	}
	q += ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	defer rows.Close()

	var clients []*client.Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, fmt.Errorf("scan client: %w", err)
		}
		clients = append(clients, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clients: %w", err)
	}

	return clients, nil
}

func (r *ClientRepo) Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	const q = `
		UPDATE clients
		SET revoked_at = ?
		WHERE id = ? AND revoked_at IS NULL
	`

	res, err := r.db.ExecContext(ctx, q, revokedAt.UTC().Format(time.RFC3339Nano), id.String())
	if err != nil {
		return fmt.Errorf("revoke client: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *ClientRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM clients WHERE id = ?`

	res, err := r.db.ExecContext(ctx, q, id.String())
	if err != nil {
		return fmt.Errorf("delete client: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanClient(row rowScanner) (*client.Client, error) {
	var (
		id         string
		name       string
		privateKey string
		publicKey  string
		address    string
		createdAt  string
		revokedAt  sql.NullString
	)

	if err := row.Scan(&id, &name, &privateKey, &publicKey, &address, &createdAt, &revokedAt); err != nil {
		return nil, err
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("parse client id: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	var parsedRevokedAt *time.Time
	if revokedAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, revokedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse revoked_at: %w", err)
		}
		parsedRevokedAt = &t
	}

	return &client.Client{
		ID:         parsedID,
		Name:       name,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Address:    address,
		CreatedAt:  parsedCreatedAt,
		RevokedAt:  parsedRevokedAt,
	}, nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}
