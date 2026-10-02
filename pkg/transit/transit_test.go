package transit_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	vault "github.com/dehwyy/vaultfx/pkg"
	"github.com/dehwyy/vaultfx/pkg/hashi"
	"github.com/dehwyy/vaultfx/pkg/transit"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

type captured struct {
	Path  string
	Token string
	Body  map[string]any
}

type fakeTransit struct {
	mu       sync.Mutex
	requests []captured
	status   int
	response map[string]any
}

func (f *fakeTransit) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		f.mu.Lock()
		f.requests = append(f.requests, captured{
			Path:  r.URL.Path,
			Token: r.Header.Get("X-Vault-Token"),
			Body:  body,
		})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		status := f.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": f.response}))
	}
}

func newTransit(t *testing.T, fake *fakeTransit, config transit.Config) *transit.Client {
	t.Helper()
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)

	client, err := hashi.NewWithConfig(hashi.Config{
		Address:    server.URL,
		Token:      hashi.StaticToken("tok"),
		MaxRetries: -1,
	})
	require.NoError(t, err)

	cipher, err := transit.New(client, config)
	require.NoError(t, err)
	return cipher
}

func b64(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

func TestEncrypt(t *testing.T) {
	tests := []struct {
		name     string
		config   transit.Config
		plain    []byte
		aad      []byte
		response map[string]any
		status   int
		wantBody map[string]any
		wantPath string
		want     string
		wantErr  error
		noCall   bool
	}{
		{
			name:     "with aad default mount",
			config:   transit.Config{Key: "k"},
			plain:    []byte("secret"),
			aad:      []byte("ctx"),
			response: map[string]any{"ciphertext": "vault:v1:abc"},
			wantBody: map[string]any{"plaintext": b64("secret"), "associated_data": b64("ctx")},
			wantPath: "/v1/transit/encrypt/k",
			want:     "vault:v1:abc",
		},
		{
			name:     "no aad custom mount",
			config:   transit.Config{Mount: "tr2", Key: "k"},
			plain:    []byte("secret"),
			response: map[string]any{"ciphertext": "vault:v1:abc"},
			wantBody: map[string]any{"plaintext": b64("secret")},
			wantPath: "/v1/tr2/encrypt/k",
			want:     "vault:v1:abc",
		},
		{name: "empty plaintext", config: transit.Config{Key: "k"}, wantErr: transit.ErrEmptyInput, noCall: true},
		{
			name:     "response without ciphertext",
			config:   transit.Config{Key: "k"},
			plain:    []byte("x"),
			response: map[string]any{},
			wantErr:  transit.ErrBadResponse,
		},
		{
			name:     "forbidden is unavailable",
			config:   transit.Config{Key: "k"},
			plain:    []byte("x"),
			status:   http.StatusForbidden,
			response: map[string]any{},
			wantErr:  transit.ErrUnavailable,
		},
		{
			name:     "server error is unavailable",
			config:   transit.Config{Key: "k"},
			plain:    []byte("x"),
			status:   http.StatusBadGateway,
			response: map[string]any{},
			wantErr:  transit.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeTransit{status: tt.status, response: tt.response}
			client := newTransit(t, fake, tt.config)

			got, err := client.Encrypt(context.Background(), tt.plain, tt.aad)
			if tt.noCall {
				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, fake.requests)
				return
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Len(t, fake.requests, 1)
			require.Equal(t, tt.wantPath, fake.requests[0].Path)
			require.Equal(t, "tok", fake.requests[0].Token)
			require.Equal(t, tt.wantBody, fake.requests[0].Body)
		})
	}
}

func TestDecrypt(t *testing.T) {
	tests := []struct {
		name       string
		ciphertext string
		aad        []byte
		response   map[string]any
		wantBody   map[string]any
		want       []byte
		wantErr    error
		noCall     bool
	}{
		{
			name:       "with aad",
			ciphertext: "vault:v1:abc",
			aad:        []byte("ctx"),
			response:   map[string]any{"plaintext": b64("secret")},
			wantBody:   map[string]any{"ciphertext": "vault:v1:abc", "associated_data": b64("ctx")},
			want:       []byte("secret"),
		},
		{
			name:       "no aad",
			ciphertext: "vault:v1:abc",
			response:   map[string]any{"plaintext": b64("secret")},
			wantBody:   map[string]any{"ciphertext": "vault:v1:abc"},
			want:       []byte("secret"),
		},
		{name: "empty ciphertext", wantErr: transit.ErrEmptyInput, noCall: true},
		{name: "no plaintext", ciphertext: "c", response: map[string]any{}, wantErr: transit.ErrBadResponse},
		{name: "plaintext not base64", ciphertext: "c", response: map[string]any{"plaintext": "!!!"}, wantErr: transit.ErrBadResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeTransit{response: tt.response}
			client := newTransit(t, fake, transit.Config{Key: "k"})

			got, err := client.Decrypt(context.Background(), tt.ciphertext, tt.aad)
			if tt.noCall {
				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, fake.requests)
				return
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, "/v1/transit/decrypt/k", fake.requests[0].Path)
			require.Equal(t, tt.wantBody, fake.requests[0].Body)
		})
	}
}

func TestSign(t *testing.T) {
	tests := []struct {
		name     string
		config   transit.Config
		input    []byte
		response map[string]any
		wantBody map[string]any
		want     string
		wantErr  error
		noCall   bool
	}{
		{
			name:     "plain",
			config:   transit.Config{Key: "sig"},
			input:    []byte("msg"),
			response: map[string]any{"signature": "vault:v1:sig"},
			wantBody: map[string]any{"input": b64("msg")},
			want:     "vault:v1:sig",
		},
		{
			name:     "with algorithms",
			config:   transit.Config{Key: "sig", HashAlgorithm: "sha2-256", SignatureAlgorithm: "pss"},
			input:    []byte("msg"),
			response: map[string]any{"signature": "vault:v1:sig"},
			wantBody: map[string]any{"input": b64("msg"), "hash_algorithm": "sha2-256", "signature_algorithm": "pss"},
			want:     "vault:v1:sig",
		},
		{name: "empty input", config: transit.Config{Key: "sig"}, wantErr: transit.ErrEmptyInput, noCall: true},
		{name: "no signature", config: transit.Config{Key: "sig"}, input: []byte("m"), response: map[string]any{}, wantErr: transit.ErrBadResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeTransit{response: tt.response}
			client := newTransit(t, fake, tt.config)

			got, err := client.Sign(context.Background(), tt.input)
			if tt.noCall {
				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, fake.requests)
				return
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, "/v1/transit/sign/sig", fake.requests[0].Path)
			require.Equal(t, tt.wantBody, fake.requests[0].Body)
		})
	}
}

func TestVerify(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		signature string
		response  map[string]any
		want      bool
		wantErr   error
		noCall    bool
	}{
		{name: "valid", input: []byte("m"), signature: "s", response: map[string]any{"valid": true}, want: true},
		{name: "invalid", input: []byte("m"), signature: "s", response: map[string]any{"valid": false}},
		{name: "empty input", signature: "s", wantErr: transit.ErrEmptyInput, noCall: true},
		{name: "empty signature", input: []byte("m"), wantErr: transit.ErrEmptyInput, noCall: true},
		{name: "no valid field", input: []byte("m"), signature: "s", response: map[string]any{}, wantErr: transit.ErrBadResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeTransit{response: tt.response}
			client := newTransit(t, fake, transit.Config{Key: "sig", HashAlgorithm: "sha2-256"})

			got, err := client.Verify(context.Background(), tt.input, tt.signature)
			if tt.noCall {
				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, fake.requests)
				return
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, "/v1/transit/verify/sig", fake.requests[0].Path)
			require.Equal(t, "s", fake.requests[0].Body["signature"])
			require.Equal(t, "sha2-256", fake.requests[0].Body["hash_algorithm"])
		})
	}
}

type failingRequester struct {
	err error
}

func (f failingRequester) Write(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
	return nil, f.err
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		wantUnavailable bool
		wantDenied      bool
	}{
		{name: "denied maps to unavailable", err: fmt.Errorf("w: %w", hashi.ErrPermissionDenied), wantUnavailable: true, wantDenied: true},
		{name: "unavailable stays", err: fmt.Errorf("w: %w", hashi.ErrUnavailable), wantUnavailable: true},
		{name: "other untouched", err: errors.New("boom")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := transit.New(failingRequester{err: tt.err}, transit.Config{Key: "k"})
			require.NoError(t, err)

			_, encErr := client.Encrypt(context.Background(), []byte("x"), nil)
			require.Equal(t, tt.wantUnavailable, errors.Is(encErr, transit.ErrUnavailable))
			require.Equal(t, tt.wantDenied, errors.Is(encErr, hashi.ErrPermissionDenied))
			require.ErrorIs(t, encErr, tt.err)
		})
	}
}

func TestNewValidation(t *testing.T) {
	tests := []struct {
		name      string
		requester transit.Requester
		config    transit.Config
		wantErr   error
	}{
		{name: "nil requester", config: transit.Config{Key: "k"}},
		{name: "empty key", requester: failingRequester{}, wantErr: transit.ErrEmptyKey},
		{name: "ok", requester: failingRequester{}, config: transit.Config{Key: "k"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := transit.New(tt.requester, tt.config)
			switch {
			case tt.requester == nil:
				require.Error(t, err)
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			default:
				require.NoError(t, err)
				require.NotNil(t, client)
			}
		})
	}
}

func TestAAD(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{name: "provider direction name", parts: []string{"prov", "in", "key"}, want: "4:prov|2:in|3:key"},
		{name: "single", parts: []string{"ab"}, want: "2:ab"},
		{name: "none", parts: nil, want: ""},
		{name: "ambiguity is prevented", parts: []string{"a|1:b"}, want: "5:a|1:b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, string(transit.AAD(tt.parts...)))
		})
	}
	require.NotEqual(t, transit.AAD("a|1:b"), transit.AAD("a", "b"))
}

func TestFxModule(t *testing.T) {
	var (
		client *transit.Client
		cipher transit.Cipher
		signer transit.Signer
	)
	app := fxtest.New(
		t,
		vault.NewHashiFxWithConfig(hashi.Config{
			Address: "http://127.0.0.1:1",
			Token:   hashi.StaticToken("t"),
		}),
		transit.FxModule(transit.Config{Key: "k"}),
		fx.Populate(&client, &cipher, &signer),
	)
	app.RequireStart()
	app.RequireStop()
	require.NotNil(t, client)
	require.NotNil(t, cipher)
	require.NotNil(t, signer)
}
