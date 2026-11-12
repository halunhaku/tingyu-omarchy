package metadata

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LyricSource struct {
	Searcher Searcher
	Lyrics   LyricsProvider
}
type LyricCandidate struct {
	Candidate Candidate
	Text      string
	Note      string
	Score     int
}
type LyricBrowser struct {
	LRCLIB  *LRCLIB
	Sources []LyricSource
}

func (l *LRCLIB) SearchLyrics(ctx context.Context, query LyricsQuery, limit int) ([]LyricCandidate, error) {
	endpoint := l.SearchEndpoint
	if endpoint == "" {
		endpoint = "https://lrclib.net/api/search"
		if l.Endpoint != "" {
			u, err := url.Parse(l.Endpoint)
			if err == nil {
				u.Path, u.RawQuery = "/api/search", ""
				endpoint = u.String()
			}
		}
	}
	params := url.Values{"track_name": {query.Title}}
	if !IsPlaceholderArtist(query.Artist) {
		params.Set("artist_name", query.Artist)
	}
	client := l.HTTP
	if client == nil {
		client = &HTTP{}
	}
	value, err := client.getJSONValue(ctx, l.Name(), endpoint, params, nil)
	if err != nil {
		return nil, err
	}
	entries, ok := value.([]any)
	if !ok {
		return nil, errors.New("invalid LRCLIB search reply")
	}
	out := []LyricCandidate{}
	for _, entry := range entries {
		hit := mapValue(entry)
		if instrumental, _ := hit["instrumental"].(bool); instrumental {
			continue
		}
		text := lrclibText(hit)
		if text == "" {
			continue
		}
		candidate := Candidate{Provider: l.Name(), Title: stringValue(hit["trackName"]), Artist: stringValue(hit["artistName"]), Album: stringValue(hit["albumName"]), Duration: time.Duration(numberValue(hit["duration"]) * float64(time.Second)), SourceID: strconv.FormatInt(int64(numberValue(hit["id"])), 10)}
		out = append(out, LyricCandidate{Candidate: candidate, Text: text})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Search queries each source independently: a dead source cannot hide results
// returned by another. Lyrics from QQ/NetEase are loaded only on selection.
func (b *LyricBrowser) Search(ctx context.Context, query LyricsQuery) ([]LyricCandidate, []string) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := []LyricCandidate{}
	failures := []string{}
	collect := func(source string, rows []LyricCandidate, err error) {
		mu.Lock()
		defer mu.Unlock()
		out = append(out, rows...)
		if err != nil {
			failures = append(failures, source)
		}
	}
	if b.LRCLIB != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := b.LRCLIB.SearchLyrics(ctx, query, 12)
			collect("lrclib", rows, err)
		}()
	}
	for _, source := range b.Sources {
		if source.Searcher == nil {
			continue
		}
		wg.Add(1)
		go func(source LyricSource) {
			defer wg.Done()
			hits, err := source.Searcher.Search(ctx, SearchQuery{Title: query.Title, Artist: query.Artist}, 12)
			rows := []LyricCandidate{}
			for _, hit := range hits {
				rows = append(rows, LyricCandidate{Candidate: hit})
			}
			collect(source.Searcher.Name(), rows, err)
		}(source)
	}
	wg.Wait()
	for i := range out {
		out[i].Score, out[i].Note = lyricMatchScore(query, out[i].Candidate)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		a, c := out[i].Candidate, out[j].Candidate
		return a.Provider+"/"+a.SourceID < c.Provider+"/"+c.SourceID
	})
	sort.Strings(failures)
	return out, failures
}
func (b *LyricBrowser) Preview(ctx context.Context, query LyricsQuery, hit LyricCandidate) (string, error) {
	if hit.Text != "" {
		return hit.Text, nil
	}
	for _, source := range b.Sources {
		if source.Lyrics != nil && source.Lyrics.Name() == hit.Candidate.Provider {
			return source.Lyrics.Lyrics(ctx, query, &hit.Candidate)
		}
	}
	return "", errors.New("unknown lyric source")
}
func lyricMatchScore(query LyricsQuery, hit Candidate) (int, string) {
	score := 0
	notes := []string{}
	if lyricIdentity(query.Title) == lyricIdentity(hit.Title) {
		score += 100
	} else {
		notes = append(notes, "歌名或版本不同")
	}
	if !IsPlaceholderArtist(query.Artist) {
		if lyricIdentity(query.Artist) == lyricIdentity(hit.Artist) {
			score += 80
		} else {
			notes = append(notes, "歌手不同")
		}
	}
	if query.Duration > 0 && hit.Duration > 0 {
		delta := query.Duration - hit.Duration
		if delta < 0 {
			delta = -delta
		}
		if delta <= 5*time.Second {
			score += 40
		} else {
			notes = append(notes, "时长不同")
		}
	}
	if !IsPlaceholderAlbum(query.Album) && lyricIdentity(query.Album) == lyricIdentity(hit.Album) {
		score += 10
	}
	if len(notes) == 0 {
		return score, "请预览确认歌词版本"
	}
	return score, strings.Join(notes, "；")
}

func findExtraLyrics(ctx context.Context, sources []LyricSource, query LyricsQuery) (string, string, error) {
	var firstErr error
	for _, source := range sources {
		if source.Searcher == nil || source.Lyrics == nil {
			continue
		}
		hits, err := source.Searcher.Search(ctx, SearchQuery{Title: query.Title, Artist: query.Artist}, 8)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, hit := range hits {
			// Automatic acceptance requires both song and artist identity, plus a
			// close duration when known. Manual search can display other versions.
			if IsPlaceholderArtist(query.Artist) || lyricIdentity(hit.Title) != lyricIdentity(query.Title) || lyricIdentity(hit.Artist) != lyricIdentity(query.Artist) {
				continue
			}
			if query.Duration > 0 {
				delta := query.Duration - hit.Duration
				if delta < 0 {
					delta = -delta
				}
				if hit.Duration <= 0 || delta > 5*time.Second {
					continue
				}
			}
			text, err := source.Lyrics.Lyrics(ctx, query, &hit)
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if strings.TrimSpace(text) != "" {
				return text, source.Lyrics.Name(), nil
			}
		}
	}
	return "", "", firstErr
}
