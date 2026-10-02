package hashi

import (
	"context"
	"fmt"
	"net/http"

	vaultclient "github.com/hashicorp/vault-client-go"
)

type Hashi struct {
	vault  *vaultclient.Client
	tokens TokenSource
}

type vaultCall func(token vaultclient.RequestOption) (*vaultclient.Response[map[string]any], error)

func New(_ Opts) *Hashi {
	config, err := ConfigFromEnv()
	if err != nil {
		panic(err)
	}

	client, err := NewWithConfig(config)
	if err != nil {
		panic(err)
	}
	return client
}

func NewWithConfig(config Config) (*Hashi, error) {
	if config.Address == "" {
		return nil, ErrEmptyAddress
	}
	if config.Token == nil {
		return nil, ErrEmptyToken
	}

	client, err := vaultclient.New(config.clientOptions()...)
	if err != nil {
		return nil, fmt.Errorf("vault: new client: %w", err)
	}

	return &Hashi{
		vault:  client,
		tokens: config.Token,
	}, nil
}

func (h *Hashi) do(ctx context.Context, call vaultCall) (map[string]any, error) {
	used, err := h.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}

	data, err := call(vaultclient.WithToken(used))
	if !vaultclient.IsErrorStatus(err, http.StatusForbidden) {
		return finish(data, err)
	}

	h.tokens.Invalidate()
	refreshed, tokenErr := h.tokens.Token(ctx)
	if tokenErr != nil || refreshed == used {
		return nil, classify(err)
	}

	return finish(call(vaultclient.WithToken(refreshed)))
}

func finish(response *vaultclient.Response[map[string]any], err error) (map[string]any, error) {
	if err != nil {
		return nil, classify(err)
	}
	if response == nil || response.Data == nil {
		return map[string]any{}, nil
	}
	return response.Data, nil
}
