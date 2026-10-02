package vault_test

import (
	"testing"

	vault "github.com/dehwyy/vaultfx/pkg"
	"github.com/dehwyy/vaultfx/pkg/hashi"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestNewHashiFxWithConfigProvidesAllInterfaces(t *testing.T) {
	var (
		provider vault.SecretsProvider
		writer   vault.SecretsWriter
		store    vault.Store
		client   *hashi.Hashi
	)
	app := fxtest.New(
		t,
		vault.NewHashiFxWithConfig(hashi.Config{
			Address: "http://127.0.0.1:1",
			Token:   hashi.StaticToken("t"),
		}),
		fx.Populate(
			&provider,
			&writer,
			&store,
			&client,
		),
	)
	app.RequireStart()
	app.RequireStop()
	require.NotNil(t, provider)
	require.NotNil(t, writer)
	require.NotNil(t, store)
	require.NotNil(t, client)
}

func TestNewHashiFxFromEnv(t *testing.T) {
	t.Setenv("KEY_VAULT_ADDRESS", "http://127.0.0.1:1")
	t.Setenv("KEY_VAULT_TOKEN", "t")

	var provider vault.SecretsProvider
	app := fxtest.New(
		t,
		vault.NewHashiFx(),
		fx.Populate(&provider),
	)
	app.RequireStart()
	app.RequireStop()
	require.NotNil(t, provider)
}

func TestNewHashiFxWithConfigFailsOnInvalidConfig(t *testing.T) {
	app := fx.New(
		vault.NewHashiFxWithConfig(hashi.Config{}),
		fx.Invoke(func(vault.SecretsProvider) {}),
	)
	require.ErrorIs(t, app.Err(), hashi.ErrEmptyAddress)
}
