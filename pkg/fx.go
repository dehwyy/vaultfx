package vault

import (
	"github.com/dehwyy/vaultfx/pkg/hashi"
	"go.uber.org/fx"
)

func NewHashiFx() fx.Option {
	return fx.Provide(func(opts hashi.Opts) SecretsProvider {
		return hashi.New(opts)
	})
}

func NewHashi(opts hashi.Opts) SecretsProvider {
	return hashi.New(opts)
}
