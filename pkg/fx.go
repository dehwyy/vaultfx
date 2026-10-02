package vault

import (
	"github.com/dehwyy/vaultfx/pkg/hashi"
	"go.uber.org/fx"
)

func NewHashiFx() fx.Option {
	return fx.Options(
		fx.Provide(func(opts hashi.Opts) *hashi.Hashi {
			return hashi.New(opts)
		}),
		interfaces(),
	)
}

func NewHashiFxWithConfig(config hashi.Config) fx.Option {
	return fx.Options(
		fx.Provide(func() (*hashi.Hashi, error) {
			return hashi.NewWithConfig(config)
		}),
		interfaces(),
	)
}

func interfaces() fx.Option {
	return fx.Provide(
		func(client *hashi.Hashi) SecretsProvider { return client },
		func(client *hashi.Hashi) SecretsWriter { return client },
		func(client *hashi.Hashi) Store { return client },
	)
}

func NewHashi(opts hashi.Opts) SecretsProvider {
	return hashi.New(opts)
}

func NewHashiWithConfig(config hashi.Config) (*hashi.Hashi, error) {
	return hashi.NewWithConfig(config)
}
