package shell

import (
	"github.com/egoist/mygo/ui"
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"testing"
)

type lyricUIActions struct {
	metadataActions
	searches   int
	previews   int
	imports    int
	saves      int
	savedTrack library.Track
	text       string
}

func (a *lyricUIActions) SearchLyricCandidates(track library.Track, q metadata.LyricsQuery, id uint64) {
	a.searches++
}
func (a *lyricUIActions) LoadLyricPreview(q metadata.LyricsQuery, hit metadata.LyricCandidate, id uint64) {
	a.previews++
}
func (a *lyricUIActions) ImportLyricFile(id uint64) { a.imports++ }
func (a *lyricUIActions) SaveLyricText(track library.Track, text string, id uint64) {
	a.saves++
	a.savedTrack = track
	a.text = text
}
func TestLyricDialogPreviewsImportsAndSavesWithoutMetadataWrite(t *testing.T) {
	m, _ := newMetadataModel(t)
	a := &lyricUIActions{}
	m.SetMetadataActions(a)
	track := m.State().Library().Items()[0]
	m.State().OpenLyricSearch(track)
	m.State().SetLyricCandidates(m.State().LyricSearch().RequestID, []metadata.LyricCandidate{{Candidate: metadata.Candidate{Provider: "qqmusic", Title: "Song", Artist: "Artist"}, Note: "请预览确认"}}, "found")
	tt := ui.NewTester(m.View, DefaultWidth, DefaultHeight)
	frame(tt)
	for _, text := range []string{"搜索歌词", "QQ 音乐", "导入 LRC / TXT", "保存歌词"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %s", text)
		}
	}
	if err := tt.Click("预览歌词"); err != nil {
		t.Fatal(err)
	}
	if a.previews != 1 || a.saves != 0 {
		t.Fatal("preview saved data")
	}
	m.State().SetLyricText(m.State().LyricSearch().RequestID, "[00:01]new lyrics", "loaded")
	frame(tt)
	if err := tt.Click("导入 LRC / TXT"); err != nil {
		t.Fatal(err)
	}
	if a.imports != 1 {
		t.Fatal("import not called")
	}
	m.State().SetLyricText(m.State().LyricSearch().RequestID, "[00:01]imported lyrics", "loaded")
	frame(tt)
	if err := tt.Click("保存歌词"); err != nil {
		t.Fatal(err)
	}
	if a.saves != 1 || len(a.manuals) != 0 || a.text != "[00:01]imported lyrics" || a.savedTrack.ID != track.ID {
		t.Fatal("lyric save changed metadata path")
	}
}
