package hashi

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	hashi "github.com/hashicorp/vault/api"
)

type Hashi struct {
	vault  *hashi.Client
	tokens TokenSource

	mu      sync.Mutex
	applied string
}

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

	apiConfig := hashi.DefaultConfig()
	apiConfig.HttpClient = config.httpClient()
	apiConfig.Address = config.Address
	switch {
	case config.MaxRetries > 0:
		apiConfig.MaxRetries = config.MaxRetries
	case config.MaxRetries < 0:
		apiConfig.MaxRetries = 0
	}

	client, err := hashi.NewClient(apiConfig)
	if err != nil {
		return nil, fmt.Errorf("vault: new client: %w", err)
	}
	client.ClearToken()

	return &Hashi{
		vault:  client,
		tokens: config.Token,
	}, nil
}

func (h *Hashi) applyToken(ctx context.Context) (string, error) {
	token, err := h.tokens.Token(ctx)
	if err != nil {
		return "", err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if token != h.applied {
		h.vault.SetToken(token)
		h.applied = token
	}
	return token, nil
}

func (h *Hashi) do(ctx context.Context, call func() error) error {
	used, err := h.applyToken(ctx)
	if err != nil {
		return err
	}

	err = call()
	if statusOf(err) != http.StatusForbidden {
		return classify(err)
	}

	h.tokens.Invalidate()
	refreshed, tokenErr := h.applyToken(ctx)
	if tokenErr != nil || refreshed == used {
		return classify(err)
	}
	return classify(call())
}
