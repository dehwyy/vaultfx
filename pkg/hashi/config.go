package hashi

import (
	"net/http"
	"os"
	"strings"
	"time"

	vaultclient "github.com/hashicorp/vault-client-go"
	"go.uber.org/fx"
)

const (
	envKeyVaultAddress   = "KEY_VAULT_ADDRESS"
	envKeyVaultToken     = "KEY_VAULT_TOKEN"
	envKeyVaultTokenFile = "KEY_VAULT_TOKEN_FILE"
	envVaultTokenFile    = "VAULT_TOKEN_FILE"

	defaultTimeout = 10 * time.Second
)

type Opts struct {
	fx.In
}

type Config struct {
	Address            string
	Token              TokenSource
	Timeout            time.Duration
	MaxRetries         int
	InsecureSkipVerify bool
	HTTPClient         *http.Client
}

func ConfigFromEnv() (Config, error) {
	address := os.Getenv(envKeyVaultAddress)
	if address == "" {
		return Config{}, envNotSet(envKeyVaultAddress)
	}

	token, err := tokenFromEnv()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Address:            address,
		Token:              token,
		InsecureSkipVerify: true,
	}, nil
}

func tokenFromEnv() (TokenSource, error) {
	if token := os.Getenv(envKeyVaultToken); token != "" {
		return StaticToken(token), nil
	}
	for _, name := range []string{envKeyVaultTokenFile, envVaultTokenFile} {
		if path := strings.TrimSpace(os.Getenv(name)); path != "" {
			return FileToken(path, 0), nil
		}
	}
	return nil, envNotSet(envKeyVaultToken)
}

func (c Config) clientOptions() []vaultclient.ClientOption {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	retry := vaultclient.DefaultConfiguration().RetryConfiguration
	switch {
	case c.MaxRetries > 0:
		retry.RetryMax = c.MaxRetries
	case c.MaxRetries < 0:
		retry.RetryMax = 0
	}

	options := []vaultclient.ClientOption{
		vaultclient.WithAddress(c.Address),
		vaultclient.WithRequestTimeout(timeout),
		vaultclient.WithRetryConfiguration(retry),
	}
	if c.HTTPClient != nil {
		return append(options, vaultclient.WithHTTPClient(c.HTTPClient))
	}
	return append(options, vaultclient.WithTLS(vaultclient.TLSConfiguration{
		InsecureSkipVerify: c.InsecureSkipVerify,
	}))
}
