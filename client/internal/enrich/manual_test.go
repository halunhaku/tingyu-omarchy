package enrich

import (
	"context"
	"testing"

	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
)

func TestManualPassAfterAutomaticQuotaExhausted(t *testing.T) {
	searcher := &fakeSearcher{}
	lib := newFakeLibrary(
		library.Track{ID: "a", Title: "Song One"},
		library.Track{ID: "b", Title: "Song Two"},
		library.Track{ID: "c", Title: "Song Three"},
	)
	service := newService(t, lib, &metadata.Enricher{Primary: searcher}, 2)
	ctx := context.Background()
	if got := service.EnrichTargets(ctx, 0, nil, nil); got.Attempted != 2 {
		t.Fatalf("automatic pass = %+v, want 2 attempts", got)
	}
	for i := 0; i < 2; i++ {
		var last Pass
		got := service.EnrichAllTargets(ctx, 0, func(p Pass) { last = p }, nil)
		if got.Attempted != 3 || !last.Finished || last.Stopped != "" {
			t.Fatalf("manual pass %d = %+v, progress %+v", i, got, last)
		}
		if used := service.Stats().SessionUsed; used != 2 {
			t.Fatalf("manual pass changed automatic quota to %d", used)
		}
	}
	if got := service.EnrichTargets(ctx, 0, nil, nil); got.Attempted != 0 {
		t.Fatalf("manual pass reset automatic quota: %+v", got)
	}
}

func TestManualPassAllowedWhenAutomaticDisabled(t *testing.T) {
	lib := newFakeLibrary(library.Track{ID: "a", Title: "Song"})
	service := newService(t, lib, &metadata.Enricher{Primary: &fakeSearcher{}}, -1)
	if got := service.EnrichTargets(context.Background(), 0, nil, nil); got.Attempted != 0 {
		t.Fatalf("automatic pass should be disabled: %+v", got)
	}
	if got := service.EnrichAllTargets(context.Background(), 0, nil, nil); got.Attempted != 1 {
		t.Fatalf("explicit manual pass should run: %+v", got)
	}
}

func TestManualPassPublishesCurrentBeforeNetworkAndCanCancel(t *testing.T) {
	searcher := &fakeSearcher{}
	lib := newFakeLibrary(library.Track{ID: "a", Title: "Song One"}, library.Track{ID: "b", Title: "Song Two"})
	service := newService(t, lib, &metadata.Enricher{Primary: searcher}, 1)
	var last Pass
	beforeNetwork := false
	cancel := false
	got := service.EnrichAllTargets(context.Background(), 0, func(p Pass) {
		last = p
		if p.Current != "" && p.Done == 0 && searcher.calls.Load() == 0 {
			beforeNetwork = true
		}
		if p.Done == 1 {
			cancel = true
		}
	}, func() bool { return cancel })
	if !beforeNetwork {
		t.Fatal("current track was not published before network request")
	}
	if got.Attempted != 1 || !last.Finished || last.Stopped != "cancelled" {
		t.Fatalf("cancelled pass = %+v, progress %+v", got, last)
	}
}
