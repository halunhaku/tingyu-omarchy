package enrich

import (
	"context"
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"testing"
)

func TestRecognizedNamesRemainEvenWhenProviderReturnsDifferentNames(t *testing.T) {
	original := library.Track{ID: "a", Title: "Original", Artist: "Original artist", Album: "Original album"}
	lib := newFakeLibrary(original)
	searcher := &fakeSearcher{hit: metadata.Candidate{Title: "Confirmed", Artist: "Other artist", Album: "Other album"}}
	service := newService(t, lib, &metadata.Enricher{Primary: searcher, Lyrics: &fakeLyrics{text: "[00:01]lyrics"}}, 1)
	names := metadata.AIResult{Title: "Confirmed", Artist: "Chosen artist", Album: "Chosen album"}
	result := service.EnrichRecognized(context.Background(), original, names)
	if result.Err != nil || !result.Changed {
		t.Fatalf("result=%+v", result)
	}
	saved := lib.tracks["a"]
	if saved.Title != names.Title || saved.Artist != names.Artist || saved.Album != names.Album || saved.Lyrics == "" {
		t.Fatalf("saved=%+v", saved)
	}
	if service.Stats().SessionUsed != 0 {
		t.Fatal("explicit AI apply consumed automatic quota")
	}
}
func TestRecognizedTrackKeepsExistingAssetsAndCancelledWriteDoesNotRun(t *testing.T) {
	original := library.Track{ID: "a", Title: "Original", Artist: "Artist", Album: "Album", Lyrics: "existing lyrics", CoverArtPath: "existing.jpg"}
	lib := newFakeLibrary(original)
	service := newService(t, lib, &metadata.Enricher{}, 1)
	result := service.EnrichRecognized(context.Background(), original, metadata.AIResult{Title: "Confirmed"})
	if result.Err != nil || lib.tracks["a"].Lyrics != original.Lyrics || lib.tracks["a"].CoverArtPath != original.CoverArtPath || lib.tracks["a"].Artist != original.Artist {
		t.Fatalf("result=%+v track=%+v", result, lib.tracks["a"])
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = service.EnrichRecognized(ctx, original, metadata.AIResult{Title: "Cancelled"})
	if result.Err == nil || lib.tracks["a"].Title != "Confirmed" {
		t.Fatal("cancelled request wrote data")
	}
}
