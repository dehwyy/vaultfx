package transit

import (
	"context"
	"errors"
	"fmt"

	"github.com/dehwyy/vaultfx/pkg/hashi"
)

const defaultMount = "transit"

var (
	ErrUnavailable = hashi.ErrUnavailable
	ErrEmptyKey    = errors.New("transit: key name is empty")
	ErrEmptyInput  = errors.New("transit: input is empty")
	ErrBadResponse = errors.New("transit: unexpected vault response")
)

type Requester interface {
	Write(ctx context.Context, path string, data map[string]any) (map[string]any, error)
}

type Cipher interface {
	Encrypt(ctx context.Context, plaintext, aad []byte) (string, error)
	Decrypt(ctx context.Context, ciphertext string, aad []byte) ([]byte, error)
}

type Signer interface {
	Sign(ctx context.Context, input []byte) (string, error)
	Verify(ctx context.Context, input []byte, signature string) (bool, error)
}

type Config struct {
	Mount              string
	Key                string
	HashAlgorithm      string
	SignatureAlgorithm string
}

type Client struct {
	requester Requester
	config    Config
}

var (
	_ Cipher = (*Client)(nil)
	_ Signer = (*Client)(nil)
)

func New(requester Requester, config Config) (*Client, error) {
	if requester == nil {
		return nil, errors.New("transit: requester is nil")
	}
	if config.Key == "" {
		return nil, ErrEmptyKey
	}
	if config.Mount == "" {
		config.Mount = defaultMount
	}
	return &Client{
		requester: requester,
		config:    config,
	}, nil
}

func (c *Client) path(operation string) string {
	return fmt.Sprintf("%s/%s/%s", c.config.Mount, operation, c.config.Key)
}

func (c *Client) call(ctx context.Context, operation string, body map[string]any) (map[string]any, error) {
	data, err := c.requester.Write(ctx, c.path(operation), body)
	if err != nil {
		if errors.Is(err, hashi.ErrPermissionDenied) && !errors.Is(err, ErrUnavailable) {
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		return nil, err
	}
	return data, nil
}
