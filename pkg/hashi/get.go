package hashi

import (
	"context"
	"strings"

	vaultclient "github.com/hashicorp/vault-client-go"
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
		data, err := h.do(ctx, func(token vaultclient.RequestOption) (*vaultclient.Response[map[string]any], error) {
			return h.vault.Read(ctx, keyParts[0]+"/"+keyParts[1], token)
		})
		if err != nil {
			return "", err
		}
		return data[keyParts[2]], nil
	}

	return "", ErrUnsupportedVaultType
}
