package hashi

import (
	"errors"
	"fmt"
	"net/http"

	vaultclient "github.com/hashicorp/vault-client-go"
)

var (
	ErrInvalidKeyFormat     = errors.New("invalid key format: must be at least 3 parts - `types`.`namespace`.`key`")
	ErrUnsupportedVaultType = errors.New("unsupported vault type. Only 'kv' is supported")
	ErrNotFound             = errors.New("vault: secret not found")
	ErrPermissionDenied     = errors.New("vault: permission denied")
	ErrUnavailable          = errors.New("vault: unavailable")
	ErrEmptyAddress         = errors.New("vault: address is empty")
	ErrEmptyPath            = errors.New("vault: path is empty")
	ErrInvalidSecretRef     = errors.New("vault: secret ref requires mount and path")
	ErrUnsupportedKVVersion = errors.New("vault: unsupported kv version")
	ErrEmptyToken           = errors.New("vault: token source is not configured")
)

func classify(err error) error {
	if err == nil {
		return nil
	}

	var responseErr *vaultclient.ResponseError
	if !errors.As(err, &responseErr) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	switch status := responseErr.StatusCode; {
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case status == http.StatusForbidden:
		return fmt.Errorf("%w: %w", ErrPermissionDenied, err)
	case status >= http.StatusInternalServerError:
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return err
}

func envNotSet(name string) error {
	return errors.New("variable not set: " + name)
}
