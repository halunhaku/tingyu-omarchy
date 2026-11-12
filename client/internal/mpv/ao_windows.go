//go:build windows

package mpv

// defaultAudioOutput is the Windows audio driver. The shinchiro libmpv build
// ships wasapi (WASAPI shared/exclusive) as its native output; mpv's own
// default would also be wasapi, but naming it keeps the client deterministic.
func defaultAudioOutput() string { return "wasapi" }
