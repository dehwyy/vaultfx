package hashi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
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

func TestPutAndRead(t *testing.T) {
	tests := []struct {
		name     string
		ref      SecretRef
		wantPath string
		wantBody map[string]any
	}{
		{
			name:     "v1 default",
			ref:      SecretRef{Mount: "kv", Path: "a/b"},
			wantPath: "/v1/kv/a/b",
			wantBody: map[string]any{"k": "v"},
		},
		{
			name:     "v1 explicit",
			ref:      SecretRef{Mount: "kv", Path: "a/b", Version: KVv1},
			wantPath: "/v1/kv/a/b",
			wantBody: map[string]any{"k": "v"},
		},
		{
			name:     "v2 wraps data",
			ref:      SecretRef{Mount: "kv-audit", Path: "daily/2026-10-02", Version: KVv2},
			wantPath: "/v1/kv-audit/data/daily/2026-10-02",
			wantBody: map[string]any{"data": map[string]any{"k": "v"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeVault(t, func(call int, req recordedRequest, w http.ResponseWriter) {
				if req.Method == http.MethodGet {
					data := map[string]any{"k": "v"}
					if tt.ref.Version == KVv2 {
						writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"data": data, "metadata": map[string]any{"version": 1}}})
						return
					}
					writeJSON(w, http.StatusOK, map[string]any{"data": data})
					return
				}
				if tt.ref.Version == KVv2 {
					writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"version": 1}})
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})
			client := newClient(t, fake, StaticToken("tok"))

			require.NoError(t, client.Put(context.Background(), tt.ref, map[string]any{"k": "v"}))
			got, err := client.Read(context.Background(), tt.ref)
			require.NoError(t, err)
			require.Equal(t, map[string]any{"k": "v"}, got)

			requests := fake.requests()
			require.Len(t, requests, 2)
			require.Equal(t, http.MethodPost, requests[0].Method)
			require.Equal(t, tt.wantPath, requests[0].Path)
			require.Equal(t, tt.wantBody, requests[0].Body)
			require.Equal(t, "tok", requests[0].Token)
		})
	}
}

func TestPutAndReadErrors(t *testing.T) {
	tests := []struct {
		name    string
		ref     SecretRef
		status  int
		wantErr error
		noCall  bool
	}{
		{name: "missing mount", ref: SecretRef{Path: "a"}, noCall: true},
		{name: "missing path", ref: SecretRef{Mount: "kv"}, noCall: true},
		{name: "bad version", ref: SecretRef{Mount: "kv", Path: "a", Version: 3}, noCall: true},
		{name: "denied", ref: SecretRef{Mount: "kv", Path: "a"}, status: http.StatusForbidden, wantErr: ErrPermissionDenied},
		{name: "not found", ref: SecretRef{Mount: "kv", Path: "a"}, status: http.StatusNotFound, wantErr: ErrNotFound},
		{name: "unavailable", ref: SecretRef{Mount: "kv", Path: "a", Version: KVv2}, status: http.StatusServiceUnavailable, wantErr: ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeVault(t, func(call int, req recordedRequest, w http.ResponseWriter) {
				writeJSON(w, tt.status, map[string]any{"errors": []string{"x"}})
			})
			client := newClient(t, fake, StaticToken("tok"))

			putErr := client.Put(context.Background(), tt.ref, map[string]any{"k": "v"})
			_, readErr := client.Read(context.Background(), tt.ref)
			require.Error(t, putErr)
			require.Error(t, readErr)
			if tt.noCall {
				require.Empty(t, fake.requests())
				return
			}
			require.ErrorIs(t, putErr, tt.wantErr)
			require.ErrorIs(t, readErr, tt.wantErr)
		})
	}
}

func TestPutUsesRotatedToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte("tok-old"), 0o600))
	fake := newFakeVault(t, func(call int, req recordedRequest, w http.ResponseWriter) {
		if req.Token != "tok-new" {
			writeJSON(w, http.StatusForbidden, map[string]any{"errors": []string{"permission denied"}})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	client := newClient(t, fake, FileToken(path, 0))

	err := client.Put(context.Background(), SecretRef{Mount: "kv", Path: "a"}, map[string]any{"k": "v"})
	require.ErrorIs(t, err, ErrPermissionDenied)

	require.NoError(t, os.WriteFile(path, []byte("tok-new"), 0o600))

	err = client.Put(context.Background(), SecretRef{Mount: "kv", Path: "a"}, map[string]any{"k": "v"})
	require.NoError(t, err)

	requests := fake.requests()
	require.Len(t, requests, 3)
	require.Equal(t, "tok-old", requests[0].Token)
	require.Equal(t, "tok-old", requests[1].Token)
	require.Equal(t, "tok-new", requests[2].Token)
}

func TestTokenIsPerRequest(t *testing.T) {
	fake := newFakeVault(t, kvSecretHandler)
	first := newClient(t, fake, StaticToken("tok-a"))
	second := newClient(t, fake, StaticToken("tok-b"))

	var group sync.WaitGroup
	for range 20 {
		group.Add(2)
		go func() {
			defer group.Done()
			_, err := first.Get(context.Background(), "kv.db.password")
			require.NoError(t, err)
		}()
		go func() {
			defer group.Done()
			_, err := second.Get(context.Background(), "kv.db.password")
			require.NoError(t, err)
		}()
	}
	group.Wait()

	tokens := map[string]int{}
	for _, request := range fake.requests() {
		tokens[request.Token]++
	}
	require.Equal(t, map[string]int{"tok-a": 20, "tok-b": 20}, tokens)
}

func TestNetworkFailureIsUnavailable(t *testing.T) {
	fake := newFakeVault(t, kvSecretHandler)
	client := newClient(t, fake, StaticToken("tok"))
	fake.server.Close()

	_, err := client.Get(context.Background(), "kv.db.password")
	require.ErrorIs(t, err, ErrUnavailable)

	_, err = client.Write(context.Background(), "transit/encrypt/k", map[string]any{"a": "b"})
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestWriteNoContentReturnsEmptyData(t *testing.T) {
	fake := newFakeVault(t, func(call int, req recordedRequest, w http.ResponseWriter) {
		w.WriteHeader(http.StatusNoContent)
	})
	client := newClient(t, fake, StaticToken("tok"))

	got, err := client.Write(context.Background(), "sys/x", map[string]any{"a": "b"})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}

func TestSecretRefSentinels(t *testing.T) {
	tests := []struct {
		name    string
		ref     SecretRef
		wantErr error
	}{
		{name: "missing mount", ref: SecretRef{Path: "a"}, wantErr: ErrInvalidSecretRef},
		{name: "missing path", ref: SecretRef{Mount: "kv"}, wantErr: ErrInvalidSecretRef},
		{name: "bad version", ref: SecretRef{Mount: "kv", Path: "a", Version: 3}, wantErr: ErrUnsupportedKVVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeVault(t, kvSecretHandler)
			client := newClient(t, fake, StaticToken("tok"))

			require.ErrorIs(t, client.Put(context.Background(), tt.ref, nil), tt.wantErr)
			_, err := client.Read(context.Background(), tt.ref)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}

	fake := newFakeVault(t, kvSecretHandler)
	client := newClient(t, fake, StaticToken("tok"))
	_, err := client.Write(context.Background(), "", nil)
	require.ErrorIs(t, err, ErrEmptyPath)
}

func TestWrite(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		status  int
		want    map[string]any
		wantErr error
		noCall  bool
	}{
		{name: "returns data", path: "transit/encrypt/k", status: http.StatusOK, want: map[string]any{"ciphertext": "c"}},
		{name: "empty path", path: "", noCall: true},
		{name: "denied", path: "transit/encrypt/k", status: http.StatusForbidden, wantErr: ErrPermissionDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeVault(t, func(call int, req recordedRequest, w http.ResponseWriter) {
				writeJSON(w, tt.status, map[string]any{"data": map[string]any{"ciphertext": "c"}})
			})
			client := newClient(t, fake, StaticToken("tok"))

			got, err := client.Write(context.Background(), tt.path, map[string]any{"a": "b"})
			if tt.noCall {
				require.Error(t, err)
				require.Empty(t, fake.requests())
				return
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, "/v1/transit/encrypt/k", fake.requests()[0].Path)
			require.Equal(t, map[string]any{"a": "b"}, fake.requests()[0].Body)
		})
	}
}
