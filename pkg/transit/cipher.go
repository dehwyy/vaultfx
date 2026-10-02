package transit

import (
	"context"
	"encoding/base64"
	"fmt"
)

func (c *Client) Encrypt(ctx context.Context, plaintext, aad []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", fmt.Errorf("%w: plaintext", ErrEmptyInput)
	}

	body := map[string]any{
		"plaintext": base64.StdEncoding.EncodeToString(plaintext),
	}
	if len(aad) > 0 {
		body["associated_data"] = base64.StdEncoding.EncodeToString(aad)
	}

	data, err := c.call(ctx, "encrypt", body)
	if err != nil {
		return "", err
	}
	ciphertext, ok := data["ciphertext"].(string)
	if !ok || ciphertext == "" {
		return "", fmt.Errorf("%w: encrypt response without ciphertext", ErrBadResponse)
	}
	return ciphertext, nil
}

func (c *Client) Decrypt(ctx context.Context, ciphertext string, aad []byte) ([]byte, error) {
	if ciphertext == "" {
		return nil, fmt.Errorf("%w: ciphertext", ErrEmptyInput)
	}

	body := map[string]any{
		"ciphertext": ciphertext,
	}
	if len(aad) > 0 {
		body["associated_data"] = base64.StdEncoding.EncodeToString(aad)
	}

	data, err := c.call(ctx, "decrypt", body)
	if err != nil {
		return nil, err
	}
	encoded, ok := data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: decrypt response without plaintext", ErrBadResponse)
	}
	plaintext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: decrypt response is not base64", ErrBadResponse)
	}
	return plaintext, nil
}
