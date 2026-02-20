package syncer

import (
	"context"
	"testing"

	"rdmirrorsync/internal/rdapi"
)

type fakeAPI struct {
	src []rdapi.Torrent
	dst []rdapi.Torrent

	added   []string
	deleted []string
}

func (f *fakeAPI) ListAllTorrents(_ context.Context, token string) ([]rdapi.Torrent, error) {
	if token == "src" {
		return f.src, nil
	}
	return f.dst, nil
}

func (f *fakeAPI) AddMagnetByHash(_ context.Context, _ string, hash string) (string, error) {
	f.added = append(f.added, hash)
	return "new-id-" + hash, nil
}

func (f *fakeAPI) SelectFilesAll(_ context.Context, _ string, _ string) error { return nil }

func (f *fakeAPI) DeleteTorrent(_ context.Context, _ string, torrentID string) error {
	f.deleted = append(f.deleted, torrentID)
	return nil
}

func TestRunOnceAddOnly(t *testing.T) {
	api := &fakeAPI{
		src: []rdapi.Torrent{
			{ID: "1", Hash: "A"},
			{ID: "2", Hash: "B"},
		},
		dst: []rdapi.Torrent{
			{ID: "3", Hash: "a"},
		},
	}
	r := NewRunner(api, RunnerConfig{
		SrcToken: "src",
		DstToken: "dst",
		Mode:     ModeAddOnly,
	})

	stats, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if stats.NeedAdd != 1 || stats.Added != 1 {
		t.Fatalf("unexpected add stats: %+v", stats)
	}
	if len(api.deleted) != 0 {
		t.Fatalf("did not expect deletes in add-only mode")
	}
}

func TestRunOnceMirrorDeleteWithProtection(t *testing.T) {
	api := &fakeAPI{
		src: []rdapi.Torrent{
			{ID: "1", Hash: "A", Filename: "Movie"},
		},
		dst: []rdapi.Torrent{
			{ID: "d1", Hash: "A", Filename: "Movie"},
			{ID: "d2", Hash: "B", Filename: "[KEEP] Local item"},
			{ID: "d3", Hash: "C", Filename: "Delete me"},
		},
	}
	r := NewRunner(api, RunnerConfig{
		SrcToken:        "src",
		DstToken:        "dst",
		Mode:            ModeMirrorDelete,
		ProtectDstRegex: `^\[KEEP\]`,
	})

	stats, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if stats.ProtectedDst != 1 {
		t.Fatalf("expected 1 protected destination item, got %d", stats.ProtectedDst)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "d3" {
		t.Fatalf("expected delete of d3 only, got %+v", api.deleted)
	}
}
