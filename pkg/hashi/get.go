package hashi

import (
	"context"
	"strings"

	hashi "github.com/hashicorp/vault/api"
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
		var value *hashi.KVSecret
		err := h.do(ctx, func() error {
			var callErr error
			value, callErr = h.vault.KVv1(keyParts[0]).Get(ctx, keyParts[1])
			return callErr
		})
		if err != nil {
			return "", err
		}

		return value.Data[keyParts[2]], nil
	}

	return "", ErrUnsupportedVaultType
}
