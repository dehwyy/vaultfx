package hashi

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const DefaultTokenTTL = time.Minute

type TokenSource interface {
	Token(ctx context.Context) (string, error)
	Invalidate()
}

type staticToken struct {
	value string
}

func StaticToken(value string) TokenSource {
	return &staticToken{value: strings.TrimSpace(value)}
}

func (s *staticToken) Token(_ context.Context) (string, error) {
	if s.value == "" {
		return "", ErrEmptyToken
	}
	return s.value, nil
}

func (s *staticToken) Invalidate() {}

type fileToken struct {
	path string
	ttl  time.Duration
	now  func() time.Time

	mu     sync.Mutex
	token  string
	readAt time.Time
	stale  bool
}

func FileToken(path string, ttl time.Duration) TokenSource {
	return newFileToken(path, ttl, time.Now)
}

func newFileToken(path string, ttl time.Duration, now func() time.Time) *fileToken {
	if ttl <= 0 {
		ttl = DefaultTokenTTL
	}
	return &fileToken{
		path: path,
		ttl:  ttl,
		now:  now,
	}
}

func (f *fileToken) Token(_ context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.token != "" && !f.stale && f.now().Sub(f.readAt) < f.ttl {
		return f.token, nil
	}

	token, err := f.read()
	f.readAt = f.now()
	f.stale = false
	if err != nil {
		if f.token != "" {
			return f.token, nil
		}
		return "", err
	}
	f.token = token
	return token, nil
}

func (f *fileToken) Invalidate() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stale = true
}

func (f *fileToken) read() (string, error) {
	raw, err := os.ReadFile(f.path)
	if err != nil {
		return "", fmt.Errorf("%w: read token file %q: %w", ErrUnavailable, f.path, err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", fmt.Errorf("%w: token file %q is empty", ErrUnavailable, f.path)
	}
	return token, nil
}
