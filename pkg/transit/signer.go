package transit

import (
	"context"
	"encoding/base64"
	"fmt"
)

func (c *Client) signBody(input []byte) map[string]any {
	body := map[string]any{
		"input": base64.StdEncoding.EncodeToString(input),
	}
	if c.config.HashAlgorithm != "" {
		body["hash_algorithm"] = c.config.HashAlgorithm
	}
	if c.config.SignatureAlgorithm != "" {
		body["signature_algorithm"] = c.config.SignatureAlgorithm
	}
	return body
}

func (c *Client) Sign(ctx context.Context, input []byte) (string, error) {
	if len(input) == 0 {
		return "", fmt.Errorf("%w: sign input", ErrEmptyInput)
	}

	data, err := c.call(ctx, "sign", c.signBody(input))
	if err != nil {
		return "", err
	}
	signature, ok := data["signature"].(string)
	if !ok || signature == "" {
		return "", fmt.Errorf("%w: sign response without signature", ErrBadResponse)
	}
	return signature, nil
}

func (c *Client) Verify(ctx context.Context, input []byte, signature string) (bool, error) {
	if len(input) == 0 {
		return false, fmt.Errorf("%w: verify input", ErrEmptyInput)
	}
	if signature == "" {
		return false, fmt.Errorf("%w: signature", ErrEmptyInput)
	}

	body := c.signBody(input)
	body["signature"] = signature

	data, err := c.call(ctx, "verify", body)
	if err != nil {
		return false, err
	}
	valid, ok := data["valid"].(bool)
	if !ok {
		return false, fmt.Errorf("%w: verify response without valid", ErrBadResponse)
	}
	return valid, nil
}
