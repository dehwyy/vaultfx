package hashi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
	Method string
	Path   string
	Token  string
	Body   map[string]any
}

type fakeVault struct {
	t       *testing.T
	server  *httptest.Server
	mu      sync.Mutex
	records []recordedRequest
	handler func(call int, req recordedRequest, w http.ResponseWriter)
}

func newFakeVault(t *testing.T, handler func(call int, req recordedRequest, w http.ResponseWriter)) *fakeVault {
	t.Helper()
	fake := &fakeVault{
		t:       t,
		handler: handler,
	}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := recordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Token:  r.Header.Get("X-Vault-Token"),
		}
		if r.Body != nil && r.ContentLength != 0 {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req.Body))
		}
		fake.mu.Lock()
		fake.records = append(fake.records, req)
		call := len(fake.records)
		fake.mu.Unlock()
		fake.handler(call, req, w)
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeVault) requests() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.records...)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		panic(err)
	}
}

func newClient(t *testing.T, fake *fakeVault, token TokenSource) *Hashi {
	t.Helper()
	client, err := NewWithConfig(Config{
		Address:    fake.server.URL,
		Token:      token,
		MaxRetries: -1,
		Timeout:    2 * time.Second,
	})
	require.NoError(t, err)
	return client
}
