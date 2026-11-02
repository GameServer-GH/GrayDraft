package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOverviewModelEnvironmentOverridesYAML(t *testing.T) {
	cfg := &Config{}
	cfg.Chat.OverviewModel = "fast"
	t.Setenv("OVERVIEW_MODEL", "complex")
	overlayEnv(cfg)
	applyDefaults(cfg)
	if cfg.Chat.OverviewModel != "complex" {
		t.Fatal(cfg.Chat.OverviewModel)
	}
	cfg = &Config{}
	applyDefaults(cfg)
	if cfg.Chat.OverviewModel != "complex" {
		t.Fatal("default model changed")
	}
}

func TestOverviewModelRejectsUnknownTier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("chat:\n  overview_model: typo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OVERVIEW_MODEL", "typo")
	if _, err := Load(path); err == nil {
		t.Fatal("unknown overview tier accepted")
	}
}

func TestOverviewHedgeConfigurationDefaultsOffAndRejectsInvalid(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)
	if cfg.Chat.OverviewHedgeAfterMS != 0 {
		t.Fatal("hedging enabled by default")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("chat:\n  overview_hedge_after_ms: 4000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"-1", "bad"} {
		t.Setenv("OVERVIEW_HEDGE_AFTER_MS", value)
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	t.Setenv("OVERVIEW_HEDGE_AFTER_MS", "2000")
	loaded, err := Load(path)
	if err != nil || loaded.Chat.OverviewHedgeAfterMS != 2000 {
		t.Fatalf("%+v %v", loaded, err)
	}
}
