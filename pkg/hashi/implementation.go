package hashi

import (
	"context"
	"strings"
)

func (h *Hashi) MustGet(ctx context.Context, key string) any {
	value, err := h.Get(ctx, key)
	if err != nil {
		panic(err)
	}
	return value
}

func (h *Hashi) Get(ctx context.Context, key string) (any, error) {
	keyParts := strings.SplitN(key, ".", 3)
	if len(keyParts) < 3 {
		return "", ErrInvalidKeyFormat
	}

	switch keyParts[0] {
	case "kv":
		value, err := h.vault.KVv1(keyParts[0]).Get(ctx, keyParts[1])
		if err != nil {
			return "", err
		}

		return value.Data[keyParts[2]], nil
	}

	return "", ErrUnsupportedVaultType
}
