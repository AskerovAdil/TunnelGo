package client

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tunnelgo/tunnelgo/internal/domain"
)

const (
	NameMinLen = 1
	NameMaxLen = 64
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type Client struct {
	ID         uuid.UUID
	Name       string
	PrivateKey string
	PublicKey  string
	Address    string
	CreatedAt  time.Time
	RevokedAt  *time.Time
}

func (c *Client) IsRevoked() bool {
	return c.RevokedAt != nil
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) < NameMinLen || len(name) > NameMaxLen {
		return fmt.Errorf("%w: name length must be %d..%d", domain.ErrInvalidInput, NameMinLen, NameMaxLen)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%w: name has invalid characters", domain.ErrInvalidInput)
	}
	return nil
}

func ValidateAddress(address string) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return fmt.Errorf("%w: address is required", domain.ErrInvalidInput)
	}
	return nil
}
