package hashi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type manualClock struct {
	current time.Time
}

func (c *manualClock) now() time.Time {
	return c.current
}

func TestStaticToken(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr error
	}{
		{name: "value", value: "hvs.abc", want: "hvs.abc"},
		{name: "trimmed", value: "  hvs.abc\n", want: "hvs.abc"},
		{name: "empty", value: "  ", wantErr: ErrEmptyToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := StaticToken(tt.value)
			got, err := source.Token(context.Background())
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			source.Invalidate()
			again, err := source.Token(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.want, again)
		})
	}
}

func TestFileToken(t *testing.T) {
	ctx := context.Background()

	t.Run("reads and trims file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(path, []byte("tok-1\n"), 0o600))
		got, err := FileToken(path, time.Minute).Token(ctx)
		require.NoError(t, err)
		require.Equal(t, "tok-1", got)
	})

	t.Run("caches until ttl then rereads", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(path, []byte("tok-1"), 0o600))
		clock := &manualClock{current: time.Unix(1000, 0)}
		source := newFileToken(path, time.Minute, clock.now)

		got, err := source.Token(ctx)
		require.NoError(t, err)
		require.Equal(t, "tok-1", got)

		require.NoError(t, os.WriteFile(path, []byte("tok-2"), 0o600))
		clock.current = clock.current.Add(30 * time.Second)
		got, err = source.Token(ctx)
		require.NoError(t, err)
		require.Equal(t, "tok-1", got)

		clock.current = clock.current.Add(31 * time.Second)
		got, err = source.Token(ctx)
		require.NoError(t, err)
		require.Equal(t, "tok-2", got)
	})

	t.Run("invalidate forces reread", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(path, []byte("tok-1"), 0o600))
		source := FileToken(path, time.Hour)
		_, err := source.Token(ctx)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(path, []byte("tok-2"), 0o600))
		source.Invalidate()
		got, err := source.Token(ctx)
		require.NoError(t, err)
		require.Equal(t, "tok-2", got)
	})

	t.Run("keeps last token when file becomes unreadable or empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(path, []byte("tok-1"), 0o600))
		source := FileToken(path, time.Hour)
		_, err := source.Token(ctx)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(path, []byte("  "), 0o600))
		source.Invalidate()
		got, err := source.Token(ctx)
		require.NoError(t, err)
		require.Equal(t, "tok-1", got)

		require.NoError(t, os.Remove(path))
		source.Invalidate()
		got, err = source.Token(ctx)
		require.NoError(t, err)
		require.Equal(t, "tok-1", got)
	})

	t.Run("errors when nothing was ever read", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		_, err := FileToken(missing, time.Minute).Token(ctx)
		require.ErrorIs(t, err, ErrUnavailable)

		empty := filepath.Join(t.TempDir(), "empty")
		require.NoError(t, os.WriteFile(empty, []byte("\n"), 0o600))
		_, err = FileToken(empty, time.Minute).Token(ctx)
		require.ErrorIs(t, err, ErrUnavailable)
	})

	t.Run("default ttl when non positive", func(t *testing.T) {
		source := newFileToken("x", 0, time.Now)
		require.Equal(t, DefaultTokenTTL, source.ttl)
	})
}
