package library

import (
	"context"
	"testing"
)

func TestLyricsWritePreservesMetadataAndProtectsNewerLyrics(t *testing.T) {
	path := tagsFixture(t)
	lib := openWritable(t, path)
	ctx := context.Background()
	original, _, err := lib.Track(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	text := "[00:01]新的歌词"
	changed, err := lib.ReplaceLyrics(ctx, original.ID, text, original.Lyrics)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	updated, _, _ := lib.Track(ctx, original.ID)
	updated.Lyrics = original.Lyrics
	if updated != original {
		t.Fatal("lyric write changed metadata")
	}
	if changed, err := lib.ReplaceLyrics(ctx, original.ID, "stale replacement", original.Lyrics); err == nil || changed {
		t.Fatal("overwrote newer lyrics")
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, _, _ := reopened.Track(ctx, original.ID)
	if persisted.Lyrics != text {
		t.Fatal("lyrics not persisted")
	}
	if _, err := reopened.ReplaceLyrics(ctx, original.ID, "read-only", text); err == nil {
		t.Fatal("read-only save allowed")
	}
}
