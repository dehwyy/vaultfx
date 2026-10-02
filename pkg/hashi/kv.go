package hashi

import (
	"context"
	"fmt"

	vaultclient "github.com/hashicorp/vault-client-go"
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
		return ErrInvalidSecretRef
	}
	if r.Version != 0 && r.Version != KVv1 && r.Version != KVv2 {
		return fmt.Errorf("%w: %d", ErrUnsupportedKVVersion, r.Version)
	}
	return nil
}

func (r SecretRef) apiPath() string {
	if r.Version == KVv2 {
		return r.Mount + "/data/" + r.Path
	}
	return r.Mount + "/" + r.Path
}

func (h *Hashi) Put(ctx context.Context, ref SecretRef, data map[string]any) error {
	if err := ref.validate(); err != nil {
		return err
	}

	body := data
	if ref.Version == KVv2 {
		body = map[string]any{"data": data}
	}

	_, err := h.do(ctx, func(token vaultclient.RequestOption) (*vaultclient.Response[map[string]any], error) {
		return h.vault.Write(ctx, ref.apiPath(), body, token)
	})
	return err
}

func (h *Hashi) Read(ctx context.Context, ref SecretRef) (map[string]any, error) {
	if err := ref.validate(); err != nil {
		return nil, err
	}

	data, err := h.do(ctx, func(token vaultclient.RequestOption) (*vaultclient.Response[map[string]any], error) {
		return h.vault.Read(ctx, ref.apiPath(), token)
	})
	if err != nil {
		return nil, err
	}

	if ref.Version != KVv2 {
		return data, nil
	}

	inner, ok := data["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s has no data", ErrNotFound, ref.Mount, ref.Path)
	}
	return inner, nil
}
