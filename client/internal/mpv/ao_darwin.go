//go:build darwin

package mpv

// defaultAudioOutput is the audio driver mpv uses on this platform. The
// client is built against media_kit's audio-only Mpv.framework on macOS,
// which has CoreAudio only.
func defaultAudioOutput() string { return "coreaudio" }
