package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/halunhaku/tingyu/client/internal/mpv"
)

// fakeMediaControl records what the media session asked the player to do.
type fakeMediaControl struct {
	calls []string
	seek  time.Duration
}

func (f *fakeMediaControl) Play() error     { f.calls = append(f.calls, "play"); return nil }
func (f *fakeMediaControl) Pause() error    { f.calls = append(f.calls, "pause"); return nil }
func (f *fakeMediaControl) Next() error     { f.calls = append(f.calls, "next"); return nil }
func (f *fakeMediaControl) Previous() error { f.calls = append(f.calls, "previous"); return nil }
func (f *fakeMediaControl) Seek(position time.Duration) error {
	f.calls = append(f.calls, "seek")
	f.seek = position
	return nil
}

// TestMediaCommandsReachTheController is the behavioural contract M1's S2
// item 3 checked against the ObjC bridge: every command the system sends
// must reach the matching controller call. The official API folds the
// play/pause key into play or pause by the state it was told, so those two
// are what arrive.
func TestMediaCommandsReachTheController(t *testing.T) {
	cases := []struct {
		command mygo.MediaCommand
		call    string
		name    string
	}{
		{mygo.MediaPlay, "play", "play"},
		{mygo.MediaPause, "pause", "pause"},
		{mygo.MediaStop, "pause", "stop"},
		{mygo.MediaNext, "next", "nextTrack"},
		{mygo.MediaPrevious, "previous", "previousTrack"},
		{mygo.MediaSeek, "seek", "changePlaybackPosition"},
	}
	for _, test := range cases {
		control := &fakeMediaControl{}
		var logged string
		handleMediaCommand(control, func(format string, args ...any) { logged = format }, test.command, 42*time.Second)
		if len(control.calls) != 1 || control.calls[0] != test.call {
			t.Errorf("%s: calls = %v, want one %q", test.name, control.calls, test.call)
		}
		if test.command == mygo.MediaSeek && control.seek != 42*time.Second {
			t.Errorf("seek went to %v, want 42s", control.seek)
		}
		if got := mediaCommandName(test.command); got != test.name {
			t.Errorf("name of %#x = %q, want %q", uint32(test.command), got, test.name)
		}
		if logged == "" {
			t.Errorf("%s: no log line", test.name)
		}
	}
	// A missing controller must be a no-op, not a panic: the app registers
	// commands before initSession, when a.control may still be nil.
	handleMediaCommand(nil, nil, mygo.MediaNext, 0)
}

// TestNowPlayingCommandsFollowQueueAndDuration pins the enabled command set:
// next/previous only when the queue has one, seek only when the duration is
// known.
func TestNowPlayingCommandsFollowQueueAndDuration(t *testing.T) {
	cases := []struct {
		name     string
		bounds   queueBounds
		duration time.Duration
		want     mygo.MediaCommand
	}{
		{"single track", queueBounds{}, 3 * time.Minute, mygo.MediaPlay | mygo.MediaPause | mygo.MediaSeek},
		{"unknown duration", queueBounds{}, 0, mygo.MediaPlay | mygo.MediaPause},
		{"queue both ways", queueBounds{hasNext: true, hasPrev: true}, time.Minute,
			mygo.MediaPlay | mygo.MediaPause | mygo.MediaSeek | mygo.MediaNext | mygo.MediaPrevious},
		{"last track", queueBounds{hasPrev: true}, time.Minute,
			mygo.MediaPlay | mygo.MediaPause | mygo.MediaSeek | mygo.MediaPrevious},
	}
	for _, test := range cases {
		if got := nowPlayingCommands(test.bounds, test.duration); got != test.want {
			t.Errorf("%s: commands = %#x, want %#x", test.name, uint32(got), uint32(test.want))
		}
	}
}

// TestSessionStateReportsTheEndAsStopped pins what MPRIS calls a finished
// queue: a player that ran to the end is Stopped, not Paused. A one-track
// queue with repeat off reported Paused at the last position forever before.
func TestSessionStateReportsTheEndAsStopped(t *testing.T) {
	cases := []struct {
		name     string
		snapshot mpv.Snapshot
		want     mygo.PlaybackState
	}{
		{"playing", mpv.Snapshot{Playing: true, Processing: mpv.Ready}, mygo.PlaybackPlaying},
		{"paused", mpv.Snapshot{Processing: mpv.Ready, Position: 5 * time.Second}, mygo.PlaybackPaused},
		{"loading", mpv.Snapshot{Processing: mpv.Loading}, mygo.PlaybackPaused},
		{"completed", mpv.Snapshot{Processing: mpv.Completed, Position: 14 * time.Second}, mygo.PlaybackStopped},
		{"at the last frame", mpv.Snapshot{Playing: true, Processing: mpv.Completed}, mygo.PlaybackPlaying},
	}
	for _, test := range cases {
		if got := sessionState(test.snapshot); got != test.want {
			t.Errorf("%s: state = %d, want %d", test.name, got, test.want)
		}
	}
}

// TestSessionPositionStartsANewTrackAtZero pins the position Now Playing
// publishes: a track change resets it (mpv still reports the old file's time
// for a moment), and no position is ever past the track's length.
func TestSessionPositionStartsANewTrackAtZero(t *testing.T) {
	cases := []struct {
		name         string
		snapshot     mpv.Snapshot
		duration     time.Duration
		trackChanged bool
		want         time.Duration
	}{
		{"new track", mpv.Snapshot{Position: 12 * time.Second}, 7 * time.Second, true, 0},
		{"same track keeps the position", mpv.Snapshot{Position: 5 * time.Second}, 7 * time.Second, false, 5 * time.Second},
		{"past the length", mpv.Snapshot{Position: 12 * time.Second}, 7 * time.Second, false, 7 * time.Second},
		{"unknown length", mpv.Snapshot{Position: 5 * time.Second}, 0, false, 5 * time.Second},
		{"negative", mpv.Snapshot{Position: -time.Second}, 7 * time.Second, false, 0},
	}
	for _, test := range cases {
		if got := sessionPosition(test.snapshot, test.duration, test.trackChanged); got != test.want {
			t.Errorf("%s: position = %v, want %v", test.name, got, test.want)
		}
	}
}
