package syncer

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"rdmirrorsync/internal/rdapi"
)

type Mode string

const (
	ModeAddOnly      Mode = "add-only"
	ModeMirrorDelete Mode = "mirror-delete"
)

type RunnerConfig struct {
	SrcToken string
	DstToken string

	Mode       Mode
	DryRun     bool
	WriteDelay time.Duration

	ProtectDstRegex string
}

type Stats struct {
	SourceCount int `json:"source_count"`
	DestCount   int `json:"dest_count"`

	NeedAdd    int `json:"need_add"`
	NeedDelete int `json:"need_delete"`

	Added         int `json:"added"`
	Deleted       int `json:"deleted"`
	AddErrors     int `json:"add_errors"`
	DeleteErrors  int `json:"delete_errors"`
	SkippedBadSrc int `json:"skipped_bad_src"`
	ProtectedDst  int `json:"protected_dst"`

	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

type API interface {
	ListAllTorrents(ctx context.Context, token string) ([]rdapi.Torrent, error)
	AddMagnetByHash(ctx context.Context, token, hash string) (string, error)
	SelectFilesAll(ctx context.Context, token, torrentID string) error
	DeleteTorrent(ctx context.Context, token, torrentID string) error
}

type Runner struct {
	api API
	cfg RunnerConfig
}

func NewRunner(api API, cfg RunnerConfig) *Runner {
	return &Runner{api: api, cfg: cfg}
}

func (r *Runner) RunOnce(ctx context.Context) (Stats, error) {
	stats := Stats{StartedAt: time.Now()}

	src, err := r.api.ListAllTorrents(ctx, r.cfg.SrcToken)
	if err != nil {
		return stats, err
	}
	dst, err := r.api.ListAllTorrents(ctx, r.cfg.DstToken)
	if err != nil {
		return stats, err
	}

	stats.SourceCount = len(src)
	stats.DestCount = len(dst)

	var protectRe *regexp.Regexp
	if r.cfg.ProtectDstRegex != "" {
		re, err := regexp.Compile(r.cfg.ProtectDstRegex)
		if err != nil {
			return stats, err
		}
		protectRe = re
	}

	srcByHash := make(map[string]rdapi.Torrent, len(src))
	dstByHash := make(map[string]rdapi.Torrent, len(dst))

	for _, t := range src {
		h := normalizeHash(t.Hash)
		if h == "" {
			stats.SkippedBadSrc++
			continue
		}
		srcByHash[h] = t
	}
	for _, t := range dst {
		h := normalizeHash(t.Hash)
		if h == "" {
			continue
		}
		dstByHash[h] = t
	}

	needAdd := make([]string, 0)
	for h := range srcByHash {
		if _, ok := dstByHash[h]; !ok {
			needAdd = append(needAdd, h)
		}
	}
	sort.Strings(needAdd)
	stats.NeedAdd = len(needAdd)

	needDelete := make([]string, 0)
	if r.cfg.Mode == ModeMirrorDelete {
		for h, dstT := range dstByHash {
			if _, ok := srcByHash[h]; ok {
				continue
			}
			if protectRe != nil && protectRe.MatchString(dstT.Filename) {
				stats.ProtectedDst++
				continue
			}
			needDelete = append(needDelete, h)
		}
		sort.Strings(needDelete)
	}
	stats.NeedDelete = len(needDelete)

	for _, h := range needAdd {
		srcT := srcByHash[h]
		if r.cfg.DryRun {
			log.Printf("[DRY_RUN] add hash=%s name=%q", h, srcT.Filename)
			continue
		}

		newID, err := r.api.AddMagnetByHash(ctx, r.cfg.DstToken, h)
		if err != nil {
			stats.AddErrors++
			log.Printf("add failed hash=%s name=%q err=%v", h, srcT.Filename, err)
			continue
		}
		if err := selectFilesWithRetry(ctx, r.api, r.cfg.DstToken, newID, 2*time.Second, 3, 3*time.Second); err != nil {
			stats.AddErrors++
			log.Printf("select files failed id=%s hash=%s name=%q err=%v", newID, h, srcT.Filename, err)
			continue
		}

		stats.Added++
		log.Printf("added hash=%s name=%q id=%s", h, srcT.Filename, newID)
		if r.cfg.WriteDelay > 0 {
			time.Sleep(r.cfg.WriteDelay)
		}
	}

	if r.cfg.Mode == ModeMirrorDelete {
		for _, h := range needDelete {
			dstT := dstByHash[h]
			if r.cfg.DryRun {
				log.Printf("[DRY_RUN] delete hash=%s name=%q id=%s", h, dstT.Filename, dstT.ID)
				continue
			}
			if dstT.ID == "" {
				stats.DeleteErrors++
				log.Printf("skip delete hash=%s name=%q empty id", h, dstT.Filename)
				continue
			}
			if err := r.api.DeleteTorrent(ctx, r.cfg.DstToken, dstT.ID); err != nil {
				stats.DeleteErrors++
				log.Printf("delete failed hash=%s name=%q id=%s err=%v", h, dstT.Filename, dstT.ID, err)
				continue
			}
			stats.Deleted++
			log.Printf("deleted hash=%s name=%q id=%s", h, dstT.Filename, dstT.ID)
			if r.cfg.WriteDelay > 0 {
				time.Sleep(r.cfg.WriteDelay)
			}
		}
	}

	stats.FinishedAt = time.Now()
	return stats, nil
}

// selectFilesWithRetry calls SelectFilesAll after an initial delay, then retries on failure.
// Real-Debrid often needs a few seconds after addMagnet before the torrent is ready for file selection.
func selectFilesWithRetry(ctx context.Context, api API, token, torrentID string, initialDelay time.Duration, maxAttempts int, retryDelay time.Duration) error {
	sleep := func(d time.Duration) error {
		timer := time.NewTimer(d)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}

	if err := sleep(initialDelay); err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			log.Printf("select files pending, waiting to retry id=%s attempt=%d/%d last_err=%v", torrentID, attempt+1, maxAttempts, lastErr)
			if err := sleep(retryDelay); err != nil {
				return err
			}
		}

		lastErr = api.SelectFilesAll(ctx, token, torrentID)
		if lastErr == nil {
			return nil
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
}

func normalizeHash(h string) string {
	return strings.ToLower(strings.TrimSpace(h))
}
