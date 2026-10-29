package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// SecretProvider resolves references without exposing the secret store to tools.
type SecretProvider interface {
	Resolve(context.Context, []string) (map[string]string, error)
}

// EnvironmentSecretProvider is the local adapter; production deployments can
// replace it with Vault/KMS while keeping the same reference-only contract.
type EnvironmentSecretProvider struct{}

func (EnvironmentSecretProvider) Resolve(_ context.Context, refs []string) (map[string]string, error) {
	out := make(map[string]string, len(refs))
	for _, ref := range refs {
		value, ok := os.LookupEnv(ref)
		if !ok {
			return nil, fmt.Errorf("secret reference %q not found", ref)
		}
		out[ref] = value
	}
	return out, nil
}

// SandboxRunner is the isolation boundary for non in-process tools.
type SandboxRunner interface {
	Run(context.Context, ToolSpec, ToolCall, map[string]string) (Observation, error)
}

// DockerSandboxRunner executes a tool worker as a short-lived container. The image
// is responsible for enforcing the declared allowlist through its egress proxy.
type DockerSandboxRunner struct {
	DockerBin     string
	Image         string
	EgressNetwork string
}

type RemoteSandboxRunner struct {
	URL, Token string
	Client     *http.Client
}

func (r RemoteSandboxRunner) Run(ctx context.Context, spec ToolSpec, call ToolCall, secrets map[string]string) (Observation, error) {
	payload, _ := json.Marshal(struct {
		Spec    ToolSpec          `json:"spec"`
		Call    ToolCall          `json:"call"`
		Secrets map[string]string `json:"secrets"`
	}{spec, call, secrets})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(r.URL, "/")+"/execute", bytes.NewReader(payload))
	if err != nil {
		return Observation{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.Token != "" {
		req.Header.Set("Authorization", "Bearer "+r.Token)
	}
	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Observation{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return Observation{}, err
	}
	if resp.StatusCode/100 != 2 {
		return Observation{CallID: call.CallID, Name: call.Name}, fmt.Errorf("remote sandbox HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var result struct {
		CallID string          `json:"call_id"`
		Name   string          `json:"name"`
		Output json.RawMessage `json:"output"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return Observation{}, err
	}
	obs := Observation{CallID: result.CallID, Name: result.Name, Output: result.Output}
	if result.Error != "" {
		obs.Err = fmt.Errorf("%s", result.Error)
		return obs, obs.Err
	}
	return obs, nil
}

func (r DockerSandboxRunner) Run(ctx context.Context, spec ToolSpec, call ToolCall, secrets map[string]string) (Observation, error) {
	if strings.TrimSpace(r.Image) == "" || strings.TrimSpace(spec.RunnerCommand) == "" {
		return Observation{CallID: call.CallID, Name: call.Name}, fmt.Errorf("sandbox image and command are required")
	}
	if err := ValidateNetworkArguments(call.Arguments, spec.NetworkAllowlist); err != nil {
		return Observation{CallID: call.CallID, Name: call.Name}, err
	}
	bin := r.DockerBin
	if bin == "" {
		bin = "docker"
	}
	network := "none"
	if len(spec.NetworkAllowlist) > 0 {
		network = r.EgressNetwork
		if network == "" {
			return Observation{CallID: call.CallID, Name: call.Name}, fmt.Errorf("egress network is required for network-enabled tools")
		}
	}
	args := []string{"run", "--rm", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--network=" + network, "-e", "KNX_NETWORK_ALLOWLIST=" + strings.Join(spec.NetworkAllowlist, ",")}
	if network != "none" {
		args = append(args, "-e", "HTTP_PROXY=http://tool-egress-proxy:3128", "-e", "HTTPS_PROXY=http://tool-egress-proxy:3128")
	}
	env := os.Environ()
	for key, value := range secrets {
		name := "KNX_SECRET_" + strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(key))
		args = append(args, "-e", name)
		env = append(env, name+"="+value)
	}
	args = append(args, r.Image, spec.RunnerCommand)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(call.Arguments)
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return Observation{CallID: call.CallID, Name: call.Name, Output: out.Bytes(), Err: fmt.Errorf("sandbox worker: %w: %s", err, strings.TrimSpace(stderr.String()))}, err
	}
	var raw json.RawMessage = json.RawMessage(out.Bytes())
	if !json.Valid(raw) {
		return Observation{CallID: call.CallID, Name: call.Name, Output: raw}, fmt.Errorf("sandbox worker returned invalid JSON")
	}
	return Observation{CallID: call.CallID, Name: call.Name, Output: raw}, nil
}

func ValidateNetworkArguments(arguments []byte, allowlist []string) error {
	var value any
	if err := json.Unmarshal(arguments, &value); err != nil {
		return err
	}
	allowed := func(host string) bool {
		for _, domain := range allowlist {
			domain = strings.ToLower(strings.TrimSpace(domain))
			host = strings.ToLower(host)
			if host == domain || strings.HasSuffix(host, "."+domain) {
				return true
			}
		}
		return false
	}
	var visit func(any) error
	visit = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			for _, child := range x {
				if err := visit(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range x {
				if err := visit(child); err != nil {
					return err
				}
			}
		case string:
			if u, err := url.Parse(x); err == nil && u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") && !allowed(u.Hostname()) {
				return fmt.Errorf("network host %q is not allowlisted", u.Hostname())
			}
		}
		return nil
	}
	return visit(value)
}

// SecretEnv resolves references immediately before execution and never persists values.
func SecretEnv(ctx context.Context, provider SecretProvider, refs []string) (map[string]string, error) {
	if len(refs) == 0 {
		return map[string]string{}, nil
	}
	if provider == nil {
		return nil, fmt.Errorf("secret provider is not configured")
	}
	return provider.Resolve(ctx, refs)
}
