package config

import (
	"os"
	"testing"
	"time"

	"rdmirrorsync/internal/syncer"
)

func writeConfig(t *testing.T, content string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Setenv("CONFIG_FILE", f.Name())
}

func TestResolveDefaults(t *testing.T) {
	writeConfig(t, `{
		"src_token": "src",
		"destinations": [{"name": "stavanger", "token": "dst"}]
	}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Interval != defaultInterval {
		t.Errorf("interval: got %s, want %s", cfg.Interval, defaultInterval)
	}
	if cfg.PageLimit != defaultPageLimit {
		t.Errorf("page_limit: got %d, want %d", cfg.PageLimit, defaultPageLimit)
	}
	if len(cfg.Destinations) != 1 {
		t.Fatalf("expected 1 destination, got %d", len(cfg.Destinations))
	}
	d := cfg.Destinations[0]
	if d.Mode != syncer.ModeAddOnly {
		t.Errorf("mode: got %s, want %s", d.Mode, syncer.ModeAddOnly)
	}
	if d.DryRun {
		t.Errorf("expected dry_run=false by default")
	}
}

func TestResolvePerDestinationOverrides(t *testing.T) {
	writeConfig(t, `{
		"src_token": "src",
		"mode": "add-only",
		"dry_run": false,
		"destinations": [
			{"name": "stavanger", "token": "dst1"},
			{"name": "brother",   "token": "dst2", "mode": "mirror-delete", "dry_run": true}
		]
	}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Destinations) != 2 {
		t.Fatalf("expected 2 destinations, got %d", len(cfg.Destinations))
	}

	stavanger := cfg.Destinations[0]
	if stavanger.Mode != syncer.ModeAddOnly {
		t.Errorf("stavanger mode: got %s, want add-only", stavanger.Mode)
	}
	if stavanger.DryRun {
		t.Errorf("stavanger: expected dry_run=false")
	}

	brother := cfg.Destinations[1]
	if brother.Mode != syncer.ModeMirrorDelete {
		t.Errorf("brother mode: got %s, want mirror-delete", brother.Mode)
	}
	if !brother.DryRun {
		t.Errorf("brother: expected dry_run=true")
	}
}

func TestResolveDurationParsing(t *testing.T) {
	writeConfig(t, `{
		"src_token": "src",
		"interval": "2m",
		"http_timeout": "30s",
		"destinations": [{"name": "stavanger", "token": "dst"}]
	}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Interval != 2*time.Minute {
		t.Errorf("interval: got %s, want 2m", cfg.Interval)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("http_timeout: got %s, want 30s", cfg.HTTPTimeout)
	}
}

func TestResolveRejectsMissingSrcToken(t *testing.T) {
	writeConfig(t, `{"destinations": [{"name": "x", "token": "y"}]}`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for missing src_token")
	}
}

func TestResolveRejectsNoDestinations(t *testing.T) {
	writeConfig(t, `{"src_token": "src"}`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for empty destinations")
	}
}

func TestResolveRejectsDuplicateDestinationName(t *testing.T) {
	writeConfig(t, `{
		"src_token": "src",
		"destinations": [
			{"name": "x", "token": "t1"},
			{"name": "x", "token": "t2"}
		]
	}`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for duplicate destination name")
	}
}

func TestResolveRejectsInvalidMode(t *testing.T) {
	writeConfig(t, `{
		"src_token": "src",
		"mode": "bad-mode",
		"destinations": [{"name": "x", "token": "t"}]
	}`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestResolveEnabledFalseSkipsDestination(t *testing.T) {
	writeConfig(t, `{
		"src_token": "src",
		"destinations": [
			{"name": "active",   "token": "t1"},
			{"name": "inactive", "token": "t2", "enabled": false}
		]
	}`)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Destinations) != 1 {
		t.Fatalf("expected 1 active destination, got %d", len(cfg.Destinations))
	}
	if cfg.Destinations[0].Name != "active" {
		t.Errorf("expected destination name 'active', got %q", cfg.Destinations[0].Name)
	}
}

func TestResolveRejectsAllDisabled(t *testing.T) {
	writeConfig(t, `{
		"src_token": "src",
		"destinations": [
			{"name": "x", "token": "t", "enabled": false}
		]
	}`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error when all destinations are disabled")
	}
}

func TestResolveTokenFromEnvVar(t *testing.T) {
	t.Setenv("SRC_RD_TOKEN", "src-from-env")
	t.Setenv("RD_TOKEN_MYPLACE", "dst-from-env")
	writeConfig(t, `{
		"destinations": [{"name": "myplace"}]
	}`)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.SrcToken != "src-from-env" {
		t.Errorf("src token: got %q, want 'src-from-env'", cfg.SrcToken)
	}
	if cfg.Destinations[0].Token != "dst-from-env" {
		t.Errorf("dst token: got %q, want 'dst-from-env'", cfg.Destinations[0].Token)
	}
}

func TestResolveTokenEnvKeyFormat(t *testing.T) {
	cases := []struct{ name, want string }{
		{"stavanger", "RD_TOKEN_STAVANGER"},
		{"location-1", "RD_TOKEN_LOCATION_1"},
		{"my place", "RD_TOKEN_MY_PLACE"},
	}
	for _, c := range cases {
		if got := tokenEnvKey(c.name); got != c.want {
			t.Errorf("tokenEnvKey(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
