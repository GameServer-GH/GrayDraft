package sealed

import (
	"strings"
	"testing"
)

func TestBuiltinKeys(t *testing.T) {
	ds, intern, dash := DeepSeek(), Intern(), DashScope()
	if len(ds) != 35 || len(dash) != 35 || len(intern) != 51 {
		t.Fatalf("unexpected lengths: %d %d %d", len(ds), len(intern), len(dash))
	}
	if ds == dash {
		t.Fatal("deepseek and dashscope decrypted to the same value")
	}
	for _, key := range []string{ds, intern, dash} {
		if !strings.HasPrefix(key, "sk-") {
			t.Fatal("decrypted key missing expected prefix")
		}
	}
	tls := TLSKey()
	if len(tls) < 1000 || !strings.HasPrefix(tls, "-----BEGIN") {
		t.Fatalf("tls material not decrypted, len=%d", len(tls))
	}
}
