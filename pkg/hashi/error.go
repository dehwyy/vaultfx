package hashi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
)

var (
	ErrInvalidKeyFormat     = errors.New("invalid key format: must be at least 3 parts - `types`.`namespace`.`key`")
	ErrUnsupportedVaultType = errors.New("unsupported vault type. Only 'kv' is supported")
	ErrNotFound             = errors.New("vault: secret not found")
	ErrPermissionDenied     = errors.New("vault: permission denied")
	ErrUnavailable          = errors.New("vault: unavailable")
	ErrEmptyAddress         = errors.New("vault: address is empty")
	ErrEmptyToken           = errors.New("vault: token source is not configured")
)

func statusOf(err error) int {
	var responseErr *vaultapi.ResponseError
	if errors.As(err, &responseErr) {
		return responseErr.StatusCode
	}
	return 0
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, vaultapi.ErrSecretNotFound) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	switch status := statusOf(err); {
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case status == http.StatusForbidden:
		return fmt.Errorf("%w: %w", ErrPermissionDenied, err)
	case status >= http.StatusInternalServerError:
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	case status != 0:
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	msg := err.Error()
	for _, marker := range unavailableMarkers {
		if strings.Contains(msg, marker) {
			return fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
	}
	return err
}

var unavailableMarkers = []string{
	"connection refused",
	"no such host",
	"i/o timeout",
	"context deadline exceeded",
	"EOF",
}

func envNotSet(name string) error {
	return errors.New("variable not set: " + name)
}
