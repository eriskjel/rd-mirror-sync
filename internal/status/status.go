package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"rdmirrorsync/internal/syncer"
)

// State tracks the run history for a single destination.
type State struct {
	mu sync.RWMutex

	running       bool
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

func (s *State) snapshot(interval time.Duration) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	healthy := s.lastOK || s.running
	if !s.lastSuccessAt.IsZero() && interval > 0 && time.Since(s.lastSuccessAt) > 2*interval {
		healthy = false
	}
	return map[string]any{
		"healthy":         healthy,
		"running":         s.running,
		"last_run_at":     s.lastRunAt,
		"last_success_at": s.lastSuccessAt,
		"last_error":      s.lastError,
		"last_ok":         s.lastOK,
		"last_stats":      s.lastStats,
	}
}

// MultiState tracks run history for all destinations and serves /healthz and /metrics.
type MultiState struct {
	interval time.Duration
	names    []string // ordered for stable output
	states   map[string]*State
}

// NewMultiState creates a MultiState for the given destination names.
func NewMultiState(names []string, interval time.Duration) *MultiState {
	ms := &MultiState{
		interval: interval,
		names:    names,
		states:   make(map[string]*State, len(names)),
	}
	for _, n := range names {
		ms.states[n] = NewState()
	}
	return ms
}

// For returns the State for the given destination name.
func (ms *MultiState) For(name string) *State {
	return ms.states[name]
}

// Handler returns an http.Handler for /healthz and /metrics.
//
// GET /healthz         — all destinations; overall healthy = all healthy
// GET /healthz?dest=x  — single destination (same shape as overall, no "destinations" wrapper)
func (ms *MultiState) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		dest := r.URL.Query().Get("dest")
		if dest != "" {
			st, ok := ms.states[dest]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unknown destination"})
				return
			}
			_ = json.NewEncoder(w).Encode(st.snapshot(ms.interval))
			return
		}

		// All destinations.
		allHealthy := true
		dests := make(map[string]any, len(ms.names))
		for _, name := range ms.names {
			snap := ms.states[name].snapshot(ms.interval)
			dests[name] = snap
			if h, _ := snap["healthy"].(bool); !h {
				allHealthy = false
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"healthy":      allHealthy,
			"destinations": dests,
		})
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		for _, name := range ms.names {
			st := ms.states[name]
			st.mu.RLock()
			fmt.Fprintf(w, "rd_mirror_running{dest=%q} %d\n", name, boolToInt(st.running))
			fmt.Fprintf(w, "rd_mirror_last_run_ok{dest=%q} %d\n", name, boolToInt(st.lastOK))
			fmt.Fprintf(w, "rd_mirror_last_run_timestamp_seconds{dest=%q} %d\n", name, st.lastRunAt.Unix())
			fmt.Fprintf(w, "rd_mirror_last_success_timestamp_seconds{dest=%q} %d\n", name, st.lastSuccessAt.Unix())
			fmt.Fprintf(w, "rd_mirror_last_need_add{dest=%q} %d\n", name, st.lastStats.NeedAdd)
			fmt.Fprintf(w, "rd_mirror_last_need_delete{dest=%q} %d\n", name, st.lastStats.NeedDelete)
			fmt.Fprintf(w, "rd_mirror_last_added{dest=%q} %d\n", name, st.lastStats.Added)
			fmt.Fprintf(w, "rd_mirror_last_deleted{dest=%q} %d\n", name, st.lastStats.Deleted)
			fmt.Fprintf(w, "rd_mirror_last_add_errors{dest=%q} %d\n", name, st.lastStats.AddErrors)
			fmt.Fprintf(w, "rd_mirror_last_delete_errors{dest=%q} %d\n", name, st.lastStats.DeleteErrors)
			st.mu.RUnlock()
		}
	})

	return mux
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
