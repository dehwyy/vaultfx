package transit

import (
	"github.com/dehwyy/vaultfx/pkg/hashi"
	"go.uber.org/fx"
)

func FxModule(config Config) fx.Option {
	return fx.Provide(
		func(client *hashi.Hashi) (*Client, error) {
			return New(client, config)
		},
		func(client *Client) Cipher { return client },
		func(client *Client) Signer { return client },
	)
}
