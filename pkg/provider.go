package vault

import "context"

type SecretsProvider interface {
	Get(ctx context.Context, key string) (any, error)
	MustGet(ctx context.Context, key string) any
}
