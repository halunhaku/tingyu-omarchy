//go:build !darwin && !windows

package mpv

// defaultAudioOutput is empty elsewhere: mpv picks its own default (PulseAudio,
// PipeWire or ALSA on Linux). W0 does not target those platforms.
func defaultAudioOutput() string { return "" }
