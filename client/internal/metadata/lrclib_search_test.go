package metadata

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLRCLIBSearchFallbackSelectsRecording(t *testing.T) {
	for _, status := range []int{200, 404, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			searched := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/get" {
					w.WriteHeader(status)
					fmt.Fprint(w, "{}")
					return
				}
				searched = true
				if r.URL.Query().Get("album_name") != "" || r.URL.Query().Get("duration") != "" {
					t.Error("search should not require exact album/duration")
				}
				fmt.Fprint(w, `[
     {"trackName":"一路向北","artistName":"翻唱歌手","duration":294,"syncedLyrics":"wrong artist"},
     {"trackName":"一路向北 (Live)","artistName":"周杰伦","duration":294,"syncedLyrics":"wrong version"},
     {"trackName":"一路向北","artistName":"周杰伦","duration":230,"syncedLyrics":"wrong duration"},
     {"trackName":"一路向北","artistName":"周杰倫","albumName":"合辑","duration":295,"syncedLyrics":"[00:01]風景"},
     {"trackName":"一路向北","artistName":"周杰伦","albumName":"其他","duration":294,"syncedLyrics":"[00:01]後退"}
    ]`)
			}))
			defer server.Close()
			p := &LRCLIB{HTTP: &HTTP{Client: server.Client()}, Endpoint: server.URL + "/get", SearchEndpoint: server.URL + "/search"}
			text, err := p.Lyrics(context.Background(), LyricsQuery{Title: "一路向北", Artist: "周杰伦", Album: "J III MP3 Player", Duration: 294 * time.Second}, nil)
			if err != nil || text != "[00:01]后退" || !searched {
				t.Fatalf("text=%q err=%v searched=%v", text, err, searched)
			}
		})
	}
}

func TestLRCLIBSearchRejectsUnrelatedRecordings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get" {
			w.WriteHeader(404)
			return
		}
		fmt.Fprint(w, `[
   {"trackName":"七里香","artistName":"其他歌手","duration":299,"syncedLyrics":"cover"},
   {"trackName":"七里香 Remix","artistName":"周杰伦","duration":299,"syncedLyrics":"remix"},
   {"trackName":"七里香","artistName":"周杰伦","duration":310,"syncedLyrics":"duration mismatch"},
   {"trackName":"七里香","artistName":"周杰伦","duration":299,"instrumental":true,"plainLyrics":"instrumental"}
  ]`)
	}))
	defer server.Close()
	p := &LRCLIB{HTTP: &HTTP{Client: server.Client()}, Endpoint: server.URL + "/get", SearchEndpoint: server.URL + "/search"}
	text, _ := p.Lyrics(context.Background(), LyricsQuery{Title: "七里香", Artist: "周杰伦", Duration: 299 * time.Second}, nil)
	if text != "" {
		t.Fatalf("accepted unrelated lyrics: %q", text)
	}
}

func TestLRCLIBRateLimitDoesNotTriggerSearch(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(429) }))
	defer server.Close()
	p := &LRCLIB{HTTP: &HTTP{Client: server.Client()}, Endpoint: server.URL + "/get", SearchEndpoint: server.URL + "/search"}
	_, err := p.Lyrics(context.Background(), LyricsQuery{Title: "Song", Artist: "Artist"}, nil)
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
