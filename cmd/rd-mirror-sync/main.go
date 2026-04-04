package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"rdmirrorsync/internal/config"
	"rdmirrorsync/internal/rdapi"
	"rdmirrorsync/internal/status"
	"rdmirrorsync/internal/syncer"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	api := rdapi.NewClient(rdapi.ClientConfig{
		BaseURL:        cfg.BaseURL,
		HTTPTimeout:    cfg.HTTPTimeout,
		MaxRetries:     cfg.MaxRetries,
		RetryBase:      cfg.RetryBase,
		RetryMaxJitter: cfg.RetryMaxJitter,
		PageLimit:      cfg.PageLimit,
	})

	names := make([]string, len(cfg.Destinations))
	for i, d := range cfg.Destinations {
		names[i] = d.Name
	}
	ms := status.NewMultiState(names, cfg.Interval)

	if cfg.HealthAddr != "" {
		go func() {
			log.Printf("health server listening on %s", cfg.HealthAddr)
			if err := http.ListenAndServe(cfg.HealthAddr, ms.Handler()); err != nil {
				log.Printf("health server stopped: %v", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	for _, dst := range cfg.Destinations {
		wg.Add(1)
		go func(dst config.Destination) {
			defer wg.Done()

			runner := syncer.NewRunner(api, syncer.RunnerConfig{
				SrcToken:        cfg.SrcToken,
				DstToken:        dst.Token,
				Mode:            dst.Mode,
				DryRun:          dst.DryRun,
				WriteDelay:      cfg.WriteDelay,
				ProtectDstRegex: dst.ProtectDstRegex,
			})

			st := ms.For(dst.Name)

			runOnce := func() {
				st.MarkStart()
				runCtx := ctx
				var cancel context.CancelFunc
				if cfg.RunTimeout > 0 {
					runCtx, cancel = context.WithTimeout(ctx, cfg.RunTimeout)
				} else {
					runCtx, cancel = context.WithCancel(ctx)
				}
				defer cancel()

				stats, err := runner.RunOnce(runCtx)
				st.MarkResult(stats, err)
				if err != nil {
					log.Printf("[%s] sync error: %v", dst.Name, err)
					return
				}
				elapsed := stats.FinishedAt.Sub(stats.StartedAt).Round(time.Millisecond)
				log.Printf(
					"[%s] sync done src=%d dst=%d need_add=%d need_delete=%d added=%d deleted=%d add_errors=%d delete_errors=%d elapsed=%s",
					dst.Name, stats.SourceCount, stats.DestCount,
					stats.NeedAdd, stats.NeedDelete, stats.Added, stats.Deleted,
					stats.AddErrors, stats.DeleteErrors, elapsed,
				)
			}

			log.Printf("[%s] starting (mode=%s dry_run=%v)", dst.Name, dst.Mode, dst.DryRun)
			runOnce()

			ticker := time.NewTicker(cfg.Interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					log.Printf("[%s] shutdown signal received; exiting", dst.Name)
					return
				case <-ticker.C:
					runOnce()
				}
			}
		}(dst)
	}

	wg.Wait()
}
