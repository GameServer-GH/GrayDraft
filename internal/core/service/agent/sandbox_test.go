package agent

import (
	"context"
	"testing"
)

func TestDockerSandboxRequiresControlledEgressNetwork(t *testing.T) {
	r := DockerSandboxRunner{Image: "tool-worker"}
	_, err := r.Run(context.Background(), ToolSpec{RunnerCommand: "run", NetworkAllowlist: []string{"api.example.com"}}, ToolCall{Name: "remote"}, nil)
	if err == nil {
		t.Fatal("expected controlled egress network error")
	}
}

func TestSecretProviderFailsClosed(t *testing.T) {
	if _, err := SecretEnv(context.Background(), nil, []string{"MISSING"}); err == nil {
		t.Fatal("expected missing provider error")
	}
}

func TestNetworkArgumentsRejectUnknownHost(t *testing.T) {
	if err := ValidateNetworkArguments([]byte(`{"url":"https://evil.example/x"}`), []string{"api.example.com"}); err == nil {
		t.Fatal("expected allowlist rejection")
	}
	if err := ValidateNetworkArguments([]byte(`{"url":"https://api.example.com/x"}`), []string{"api.example.com"}); err != nil {
		t.Fatal(err)
	}
}
