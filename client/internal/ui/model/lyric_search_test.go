package model

import (
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"github.com/halunhaku/tingyu/client/internal/player"
	"testing"
)

type lyricActionsFake struct {
	fakeMetadata
	searches   int
	previews   int
	imports    int
	saves      int
	query      metadata.LyricsQuery
	savedTrack library.Track
	savedText  string
}

func (a *lyricActionsFake) SearchLyricCandidates(track library.Track, q metadata.LyricsQuery, id uint64) {
	a.searches++
	a.query = q
}
func (a *lyricActionsFake) LoadLyricPreview(q metadata.LyricsQuery, hit metadata.LyricCandidate, id uint64) {
	a.previews++
}
func (a *lyricActionsFake) ImportLyricFile(id uint64) { a.imports++ }
func (a *lyricActionsFake) SaveLyricText(track library.Track, text string, id uint64) {
	a.saves++
	a.savedTrack = track
	a.savedText = text
}
func TestLyricsEditorSearchPreviewAndSaveOnlyLyrics(t *testing.T) {
	s := New(nil, nil, "", nil)
	a := &lyricActionsFake{}
	s.SetMetadataActions(a)
	original := library.Track{ID: "1", Title: "Original", Artist: "Artist", Album: "Album", Lyrics: "old"}
	s.SetPlayer(player.Update{Track: &original})
	s.OpenLyricSearch(original)
	id := s.LyricSearch().RequestID
	s.SetLyricCandidates(id, []metadata.LyricCandidate{{Candidate: metadata.Candidate{Provider: "qqmusic", Title: "Song"}}}, "")
	s.LyricSearch().Title = "Different search title"
	s.SearchLyrics()
	if a.query.Title != "Different search title" || a.saves != 0 {
		t.Fatal("query not editable or premature write")
	}
	s.SetLyricCandidates(s.LyricSearch().RequestID, []metadata.LyricCandidate{{Candidate: metadata.Candidate{Provider: "qqmusic", Title: "Song"}}}, "")
	s.PreviewLyricCandidate(0)
	previewID := s.LyricSearch().RequestID
	s.SaveLyrics()
	if a.saves != 0 {
		t.Fatal("saved while loading")
	}
	s.SetLyricText(previewID, "[00:01]new lyrics", "")
	s.SaveLyrics()
	if a.saves != 1 || a.savedTrack != original || a.savedText != "[00:01]new lyrics" {
		t.Fatal("save used edited search fields as metadata")
	}
	s.LyricsSaved(s.LyricSearch().RequestID, "saved", true)
	s.SetPlayer(player.Update{Track: &original})
	_, lyrics, ok := s.CurrentLyrics()
	if !ok || !lyrics.Synced || original.Lyrics != "old" {
		t.Fatal("saved lyric not refreshed or shared player data mutated")
	}
	s.CloseLyricSearch()
	s.OpenLyricSearch(original)
	if s.LyricSearch().Track.Lyrics != a.savedText {
		t.Fatal("reopened editor used stale player lyrics")
	}
	s.SetLyricText(previewID, "stale", "")
	if s.LyricSearch().Text == "stale" {
		t.Fatal("old callback changed reopened editor")
	}
}
func TestLyricsImportIsPreviewOnlyAndCanReplacePendingSearch(t *testing.T) {
	s := New(nil, nil, "", nil)
	a := &lyricActionsFake{}
	s.SetMetadataActions(a)
	s.OpenLyricSearch(library.Track{ID: "1", Title: "Song", Lyrics: "existing"})
	old := s.LyricSearch().RequestID
	s.ImportLyrics()
	if a.imports != 1 || a.saves != 0 {
		t.Fatal("import must only preview")
	}
	s.SetLyricCandidates(old, nil, "stale search")
	if !s.LyricSearch().Loading {
		t.Fatal("stale search affected import")
	}
	s.SetLyricText(s.LyricSearch().RequestID, "", "cancelled")
	if s.LyricSearch().Text != "existing" {
		t.Fatal("cancelled import lost draft")
	}
	s.LyricSearch().Text = "pasted plain lyrics"
	s.SaveLyrics()
	if a.savedText != "pasted plain lyrics" {
		t.Fatal("pasted text not saved")
	}
}
