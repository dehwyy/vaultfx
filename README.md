# vaultfx

HashiCorp Vault client for Go services built on [Uber FX](https://github.com/uber-go/fx). Reads and writes KV secrets, rotates the token from a file, and wraps the Transit engine.

```bash
go get github.com/dehwyy/vaultfx
```

## Reading secrets (unchanged)

```go
fx.New(
	vault.NewHashiFx(),
	fx.Invoke(func(secrets vault.SecretsProvider) {
		password := secrets.MustGet(context.Background(), "kv.db.password")
	}),
)
```

Key format: `kv.<secret>.<field>`, KV v1 mount `kv`. `NewHashiFx` reads the environment and panics on a missing value, as before:

| Variable | Meaning |
|---|---|
| `KEY_VAULT_ADDRESS` | Vault address, required |
| `KEY_VAULT_TOKEN` | static token |
| `KEY_VAULT_TOKEN_FILE`, `VAULT_TOKEN_FILE` | token file, used when `KEY_VAULT_TOKEN` is empty |

TLS verification stays disabled on the environment path for backward compatibility. Set `Config.InsecureSkipVerify` explicitly (default `false`) when building the client from a `Config`.

## Explicit configuration

```go
client, err := hashi.NewWithConfig(hashi.Config{
	Address:    "https://vault.internal:8200",
	Token:      hashi.FileToken("/vault/secrets/audit-token", time.Minute),
	Timeout:    5 * time.Second,
	MaxRetries: -1,
})
```

Or in FX: `vault.NewHashiFxWithConfig(cfg)`. Errors are returned, never panicked.

`MaxRetries`: `0` keeps the Vault API default, a negative value disables retries.

### Token sources

| Constructor | Behaviour |
|---|---|
| `hashi.StaticToken(v)` | fixed token |
| `hashi.FileToken(path, ttl)` | re-reads the file after `ttl` (default 1 minute). If the file becomes unreadable or empty, the last good token is kept. |

On HTTP 403 the source is invalidated, the file is re-read, and the call is repeated once only if the token actually changed. This is how a role rotated by Vault Agent (for example `balance-core-audit`) is picked up without a restart.

## Writing secrets

```go
ref := vault.SecretRef{
	Mount:   "kv-audit",
	Path:    "balance-core/audit/daily/2026-10-02",
	Version: vault.KVv2,
}
err := store.Put(ctx, ref, map[string]any{"root_hash": "..."})
data, err := store.Read(ctx, ref)
```

`Version` defaults to KV v1. Interfaces: `SecretsProvider` (unchanged), `SecretsWriter` (`Put`, `Read`), `Store` (both). `NewHashiFx` provides all three plus `*hashi.Hashi`. `Hashi.Write(ctx, path, data)` is the raw logical write used by Transit.

## Errors

`errors.Is` against `hashi.ErrNotFound` (404), `hashi.ErrPermissionDenied` (403), `hashi.ErrUnavailable` (5xx, network, timeout, unreadable token file). The original Vault error stays in the chain.

## Transit

```go
fx.New(
	vault.NewHashiFxWithConfig(cfg),
	transit.FxModule(transit.Config{
		Mount: "transit",
		Key:   "provider-core",
	}),
	fx.Invoke(func(cipher transit.Cipher) {
		aad := transit.AAD(providerID, direction, name)
		ciphertext, err := cipher.Encrypt(ctx, []byte("secret"), aad)
		plaintext, err := cipher.Decrypt(ctx, ciphertext, aad)
	}),
)
```

- `Cipher`: `Encrypt(ctx, plaintext, aad)`, `Decrypt(ctx, ciphertext, aad)`. `aad` goes to Vault as `associated_data` and is omitted when empty.
- `Signer`: `Sign(ctx, input)`, `Verify(ctx, input, signature)`. `Config.HashAlgorithm` and `Config.SignatureAlgorithm` are passed through when set.
- `transit.AAD(parts...)` builds the length-prefixed form `4:prov|2:in|3:key`, identical to the provider-core format, so existing ciphertexts stay decryptable.
- `transit.ErrUnavailable` equals `hashi.ErrUnavailable`. A 403 is reported as unavailable too (and still matches `ErrPermissionDenied`), as the provider-core client did.

`Config.Mount` defaults to `transit`, `Config.Key` is required. Several transit clients with different keys: call `transit.New(requester, cfg)` directly.

## Migrating from provider-core `internal/transit`

| provider-core | vaultfx |
|---|---|
| `transit.Config{Address, TokenFile, Mount, Key, Timeout}` | `hashi.Config{Address, Token: hashi.FileToken(path, ttl), Timeout}` + `transit.Config{Mount, Key}` |
| `transit.New(cfg)` | `transit.New(hashiClient, cfg)` |
| `transit.AAD(a, b, c)` | `transit.AAD(a, b, c)` |
| `transit.ErrUnavailable` | `transit.ErrUnavailable` |
| `Cipher` interface | `transit.Cipher` |
| retries disabled | `hashi.Config{MaxRetries: -1}` |
