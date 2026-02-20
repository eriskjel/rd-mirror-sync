package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.real-debrid.com/rest/1.0"

type Config struct {
	SrcToken string
	DstToken string

	BaseURL       string
	Mode          string // add-only | mirror-delete
	DryRun        bool
	Interval      time.Duration
	RunTimeout    time.Duration
	HTTPTimeout   time.Duration
	WriteDelay    time.Duration
	MaxRetries    int
	RetryBase     time.Duration
	RetryMaxJitter time.Duration
	PageLimit     int

	// Optional regex to protect destination-only items from deletion in mirror-delete mode.
	ProtectDstRegex string

	// Optional address for health/metrics server (e.g. ":8099"). Empty disables it.
	HealthAddr string
}

func Load() (Config, error) {
	cfg := Config{
		SrcToken: strings.TrimSpace(os.Getenv("SRC_RD_TOKEN")),
		DstToken: strings.TrimSpace(os.Getenv("DST_RD_TOKEN")),

		BaseURL:        getString("RD_API_BASE_URL", defaultBaseURL),
		Mode:           getString("MIRROR_MODE", "add-only"),
		DryRun:         getBool("DRY_RUN", true),
		Interval:       getDuration("SYNC_INTERVAL", 45*time.Second),
		RunTimeout:     getDuration("RUN_TIMEOUT", 10*time.Minute),
		HTTPTimeout:    getDuration("HTTP_TIMEOUT", 20*time.Second),
		WriteDelay:     getDuration("WRITE_DELAY", 250*time.Millisecond),
		MaxRetries:     getInt("MAX_RETRIES", 4),
		RetryBase:      getDuration("RETRY_BASE", 500*time.Millisecond),
		RetryMaxJitter: getDuration("RETRY_MAX_JITTER", 350*time.Millisecond),
		PageLimit:      getInt("PAGE_LIMIT", 250),

		ProtectDstRegex: getString("PROTECT_DST_REGEX", ""),
		HealthAddr:      getString("HEALTH_ADDR", ""),
	}

	if cfg.SrcToken == "" || cfg.DstToken == "" {
		return cfg, errors.New("SRC_RD_TOKEN and DST_RD_TOKEN are required")
	}
	if cfg.Mode != "add-only" && cfg.Mode != "mirror-delete" {
		return cfg, fmt.Errorf("invalid MIRROR_MODE=%q (expected add-only or mirror-delete)", cfg.Mode)
	}
	if cfg.Interval < 10*time.Second {
		return cfg, errors.New("SYNC_INTERVAL must be >= 10s")
	}
	if cfg.HTTPTimeout <= 0 {
		return cfg, errors.New("HTTP_TIMEOUT must be > 0")
	}
	if cfg.RunTimeout < 0 {
		return cfg, errors.New("RUN_TIMEOUT must be >= 0")
	}
	if cfg.MaxRetries < 1 {
		return cfg, errors.New("MAX_RETRIES must be >= 1")
	}
	if cfg.PageLimit < 1 {
		return cfg, errors.New("PAGE_LIMIT must be >= 1")
	}
	return cfg, nil
}

func getString(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func getBool(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func getInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err == nil {
		return d
	}
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return def
}
