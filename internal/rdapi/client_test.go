package rdapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListAllTorrentsPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			_ = json.NewEncoder(w).Encode([]Torrent{
				{ID: "1", Hash: "aaa", Filename: "A"},
				{ID: "2", Hash: "bbb", Filename: "B"},
			})
		case "2":
			_ = json.NewEncoder(w).Encode([]Torrent{
				{ID: "3", Hash: "ccc", Filename: "C"},
			})
		default:
			_ = json.NewEncoder(w).Encode([]Torrent{})
		}
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{
		BaseURL:        srv.URL,
		HTTPTimeout:    2 * time.Second,
		MaxRetries:     2,
		RetryBase:      1 * time.Millisecond,
		RetryMaxJitter: 0,
		PageLimit:      2,
	})

	all, err := client.ListAllTorrents(context.Background(), "token")
	if err != nil {
		t.Fatalf("ListAllTorrents failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 torrents, got %d", len(all))
	}
}
