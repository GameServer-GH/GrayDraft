package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// VaultSecretProvider resolves KV v2 references such as secret/data/game/api.
// The token is held by the process environment and values are never persisted.
type VaultSecretProvider struct {
	BaseURL, Token string
	Client         *http.Client
}

func (v VaultSecretProvider) Resolve(ctx context.Context, refs []string) (map[string]string, error) {
	if v.BaseURL == "" || v.Token == "" {
		return nil, fmt.Errorf("vault secret provider is not configured")
	}
	client := v.Client
	if client == nil {
		client = http.DefaultClient
	}
	out := make(map[string]string, len(refs))
	for _, ref := range refs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(v.BaseURL, "/")+"/v1/"+strings.TrimLeft(ref, "/"), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Vault-Token", v.Token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode/100 != 2 {
			return nil, fmt.Errorf("vault secret %q returned HTTP %d", ref, resp.StatusCode)
		}
		var envelope struct {
			Data struct {
				Data map[string]any `json:"data"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, err
		}
		value, ok := envelope.Data.Data["value"]
		if !ok {
			return nil, fmt.Errorf("vault secret %q has no value", ref)
		}
		out[ref] = fmt.Sprint(value)
	}
	return out, nil
}
