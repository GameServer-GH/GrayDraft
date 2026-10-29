package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVaultSecretProviderResolvesValue(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "token" {
			t.Fatal("missing token")
		}
		_, _ = w.Write([]byte(`{"data":{"data":{"value":"secret"}}}`))
	}))
	defer s.Close()
	got, err := (VaultSecretProvider{BaseURL: s.URL, Token: "token"}).Resolve(context.Background(), []string{"secret/data/tool"})
	if err != nil || got["secret/data/tool"] != "secret" {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
