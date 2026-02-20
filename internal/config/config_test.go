package config

import (
	"testing"
	"time"
)

func TestLoadDefaultsAndParsing(t *testing.T) {
	t.Setenv("SRC_RD_TOKEN", "src")
	t.Setenv("DST_RD_TOKEN", "dst")
	t.Setenv("SYNC_INTERVAL", "60")
	t.Setenv("DRY_RUN", "false")
	t.Setenv("MIRROR_MODE", "mirror-delete")
	t.Setenv("HEALTH_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Interval != 60*time.Second {
		t.Fatalf("expected 60s interval, got %s", cfg.Interval)
	}
	if cfg.DryRun {
		t.Fatalf("expected dry_run=false")
	}
	if cfg.Mode != "mirror-delete" {
		t.Fatalf("expected mirror-delete mode, got %s", cfg.Mode)
	}
	if cfg.HealthAddr != "" {
		t.Fatalf("expected empty health addr")
	}
}

func TestLoadRejectsMissingTokens(t *testing.T) {
	t.Setenv("SRC_RD_TOKEN", "")
	t.Setenv("DST_RD_TOKEN", "")

	if _, err := Load(); err == nil {
		t.Fatalf("expected error for missing tokens")
	}
}
