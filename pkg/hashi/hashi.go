package hashi

import (
	"crypto/tls"
	"net/http"
	"os"
	"time"

	hashi "github.com/hashicorp/vault/api"
	"go.uber.org/fx"
)

const (
	envKeyVaultAddress = "KEY_VAULT_ADDRESS"
	envKeyVaultToken   = "KEY_VAULT_TOKEN"
)

type Opts struct {
	fx.In
}

type Hashi struct {
	vault *hashi.Client
}

func New(opts Opts) *Hashi {
	envAddress := os.Getenv(envKeyVaultAddress)
	if envAddress == "" {
		panic("variable not set: " + envKeyVaultAddress)
	}

	envToken := os.Getenv(envKeyVaultToken)
	if envToken == "" {
		panic("variable not set: " + envKeyVaultToken)
	}

	config := hashi.DefaultConfig()
	config.HttpClient = &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	config.Address = envAddress

	client, err := hashi.NewClient(config)
	if err != nil {
		panic(err)
	}
	client.SetToken(envToken)

	return &Hashi{
		vault: client,
	}
}
