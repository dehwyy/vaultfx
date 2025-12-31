package hashi

import "errors"

var (
	ErrInvalidKeyFormat     = errors.New("invalid key format: must be at least 3 parts - `types`.`namespace`.`key`")
	ErrUnsupportedVaultType = errors.New("unsupported vault type. Only 'kv' is supported")
)
