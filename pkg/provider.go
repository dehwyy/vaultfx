package vault

import (
	"context"

	"github.com/dehwyy/vaultfx/pkg/hashi"
)

type SecretsProvider interface {
	Get(ctx context.Context, key string) (any, error)
	MustGet(ctx context.Context, key string) any
}

type (
	SecretRef = hashi.SecretRef
	KVVersion = hashi.KVVersion
)

const (
	KVv1 = hashi.KVv1
	KVv2 = hashi.KVv2
)

type SecretsWriter interface {
	Put(ctx context.Context, ref SecretRef, data map[string]any) error
	Read(ctx context.Context, ref SecretRef) (map[string]any, error)
}

type Store interface {
	SecretsProvider
	SecretsWriter
}
