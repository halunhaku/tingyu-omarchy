package metadata

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// LRCLIB is the primary lyrics source (lrclib_provider.dart): timed lyrics by
// title/artist/album/duration, no key, no account. It asks clients to send a
// recognisable User-Agent.
type LRCLIB struct {
	HTTP *HTTP
	// Endpoint overrides the public lookup URL (tests).
	Endpoint string
	// SearchEndpoint overrides the search fallback URL in tests.
	SearchEndpoint string
}

// LRCLIBEndpoint is the public get endpoint.
const LRCLIBEndpoint = "https://lrclib.net/api/get"

func (l *LRCLIB) endpoint() string {
	if strings.TrimSpace(l.Endpoint) != "" {
		return l.Endpoint
	}
	return LRCLIBEndpoint
}

// NewLRCLIB builds the provider.
func NewLRCLIB(client *HTTP) *LRCLIB { return &LRCLIB{HTTP: client} }

// Name is the provider id.
func (*LRCLIB) Name() string { return "lrclib" }

// Lyrics fetches a lyric document, preferring the synced LRC over plain text.
func (l *LRCLIB) Lyrics(ctx context.Context, query LyricsQuery, _ *Candidate) (string, error) {
	title := strings.TrimSpace(query.Title)
	if title == "" {
		return "", nil
	}
	params := url.Values{"track_name": {title}}
	if artist := strings.TrimSpace(query.Artist); artist != "" && !IsPlaceholderArtist(artist) {
		params.Set("artist_name", artist)
	}
	if album := strings.TrimSpace(query.Album); album != "" && !IsPlaceholderAlbum(album) {
		params.Set("album_name", album)
	}
	if query.Duration > 0 {
		params.Set("duration", strconv.Itoa(int(query.Duration/time.Second)))
	}
	http := l.HTTP
	if http == nil {
		http = &HTTP{}
	}
	body, err := http.getJSON(ctx, l.Name(), l.endpoint(), params, nil)
	if err == nil {
		if lyrics := lrclibText(body); lyrics != "" {
			return lyrics, nil
		}
	} else {
		var upstream *UpstreamError
		// Search can survive an unavailable exact lookup. Do not amplify rate
		// limits, cancellation or a network outage with another request.
		if !errors.As(err, &upstream) || (upstream.Status != 404 && upstream.Status < 500) {
			return "", err
		}
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	artist := strings.TrimSpace(query.Artist)
	if IsPlaceholderArtist(artist) {
		return "", err // A title alone cannot safely identify the recording.
	}
	endpoint := strings.TrimSpace(l.SearchEndpoint)
	if endpoint == "" {
		endpoint = "https://lrclib.net/api/search"
		if strings.TrimSpace(l.Endpoint) != "" {
			if custom, parseErr := url.Parse(l.Endpoint); parseErr == nil {
				custom.Path, custom.RawQuery = "/api/search", ""
				endpoint = custom.String()
			}
		}
	}
	value, searchErr := http.getJSONValue(ctx, l.Name(), endpoint, url.Values{
		"track_name": {title}, "artist_name": {artist},
	}, nil)
	if searchErr != nil {
		return "", searchErr
	}
	entries, ok := value.([]any)
	if !ok {
		return "", &UpstreamError{Provider: l.Name(), URL: endpoint, Err: errors.New("expected JSON array")}
	}
	best, bestScore := "", -1.0
	for _, entry := range entries {
		hit := mapValue(entry)
		if instrumental, _ := hit["instrumental"].(bool); instrumental {
			continue
		}
		if lyricIdentity(stringValue(hit["trackName"])) != lyricIdentity(title) ||
			lyricIdentity(stringValue(hit["artistName"])) != lyricIdentity(artist) {
			continue
		}
		text := lrclibText(hit)
		if text == "" {
			continue
		}
		score := 100.0
		if query.Duration > 0 {
			duration := numberValue(hit["duration"])
			delta := duration - query.Duration.Seconds()
			if delta < 0 {
				delta = -delta
			}
			if duration <= 0 || delta > 5 {
				continue
			}
			score -= delta * 10
		}
		if !IsPlaceholderAlbum(query.Album) && lyricIdentity(stringValue(hit["albumName"])) == lyricIdentity(query.Album) {
			score += 5
		}
		if strings.TrimSpace(stringValue(hit["syncedLyrics"])) != "" {
			score += 100
		}
		if score > bestScore {
			best, bestScore = text, score
		}
	}
	if best != "" {
		return best, nil
	}
	return "", err
}

// Ignore punctuation and script differences, but retain version words such as
// Live or Remix so a different recording is never accepted by substring.
func lyricIdentity(value string) string {
	value = (&OpenCC{}).ToSimplified(strings.ToLower(value))
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, value)
}

func lrclibText(body map[string]any) string {
	converter := &OpenCC{}
	if synced := strings.TrimSpace(stringValue(body["syncedLyrics"])); synced != "" {
		return converter.ToSimplified(synced)
	}
	if plain := strings.TrimSpace(stringValue(body["plainLyrics"])); plain != "" {
		return converter.ToSimplified(plain)
	}
	return ""
}
