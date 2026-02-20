package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"rdmirrorsync/internal/syncer"
)

type State struct {
	mu sync.RWMutex

	running bool

	lastRunAt     time.Time
	lastSuccessAt time.Time
	lastError     string
	lastOK        bool
	lastStats     syncer.Stats
}

func NewState() *State {
	return &State{}
}

func (s *State) MarkStart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	s.lastRunAt = time.Now()
}

func (s *State) MarkResult(stats syncer.Stats, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.lastStats = stats
	if err != nil {
		s.lastError = err.Error()
		s.lastOK = false
		return
	}
	s.lastError = ""
	s.lastOK = true
	s.lastSuccessAt = time.Now()
}

func (s *State) Handler(interval time.Duration) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		healthy := s.lastOK || s.running
		if !s.lastSuccessAt.IsZero() && interval > 0 && time.Since(s.lastSuccessAt) > 2*interval {
			healthy = false
		}
		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		resp := map[string]any{
			"healthy":         healthy,
			"running":         s.running,
			"last_run_at":     s.lastRunAt,
			"last_success_at": s.lastSuccessAt,
			"last_error":      s.lastError,
			"last_ok":         s.lastOK,
			"last_stats":      s.lastStats,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "rd_mirror_running %d\n", boolToInt(s.running))
		fmt.Fprintf(w, "rd_mirror_last_run_ok %d\n", boolToInt(s.lastOK))
		fmt.Fprintf(w, "rd_mirror_last_run_timestamp_seconds %d\n", s.lastRunAt.Unix())
		fmt.Fprintf(w, "rd_mirror_last_success_timestamp_seconds %d\n", s.lastSuccessAt.Unix())
		fmt.Fprintf(w, "rd_mirror_last_need_add %d\n", s.lastStats.NeedAdd)
		fmt.Fprintf(w, "rd_mirror_last_need_delete %d\n", s.lastStats.NeedDelete)
		fmt.Fprintf(w, "rd_mirror_last_added %d\n", s.lastStats.Added)
		fmt.Fprintf(w, "rd_mirror_last_deleted %d\n", s.lastStats.Deleted)
		fmt.Fprintf(w, "rd_mirror_last_add_errors %d\n", s.lastStats.AddErrors)
		fmt.Fprintf(w, "rd_mirror_last_delete_errors %d\n", s.lastStats.DeleteErrors)
	})

	return mux
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
