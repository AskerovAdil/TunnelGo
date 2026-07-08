package client

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, c *Client) error
	GetByID(ctx context.Context, id uuid.UUID) (*Client, error)
	GetByPublicKey(ctx context.Context, publicKey string) (*Client, error)
	List(ctx context.Context, includeRevoked bool) ([]*Client, error)
	Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error
}
