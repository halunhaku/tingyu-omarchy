package metadata

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQQLyricsDecodesTextAndBase64(t *testing.T) {
	for _, encoded := range []bool{false, true} {
		t.Run(fmt.Sprint(encoded), func(t *testing.T) {
			text := "[00:01.00]測試 &amp; 歌詞"
			raw := text
			if encoded {
				raw = base64.StdEncoding.EncodeToString([]byte(text))
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("songmid") != "mid" || r.Header.Get("Referer") == "" {
					t.Error("missing song id/headers")
				}
				fmt.Fprintf(w, `{"code":0,"lyric":%q}`, raw)
			}))
			defer server.Close()
			qq := NewQQMusic(&HTTP{Client: server.Client()})
			qq.LyricsEndpoint = server.URL
			text, err := qq.Lyrics(context.Background(), LyricsQuery{}, &Candidate{SourceID: "mid"})
			if err != nil || text != "[00:01.00]测试 & 歌词" {
				t.Fatalf("text=%q err=%v", text, err)
			}
		})
	}
}
func TestLyricBrowserKeepsOtherSourcesAndRanksRecording(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/broken":
			w.WriteHeader(503)
		case "/search":
			fmt.Fprint(w, `[{"id":1,"trackName":"Song Live","artistName":"Other","albumName":"Live","duration":250,"syncedLyrics":"[00:01]other"},{"id":2,"trackName":"Song","artistName":"Artist","albumName":"Album","duration":200,"plainLyrics":"right song"},{"id":3,"trackName":"Song","instrumental":true,"plainLyrics":"instrumental"}]`)
		}
	}))
	defer server.Close()
	httpClient := &HTTP{Client: server.Client()}
	lrc := NewLRCLIB(httpClient)
	lrc.SearchEndpoint = server.URL + "/search"
	qq := NewQQMusic(httpClient)
	qq.Endpoint = server.URL + "/broken"
	browser := &LyricBrowser{LRCLIB: lrc, Sources: []LyricSource{{Searcher: qq, Lyrics: qq}}}
	query := LyricsQuery{Title: "Song", Artist: "Artist", Album: "Album", Duration: 200 * time.Second}
	hits, failures := browser.Search(context.Background(), query)
	if len(hits) != 2 || len(failures) != 1 || failures[0] != "qqmusic" || hits[0].Candidate.SourceID != "2" || !strings.Contains(hits[1].Note, "歌手不同") {
		t.Fatalf("hits=%+v errors=%v", hits, failures)
	}
	text, err := browser.Preview(context.Background(), query, hits[0])
	if err != nil || text != "right song" {
		t.Fatalf("preview=%q err=%v", text, err)
	}
}

type lyricTestSource struct {
	hits    []Candidate
	fetched []string
}

func (s *lyricTestSource) Name() string { return "test" }
func (s *lyricTestSource) Search(context.Context, SearchQuery, int) ([]Candidate, error) {
	return s.hits, nil
}
func (s *lyricTestSource) Lyrics(ctx context.Context, q LyricsQuery, c *Candidate) (string, error) {
	s.fetched = append(s.fetched, c.SourceID)
	return "lyrics", nil
}
func TestExtraAutomaticLyricsSkipsWrongSingerVersionAndDuration(t *testing.T) {
	source := &lyricTestSource{hits: []Candidate{
		{Title: "Song", Artist: "Cover singer", SourceID: "wrong-singer", Duration: 200 * time.Second},
		{Title: "Song Live", Artist: "Artist", SourceID: "wrong-version", Duration: 200 * time.Second},
		{Title: "Song", Artist: "Artist", SourceID: "wrong-duration", Duration: 250 * time.Second},
		{Title: "Song", Artist: "Artist", SourceID: "right", Duration: 200 * time.Second},
	}}
	e := &Enricher{MoreLyrics: []LyricSource{{Searcher: source, Lyrics: source}}}
	text, err := e.FetchLyrics(context.Background(), LyricsQuery{Title: "Song", Artist: "Artist", Duration: 200 * time.Second})
	if err != nil || text != "lyrics" || len(source.fetched) != 1 || source.fetched[0] != "right" {
		t.Fatalf("text=%q fetched=%v error=%v", text, source.fetched, err)
	}
}
