# Changelog

## v0.1.0 (proposed)

Backward compatible: `vault.SecretsProvider`, `vault.NewHashiFx`, `vault.NewHashi`, `hashi.New`, `hashi.Opts`, `Hashi.Get`, `Hashi.MustGet` keep their signatures and environment behaviour.

### Added

- `hashi.Config`, `hashi.NewWithConfig`, `hashi.ConfigFromEnv`; `vault.NewHashiFxWithConfig`, `vault.NewHashiWithConfig` (errors instead of panics).
- `hashi.TokenSource` with `StaticToken` and `FileToken(path, ttl)`; token re-read by TTL and once more on HTTP 403 when the file changed.
- Env fallback `KEY_VAULT_TOKEN_FILE` / `VAULT_TOKEN_FILE` when `KEY_VAULT_TOKEN` is empty.
- KV write and read for v1 and v2: `Hashi.Put`, `Hashi.Read`, `hashi.SecretRef`; interfaces `vault.SecretsWriter` and `vault.Store`. `SecretsProvider` is intentionally not extended, so existing mocks keep compiling.
- `Hashi.Write` raw logical write.
- Error classification: `ErrNotFound`, `ErrPermissionDenied`, `ErrUnavailable`, original error preserved.
- `pkg/transit`: `Cipher` (Encrypt, Decrypt with AAD), `Signer` (Sign, Verify), `AAD`, `FxModule`.
- `NewHashiFx` additionally provides `*hashi.Hashi`, `SecretsWriter` and `Store`.

### Changed

- Every call (including `Get`) now goes through the token source, so `Get` benefits from file token rotation.
- `Get` errors are wrapped with the sentinel classification; `errors.As(err, *vaultapi.ResponseError)` still works.
- `go.mod`: `github.com/stretchr/testify` added (tests only).

### Unchanged on purpose

- Environment path keeps `InsecureSkipVerify: true` and the 10s HTTP timeout. Config path defaults to verified TLS.
