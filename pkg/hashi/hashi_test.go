package hashi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func kvSecretHandler(call int, req recordedRequest, w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"password": "s3cret"}})
}

func TestGet(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		want    any
		wantErr error
		calls   int
	}{
		{name: "existing field", key: "kv.db.password", want: "s3cret", calls: 1},
		{name: "missing field is nil", key: "kv.db.missing", want: nil, calls: 1},
		{name: "dotted field", key: "kv.db.a.b", want: nil, calls: 1},
		{name: "short key", key: "kv.db", wantErr: ErrInvalidKeyFormat},
		{name: "unsupported type", key: "transit.db.x", wantErr: ErrUnsupportedVaultType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeVault(t, kvSecretHandler)
			client := newClient(t, fake, StaticToken("tok"))

			got, err := client.Get(context.Background(), tt.key)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)

			requests := fake.requests()
			require.Len(t, requests, tt.calls)
			require.Equal(t, "/v1/kv/db", requests[0].Path)
			require.Equal(t, "tok", requests[0].Token)
		})
	}
}

func TestGetClassifiesErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr error
	}{
		{name: "not found", status: http.StatusNotFound, wantErr: ErrNotFound},
		{name: "forbidden", status: http.StatusForbidden, wantErr: ErrPermissionDenied},
		{name: "server error", status: http.StatusInternalServerError, wantErr: ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeVault(t, func(call int, req recordedRequest, w http.ResponseWriter) {
				writeJSON(w, tt.status, map[string]any{"errors": []string{"x"}})
			})
			client := newClient(t, fake, StaticToken("tok"))

			_, err := client.Get(context.Background(), "kv.db.password")
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestMustGetPanics(t *testing.T) {
	fake := newFakeVault(t, kvSecretHandler)
	client := newClient(t, fake, StaticToken("tok"))

	require.Equal(t, "s3cret", client.MustGet(context.Background(), "kv.db.password"))
	require.Panics(t, func() {
		client.MustGet(context.Background(), "bad")
	})
}

func TestTokenRotationOnForbidden(t *testing.T) {
	tests := []struct {
		name         string
		rotateTo     string
		wantErr      error
		wantRequests int
		wantLastTok  string
	}{
		{name: "rotated token is retried", rotateTo: "tok-new", wantRequests: 2, wantLastTok: "tok-new"},
		{name: "unchanged token is not retried", rotateTo: "tok-old", wantErr: ErrPermissionDenied, wantRequests: 1, wantLastTok: "tok-old"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "token")
			require.NoError(t, os.WriteFile(path, []byte("tok-old"), 0o600))

			fake := newFakeVault(t, func(call int, req recordedRequest, w http.ResponseWriter) {
				if req.Token == "tok-new" {
					kvSecretHandler(call, req, w)
					return
				}
				writeJSON(w, http.StatusForbidden, map[string]any{"errors": []string{"permission denied"}})
			})
			client := newClient(t, fake, FileToken(path, 0))

			_, err := client.Get(context.Background(), "kv.db.password")
			require.ErrorIs(t, err, ErrPermissionDenied)

			require.NoError(t, os.WriteFile(path, []byte(tt.rotateTo), 0o600))
			got, err := client.Get(context.Background(), "kv.db.password")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, "s3cret", got)
			}

			requests := fake.requests()
			require.Len(t, requests, 1+tt.wantRequests)
			require.Equal(t, tt.wantLastTok, requests[len(requests)-1].Token)
		})
	}
}

func TestNewWithConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{name: "no address", config: Config{Token: StaticToken("t")}, wantErr: ErrEmptyAddress},
		{name: "no token", config: Config{Address: "http://localhost:8200"}, wantErr: ErrEmptyToken},
		{name: "ok", config: Config{Address: "http://localhost:8200", Token: StaticToken("t")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewWithConfig(tt.config)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}
}

func TestConfigFromEnv(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("file-tok"), 0o600))

	tests := []struct {
		name      string
		env       map[string]string
		wantErr   string
		wantToken string
	}{
		{name: "no address", env: map[string]string{}, wantErr: "variable not set: KEY_VAULT_ADDRESS"},
		{name: "no token", env: map[string]string{"KEY_VAULT_ADDRESS": "http://v"}, wantErr: "variable not set: KEY_VAULT_TOKEN"},
		{name: "static token", env: map[string]string{"KEY_VAULT_ADDRESS": "http://v", "KEY_VAULT_TOKEN": "env-tok"}, wantToken: "env-tok"},
		{name: "static wins over file", env: map[string]string{"KEY_VAULT_ADDRESS": "http://v", "KEY_VAULT_TOKEN": "env-tok", "KEY_VAULT_TOKEN_FILE": tokenFile}, wantToken: "env-tok"},
		{name: "KEY_VAULT_TOKEN_FILE", env: map[string]string{"KEY_VAULT_ADDRESS": "http://v", "KEY_VAULT_TOKEN_FILE": tokenFile}, wantToken: "file-tok"},
		{name: "VAULT_TOKEN_FILE", env: map[string]string{"KEY_VAULT_ADDRESS": "http://v", "VAULT_TOKEN_FILE": tokenFile}, wantToken: "file-tok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range []string{"KEY_VAULT_ADDRESS", "KEY_VAULT_TOKEN", "KEY_VAULT_TOKEN_FILE", "VAULT_TOKEN_FILE"} {
				t.Setenv(name, "")
			}
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			config, err := ConfigFromEnv()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				require.PanicsWithError(t, tt.wantErr, func() {
					New(Opts{})
				})
				return
			}
			require.NoError(t, err)
			require.Equal(t, "http://v", config.Address)
			require.True(t, config.InsecureSkipVerify)
			token, err := config.Token.Token(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.wantToken, token)
		})
	}
}
