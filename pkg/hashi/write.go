package hashi

import (
	"context"
	"errors"
	"fmt"

	hashi "github.com/hashicorp/vault/api"
)

func (h *Hashi) Write(ctx context.Context, path string, data map[string]any) (map[string]any, error) {
	if path == "" {
		return nil, errors.New("vault: write path is empty")
	}

	var secret *hashi.Secret
	err := h.do(ctx, func() error {
		var callErr error
		secret, callErr = h.vault.Logical().WriteWithContext(ctx, path, data)
		return callErr
	})
	if err != nil {
		return nil, fmt.Errorf("vault: write %s: %w", path, err)
	}
	if secret == nil {
		return nil, nil
	}
	return secret.Data, nil
}
