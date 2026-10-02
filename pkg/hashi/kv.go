package hashi

import (
	"context"
	"fmt"

	hashi "github.com/hashicorp/vault/api"
)

type KVVersion int

const (
	KVv1 KVVersion = 1
	KVv2 KVVersion = 2
)

type SecretRef struct {
	Mount   string
	Path    string
	Version KVVersion
}

func (r SecretRef) validate() error {
	if r.Mount == "" || r.Path == "" {
		return fmt.Errorf("vault: secret ref requires mount and path")
	}
	if r.Version != 0 && r.Version != KVv1 && r.Version != KVv2 {
		return fmt.Errorf("vault: unsupported kv version %d", r.Version)
	}
	return nil
}

func (h *Hashi) Put(ctx context.Context, ref SecretRef, data map[string]any) error {
	if err := ref.validate(); err != nil {
		return err
	}

	return h.do(ctx, func() error {
		if ref.Version == KVv2 {
			_, err := h.vault.KVv2(ref.Mount).Put(ctx, ref.Path, data)
			return err
		}
		return h.vault.KVv1(ref.Mount).Put(ctx, ref.Path, data)
	})
}

func (h *Hashi) Read(ctx context.Context, ref SecretRef) (map[string]any, error) {
	if err := ref.validate(); err != nil {
		return nil, err
	}

	var secret *hashi.KVSecret
	err := h.do(ctx, func() error {
		var callErr error
		if ref.Version == KVv2 {
			secret, callErr = h.vault.KVv2(ref.Mount).Get(ctx, ref.Path)
			return callErr
		}
		secret, callErr = h.vault.KVv1(ref.Mount).Get(ctx, ref.Path)
		return callErr
	})
	if err != nil {
		return nil, err
	}
	return secret.Data, nil
}
