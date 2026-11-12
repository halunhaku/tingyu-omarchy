package accept

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/mpv"
	"github.com/halunhaku/tingyu/client/internal/player"
)

// m1Files is the playback set scripts/gen-m2-media.sh writes under m1/: the
// three formats the M1 plan names, with the FLAC at 96 kHz so a device that
// resamples shows up as a mismatch, plus a 2 s file for the end state.
var m1Files = []struct {
	format   string
	relative string
	rate     int
}{
	{"mp3", "m1/tone.mp3", 48000},
	{"flac", "m1/tone.flac", 96000},
	{"m4a", "m1/tone.m4a", 48000},
}

const (
	// m1Sample is how long one file plays while progress is measured: long
	// enough that the position says something about real time, short enough
	// to keep the phase near ten seconds.
	m1Sample = 3 * time.Second
	// m1AdvanceFloor is the slowest progress accepted as "playing". The exact
	// number is the evidence; this only separates playing from stalled.
	m1AdvanceFloor = 0.5
)

// audioParams is the part of mpv's audio-out-params the M1 record reports.
type audioParams struct {
	Samplerate int    `json:"samplerate"`
	Channels   string `json:"channels"`
	Format     string `json:"format"`
}

// runPlayback answers the M1 questions the M2 phases do not: which AO reached
// the device, at which sample rate and format, what hwdec is actually doing,
// how fast the position moves, and what a file leaves behind when it ends.
//
// Everything here is read through mpv properties (current-ao, audio-out-params,
// hwdec, hwdec-current), never scraped out of log lines: the AO line in the
// log is a human-readable summary, these are the values the engine itself uses.
func runPlayback(lib *mpv.Lib, mediaDir string, log func(string, ...any), report *Report) {
	// The AOInit counter is fed by mpv's "AO: [" info line, so these sessions
	// ask for info and keep everything but that line off the console.
	aoLog := func(format string, args ...any) {
		if message := fmt.Sprintf(format, args...); strings.Contains(message, "AO: [") {
			log("%s", strings.TrimSpace(message))
		}
	}
	backend := Check{ID: "m1-ao-backend", Name: "M1：AO 后端、采样率、hwdec 的实际取值（属性直读）", Data: map[string]any{
		"method": "mpv 属性 current-ao / audio-out-params / audio-device / hwdec / hwdec-current",
	}}
	perFile := map[string]any{}
	backend.Pass = true
	ran := 0

	for _, file := range m1Files {
		path, missing := requireMedia(mediaDir, filepath.FromSlash(file.relative))
		if missing != "" {
			backend.Note = "测试媒体缺失：" + missing
			continue
		}
		session, err := newSession(lib, nil, "info", nil, aoLog)
		if err != nil {
			backend.Note = "引擎创建失败：" + err.Error()
			backend.Pass = false
			continue
		}
		entry := map[string]any{"file": path, "expectedRate": file.rate}
		if err := session.play([]library.Track{localTrack(path)}); err != nil {
			entry["error"] = "起播失败：" + err.Error()
			backend.Pass = false
			perFile[file.format] = entry
			session.close()
			continue
		}
		if !waitPlaying(session, 10*time.Second) {
			entry["error"] = "没有进入播放状态"
			backend.Pass = false
			perFile[file.format] = entry
			session.close()
			continue
		}
		// current-ao only exists once the audio device is open, which is the
		// moment the file actually reaches the server.
		waitProperty(session.engine, "current-ao", 3*time.Second)
		ran++

		ao, _ := session.engine.PropertyString("current-ao")
		device, _ := session.engine.PropertyString("audio-device")
		hwdec, hwdecOK := session.engine.PropertyString("hwdec")
		currentHwdec, currentOK := session.engine.PropertyString("hwdec-current")
		codec, _ := session.engine.PropertyString("audio-codec-name")
		rawParams, paramsOK := session.engine.PropertyString("audio-out-params")

		var params audioParams
		paramsParsed := paramsOK && json.Unmarshal([]byte(rawParams), &params) == nil
		entry["ao"] = ao
		entry["audioDevice"] = device
		entry["audioOutParams"] = rawParams
		entry["sampleRate"] = params.Samplerate
		entry["channels"] = params.Channels
		entry["sampleFormat"] = params.Format
		entry["hwdec"] = hwdec
		entry["hwdecCurrentAvailable"] = currentOK
		entry["hwdecCurrent"] = currentHwdec
		entry["codec"] = codec

		startPosition := session.snapshot().Snapshot.Position
		started := time.Now()
		time.Sleep(m1Sample)
		endPosition := session.snapshot().Snapshot.Position
		wall := time.Since(started).Seconds()
		advanced := (endPosition - startPosition).Seconds()
		ratio := 0.0
		if wall > 0 {
			ratio = advanced / wall
		}
		entry["advancedSeconds"] = round2(advanced)
		entry["wallSeconds"] = round2(wall)
		entry["advancedPerSecond"] = round2(ratio)
		entry["aoInits"] = session.engine.AOInits()
		entry["underruns"] = session.engine.Underruns()

		// A real device backend, not the null/pcm sinks acceptance uses
		// elsewhere; the sample rate is the one the device runs at, so a
		// resampling device shows up here even when playback "works".
		realBackend := ao != "" && ao != "null" && ao != "pcm"
		rateMatches := paramsParsed && params.Samplerate == file.rate
		progressing := ratio > m1AdvanceFloor
		if !realBackend || !hwdecOK || !progressing || session.engine.AOInits() < 1 {
			backend.Pass = false
		}
		if !rateMatches {
			// Not a failure by itself: a device may legitimately resample. It
			// is recorded per file so the record can say which happened.
			entry["rateNote"] = fmt.Sprintf("设备采样率 %d，源 %d", params.Samplerate, file.rate)
		}
		log("m1 %-4s ao=%s rate=%dHz rateMatches=%v format=%s hwdec=%s hwdec-current-available=%v advance=%.2fs/%.2fs aoInits=%d",
			file.format, ao, params.Samplerate, rateMatches, params.Format,
			hwdec, currentOK, advanced, wall, session.engine.AOInits())
		perFile[file.format] = entry
		session.close()
	}

	backend.Data["files"] = perFile
	if ran == 0 && backend.Pass {
		backend.Skipped = true
		if backend.Note == "" {
			backend.Note = "测试媒体缺失：先跑 scripts/gen-m2-media.sh"
		}
	}
	report.Add(backend)

	// --- end of playback: a file that ends on its own ------------------
	end := Check{ID: "m1-end", Name: "M1：播放到结尾的状态（2 s 文件自己播完）", Data: map[string]any{}}
	path, missing := requireMedia(mediaDir, filepath.FromSlash("m1/short.flac"))
	if missing != "" {
		end.Skipped = true
		end.Note = missing
		report.Add(end)
		return
	}
	session, err := newSession(lib, nil, "info", nil, aoLog)
	if err != nil {
		end.Note = "引擎创建失败：" + err.Error()
		report.Add(end)
		return
	}
	defer session.close()
	// Repeat off, or the controller wraps the single-track queue the moment it
	// ends and the completed state is never stably observable.
	session.control.SetRepeat(player.RepeatOff)
	if err := session.play([]library.Track{localTrack(path)}); err != nil {
		end.Note = "起播失败：" + err.Error()
		report.Add(end)
		return
	}
	completed := waitUntil(20*time.Second, func() bool {
		return session.snapshot().Snapshot.Processing == mpv.Completed
	})
	final := session.snapshot()
	eofReached, eofOK := session.engine.PropertyString("eof-reached")
	end.Data["completed"] = completed
	end.Data["processing"] = final.Snapshot.Processing.String()
	end.Data["playing"] = final.Snapshot.Playing
	end.Data["paused"] = final.Snapshot.Paused
	end.Data["positionSeconds"] = round2(final.Snapshot.Position.Seconds())
	end.Data["durationSeconds"] = round2(final.Snapshot.Duration.Seconds())
	end.Data["eofReached"] = eofReached
	end.Data["eofReachedAvailable"] = eofOK
	end.Data["aoInits"] = session.engine.AOInits()
	end.Data["underruns"] = session.engine.Underruns()
	end.Data["failure"] = failureText(final.Failure)
	end.Pass = completed && !final.Snapshot.Playing && final.Snapshot.Duration > 0 && final.Failure == nil
	if !end.Pass {
		end.Note = fmt.Sprintf("completed=%v playing=%v duration=%.2fs failure=%s",
			completed, final.Snapshot.Playing, final.Snapshot.Duration.Seconds(), failureText(final.Failure))
	}
	log("m1 end: completed=%v playing=%v pos=%.2fs/%.2fs eof-reached=%s aoInits=%d underruns=%d",
		completed, final.Snapshot.Playing, final.Snapshot.Position.Seconds(),
		final.Snapshot.Duration.Seconds(), eofReached, session.engine.AOInits(), session.engine.Underruns())
	report.Add(end)
}

// waitProperty polls until mpv reports a non-empty value for name, or the
// timeout passes. Properties like current-ao exist only after the device opens.
func waitProperty(engine *mpv.Player, name string, timeout time.Duration) bool {
	return waitUntil(timeout, func() bool {
		value, ok := engine.PropertyString(name)
		return ok && value != ""
	})
}
