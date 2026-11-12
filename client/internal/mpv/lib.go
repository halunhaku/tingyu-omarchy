// Package mpv binds the libmpv C API in-process through purego, with no cgo.
//
// The binding is deliberately narrow: what a player needs to load files, keep a
// playlist, observe the properties a UI shows, and receive end-of-file and
// error events. It mirrors the parts of mpv/client.h the spike exercised
// (client API 2.1, mpv 0.36), and records the layout facts that cost the spike
// debugging time:
//
//   - RTLD_LAZY is mandatory: media_kit's Mpv.framework is an audio-only build
//     with unresolved libass symbols, so RTLD_NOW fails to load.
//   - Event ids are never switched on; names come from mpv_event_name.
//   - MPV_FORMAT_STRING's data is char**, not char*.
//   - mpv_get_property_string returns memory owned by mpv; copy before free.
package mpv

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// MPV_FORMAT_* values.
const (
	FormatNone   = 0
	FormatString = 1
	FormatFlag   = 3
	FormatInt64  = 4
	FormatDouble = 5
)

// mpv_end_file_reason (client.h: there is no 1; STOP is 2).
const (
	EndFileEOF      = 0
	EndFileStop     = 2
	EndFileQuit     = 3
	EndFileError    = 4
	EndFileRedirect = 5
)

// cEvent mirrors mpv_event (int32, int32, uint64, void* = 24 bytes).
type cEvent struct {
	EventID       int32
	Error         int32
	ReplyUserdata uint64
	Data          unsafe.Pointer
}

// cEventProperty mirrors mpv_event_property (name, format+padding, data).
type cEventProperty struct {
	Name   *byte
	Format int32
	_      int32
	Data   unsafe.Pointer
}

// cEventEndFile mirrors mpv_event_end_file for 0.36.
type cEventEndFile struct {
	Reason                   int32
	Error                    int32
	PlaylistEntryID          int64
	PlaylistInsertID         int64
	PlaylistInsertNumEntries int32
	_                        int32
}

// cEventHook mirrors mpv_event_hook.
type cEventHook struct {
	Name *byte
	ID   uint64
}

// cEventLogMessage mirrors mpv_event_log_message.
type cEventLogMessage struct {
	Prefix   *byte
	Level    *byte
	Text     *byte
	LogLevel int32
	_        int32
}

// Lib is a loaded libmpv and its symbols.
type Lib struct {
	handle uintptr

	create            func() unsafe.Pointer
	initialize        func(ctx unsafe.Pointer) int32
	destroy           func(ctx unsafe.Pointer)
	terminateDestroy  func(ctx unsafe.Pointer)
	setOptionString   func(ctx unsafe.Pointer, name, value *byte) int32
	setPropertyString func(ctx unsafe.Pointer, name, value *byte) int32
	setProperty       func(ctx unsafe.Pointer, name *byte, format int32, data unsafe.Pointer) int32
	getProperty       func(ctx unsafe.Pointer, name *byte, format int32, data unsafe.Pointer) int32
	getPropertyString func(ctx unsafe.Pointer, name *byte) unsafe.Pointer
	free              func(ptr unsafe.Pointer)
	command           func(ctx unsafe.Pointer, args **byte) int32
	waitEvent         func(ctx unsafe.Pointer, timeout float64) unsafe.Pointer
	observeProperty   func(ctx unsafe.Pointer, userdata uint64, name *byte, format int32) int32
	eventName         func(id int32) unsafe.Pointer
	errorString       func(code int32) unsafe.Pointer
	requestLog        func(ctx unsafe.Pointer, level *byte) int32
	hookAdd           func(ctx unsafe.Pointer, userdata uint64, name *byte, priority int32) int32
	hookContinue      func(ctx unsafe.Pointer, id uint64) int32
	clientAPIVersion  func() uint64
}

// Open loads libmpv at path. A process may load several handles, but each
// player creates its own core; tests load one.
//
// The path is validated before the loader sees it: it must be an absolute
// path to a regular file of this process's architecture, and every required
// mpv symbol must resolve. A missing file, a wrong-architecture DLL and a DLL
// without the mpv symbols therefore fail with a message naming the cause,
// instead of a bare "LoadLibrary failed" or a panic from purego.
func Open(path string) (*Lib, error) {
	if path == "" {
		return nil, fmt.Errorf("mpv: no libmpv path (set MPV_LIB or place libmpv-2.dll next to the executable)")
	}
	resolved := path
	if !acceptSoname(path) {
		// A bare SONAME (Linux system libmpv) is resolved by the dynamic
		// loader, not by the filesystem; everything else must be an absolute
		// path to a regular file.
		var err error
		resolved, err = validateLibrary(path)
		if err != nil {
			return nil, err
		}
	}
	handle, err := openLibrary(resolved)
	if err != nil {
		return nil, fmt.Errorf("mpv: load %s: %w", resolved, err)
	}
	lib := &Lib{handle: handle}
	symbols := []struct {
		name string
		dst  any
	}{
		{"mpv_create", &lib.create},
		{"mpv_initialize", &lib.initialize},
		{"mpv_destroy", &lib.destroy},
		{"mpv_terminate_destroy", &lib.terminateDestroy},
		{"mpv_set_option_string", &lib.setOptionString},
		{"mpv_set_property_string", &lib.setPropertyString},
		{"mpv_set_property", &lib.setProperty},
		{"mpv_get_property", &lib.getProperty},
		{"mpv_get_property_string", &lib.getPropertyString},
		{"mpv_free", &lib.free},
		{"mpv_command", &lib.command},
		{"mpv_wait_event", &lib.waitEvent},
		{"mpv_observe_property", &lib.observeProperty},
		{"mpv_event_name", &lib.eventName},
		{"mpv_error_string", &lib.errorString},
		{"mpv_request_log_messages", &lib.requestLog},
		{"mpv_hook_add", &lib.hookAdd},
		{"mpv_hook_continue", &lib.hookContinue},
		{"mpv_client_api_version", &lib.clientAPIVersion},
	}
	for _, symbol := range symbols {
		if err := registerLibFunc(symbol.dst, handle, symbol.name); err != nil {
			_ = closeLibrary(handle)
			return nil, fmt.Errorf("mpv: %s is not a usable libmpv (%w)", resolved, err)
		}
	}
	return lib, nil
}

// registerLibFunc turns purego's panic on a missing symbol into an error.
func registerLibFunc(dst any, handle uintptr, name string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("missing symbol %s: %v", name, recovered)
		}
	}()
	purego.RegisterLibFunc(dst, handle, name)
	return nil
}

// Close releases the library handle. It is safe twice; the caller must have
// destroyed every Player first.
func (l *Lib) Close() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	handle := l.handle
	l.handle = 0
	if err := closeLibrary(handle); err != nil {
		return fmt.Errorf("mpv: unload: %w", err)
	}
	return nil
}

// LibraryPath and ResolveLibrary live in the platform files: library_unix.go
// (macOS bundle, then the pub cache, then the Linux SONAMEs) and
// library_windows.go.

// ClientAPIVersion returns (major<<16)|minor.
func (l *Lib) ClientAPIVersion() uint64 { return l.clientAPIVersion() }

// ErrorString names an mpv error code.
func (l *Lib) ErrorString(code int32) string { return goString(l.errorString(code)) }

// EventName names an event id using the library's own table.
func (l *Lib) EventName(id int32) string { return goString(l.eventName(id)) }

// goString copies a NUL-terminated C string into Go memory.
func goString(p unsafe.Pointer) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(p, n)) != 0 {
		n++
		if n > 1<<20 {
			break
		}
	}
	return string(unsafe.Slice((*byte)(p), n))
}

// bp converts a Go string to a NUL-terminated C string. The returned pointer
// is only valid while the caller keeps the original slice alive
// (runtime.KeepAlive at the call site).
func bp(s string) *byte {
	if s == "" {
		return nil
	}
	b := make([]byte, len(s)+1)
	copy(b, s)
	return &b[0]
}

var (
	loadMu sync.Mutex
	loaded = map[string]*Lib{}
)

// Load returns a cached Lib for path, so repeated engine creation in tests
// does not dlopen the same image again.
func Load(path string) (*Lib, error) {
	loadMu.Lock()
	defer loadMu.Unlock()
	if lib, ok := loaded[path]; ok {
		return lib, nil
	}
	lib, err := Open(path)
	if err != nil {
		return nil, err
	}
	loaded[path] = lib
	return lib, nil
}
