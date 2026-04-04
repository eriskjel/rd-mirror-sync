package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"rdmirrorsync/internal/syncer"
)

const (
	defaultConfigPath  = "config.json"
	defaultBaseURL     = "https://api.real-debrid.com/rest/1.0"
	defaultMode        = "add-only"
	defaultInterval    = 45 * time.Second
	defaultRunTimeout  = 10 * time.Minute
	defaultHTTPTimeout = 20 * time.Second
	defaultWriteDelay  = 250 * time.Millisecond
	defaultMaxRetries  = 4
	defaultRetryBase   = 500 * time.Millisecond
	defaultRetryJitter = 350 * time.Millisecond
	defaultPageLimit   = 250
)

// rawDestination is the JSON shape for a single destination entry.
type rawDestination struct {
	Name            string `json:"name"`
	Token           string `json:"token"`
	Mode            string `json:"mode"`
	DryRun          *bool  `json:"dry_run"`
	Enabled         *bool  `json:"enabled"`
	ProtectDstRegex string `json:"protect_dst_regex"`
}

// rawConfig is the JSON shape of the config file.
type rawConfig struct {
	SrcToken       string           `json:"src_token"`
	BaseURL        string           `json:"base_url"`
	HealthAddr     string           `json:"health_addr"`
	Mode           string           `json:"mode"`
	DryRun         bool             `json:"dry_run"`
	Interval       string           `json:"interval"`
	RunTimeout     string           `json:"run_timeout"`
	HTTPTimeout    string           `json:"http_timeout"`
	WriteDelay     string           `json:"write_delay"`
	MaxRetries     int              `json:"max_retries"`
	RetryBase      string           `json:"retry_base"`
	RetryMaxJitter string           `json:"retry_max_jitter"`
	PageLimit      int              `json:"page_limit"`
	Destinations   []rawDestination `json:"destinations"`
}

// Destination is a fully resolved destination with all per-destination
// overrides applied on top of the global defaults.
type Destination struct {
	Name            string
	Token           string
	Mode            syncer.Mode
	DryRun          bool
	ProtectDstRegex string
}

// Config is the resolved, validated configuration.
type Config struct {
	SrcToken       string
	BaseURL        string
	HealthAddr     string
	Interval       time.Duration
	RunTimeout     time.Duration
	HTTPTimeout    time.Duration
	WriteDelay     time.Duration
	MaxRetries     int
	RetryBase      time.Duration
	RetryMaxJitter time.Duration
	PageLimit      int
	Destinations   []Destination
}

// Load reads and validates the config file. The path defaults to "config.json"
// in the working directory and can be overridden with the CONFIG_FILE env var.
func Load() (Config, error) {
	path := strings.TrimSpace(os.Getenv("CONFIG_FILE"))
	if path == "" {
		path = defaultConfigPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return resolve(raw)
}

// tokenEnvKey returns the env var name for a destination token, e.g.
// "stavanger" → RD_TOKEN_STAVANGER, "location-1" → RD_TOKEN_LOCATION_1.
func tokenEnvKey(name string) string {
	return "RD_TOKEN_" + strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(name))
}

func resolve(raw rawConfig) (Config, error) {
	srcToken := stringOr(raw.SrcToken, os.Getenv("SRC_RD_TOKEN"))
	if srcToken == "" {
		return Config{}, errors.New("src_token is required (set in config or SRC_RD_TOKEN env var)")
	}
	if len(raw.Destinations) == 0 {
		return Config{}, errors.New("at least one destination is required")
	}

	globalMode, err := parseMode(raw.Mode, defaultMode)
	if err != nil {
		return Config{}, fmt.Errorf("mode: %w", err)
	}

	cfg := Config{
		SrcToken:       srcToken,
		BaseURL:        stringOr(raw.BaseURL, defaultBaseURL),
		HealthAddr:     raw.HealthAddr,
		Interval:       durationOr(raw.Interval, defaultInterval),
		RunTimeout:     durationOr(raw.RunTimeout, defaultRunTimeout),
		HTTPTimeout:    durationOr(raw.HTTPTimeout, defaultHTTPTimeout),
		WriteDelay:     durationOr(raw.WriteDelay, defaultWriteDelay),
		MaxRetries:     intOr(raw.MaxRetries, defaultMaxRetries),
		RetryBase:      durationOr(raw.RetryBase, defaultRetryBase),
		RetryMaxJitter: durationOr(raw.RetryMaxJitter, defaultRetryJitter),
		PageLimit:      intOr(raw.PageLimit, defaultPageLimit),
	}

	if cfg.Interval < 10*time.Second {
		return Config{}, errors.New("interval must be >= 10s")
	}
	if cfg.HTTPTimeout <= 0 {
		return Config{}, errors.New("http_timeout must be > 0")
	}
	if cfg.MaxRetries < 1 {
		return Config{}, errors.New("max_retries must be >= 1")
	}
	if cfg.PageLimit < 1 {
		return Config{}, errors.New("page_limit must be >= 1")
	}

	seen := make(map[string]bool, len(raw.Destinations))
	for i, rd := range raw.Destinations {
		name := strings.TrimSpace(rd.Name)
		if name == "" {
			return Config{}, fmt.Errorf("destinations[%d]: name is required", i)
		}
		if seen[name] {
			return Config{}, fmt.Errorf("destination %q: duplicate name", name)
		}
		seen[name] = true

		if rd.Enabled != nil && !*rd.Enabled {
			continue
		}

		token := stringOr(rd.Token, os.Getenv(tokenEnvKey(name)))
		if token == "" {
			return Config{}, fmt.Errorf("destination %q: token is required (set in config or %s env var)", name, tokenEnvKey(name))
		}

		mode := globalMode
		if rd.Mode != "" {
			mode, err = parseMode(rd.Mode, "")
			if err != nil {
				return Config{}, fmt.Errorf("destination %q: %w", name, err)
			}
		}

		dryRun := raw.DryRun
		if rd.DryRun != nil {
			dryRun = *rd.DryRun
		}

		cfg.Destinations = append(cfg.Destinations, Destination{
			Name:            name,
			Token:           token,
			Mode:            mode,
			DryRun:          dryRun,
			ProtectDstRegex: rd.ProtectDstRegex,
		})
	}

	if len(cfg.Destinations) == 0 {
		return Config{}, errors.New("all destinations are disabled; enable at least one")
	}

	return cfg, nil
}

func parseMode(s, def string) (syncer.Mode, error) {
	if s == "" {
		s = def
	}
	switch syncer.Mode(s) {
	case syncer.ModeAddOnly, syncer.ModeMirrorDelete:
		return syncer.Mode(s), nil
	default:
		return "", fmt.Errorf("invalid mode %q (expected add-only or mirror-delete)", s)
	}
}

func stringOr(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}

func durationOr(s string, def time.Duration) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func intOr(n, def int) int {
	if n <= 0 {
		return def
	}
	return n
}
