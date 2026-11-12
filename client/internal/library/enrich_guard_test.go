package library

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReviewedEnrichmentProtectsLaterEdits(t *testing.T) {
	lib := openWritable(t, tagsFixture(t))
	ctx := context.Background()
	original, _, err := lib.Track(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.ApplyEnrichment(ctx, "b", EnrichmentFields{Artist: "Manual Artist"}); err != nil {
		t.Fatal(err)
	}
	changed, err := lib.ApplyEnrichmentUnchanged(ctx, original, EnrichmentFields{Title: "AI Song", Artist: "AI Artist", Lyrics: "wrong"})
	if changed || !errors.Is(err, ErrEnrichmentChanged) {
		t.Fatalf("changed=%v error=%v", changed, err)
	}
	current, _, _ := lib.Track(ctx, "b")
	if current.Artist != "Manual Artist" || current.Title != original.Title || current.Lyrics != "" {
		t.Fatal("review overwrote edit")
	}
	changed, err = lib.ApplyEnrichmentUnchanged(ctx, current, EnrichmentFields{Title: "Confirmed", Duration: 123 * time.Second})
	if !changed || err != nil {
		t.Fatalf("valid confirmation failed: %v", err)
	}
	current, _, _ = lib.Track(ctx, "b")
	if current.Title != "Confirmed" || current.Duration != 123*time.Second {
		t.Fatalf("saved=%+v", current)
	}
}
