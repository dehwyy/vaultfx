package hashi

import (
	"context"
	"fmt"

	vaultclient "github.com/hashicorp/vault-client-go"
)

func (h *Hashi) Write(ctx context.Context, path string, data map[string]any) (map[string]any, error) {
	if path == "" {
		return nil, ErrEmptyPath
	}

	result, err := h.do(ctx, func(token vaultclient.RequestOption) (*vaultclient.Response[map[string]any], error) {
		return h.vault.Write(ctx, path, data, token)
	})
	if err != nil {
		return nil, fmt.Errorf("vault: write %s: %w", path, err)
	}
	return result, nil
}
