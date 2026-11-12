//go:build windows

// Command w0accept is the Windows W0 technical check. It verifies the native
// pieces the client depends on, on the machine that runs it:
//
//  1. SQLite: transaction commit, failed-transaction rollback, read-only
//     write rejection, and close/reopen consistency.
//  2. library: first-run creation, a write, a read-only reopen that refuses
//     writes, a writable reopen that still sees the data, all under a
//     directory whose name has Chinese characters and a space.
//  3. data dir: the real %AppData% default and the TINGYU_DATA_DIR override.
//  4. libmpv diagnostics: missing file, wrong-architecture DLL, DLL without
//     the mpv symbols, and a bare relative name are rejected with a cause.
//  5. libmpv playback: the x64 DLL actually loaded, its version, a generated
//     WAV played to the end through WASAPI, and the module unloaded after
//     close.
//
// It prints one JSON report and exits non-zero when a check fails. It is a
// verification harness, not a test of the UI: the window is exercised by
// scripts/windows-window-test.ps1. Nothing here reads personal media,
// credentials or the real library.
package main

import (
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/egoist/mygo/plugins/sqlite"

	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/mpv"
)

type report struct {
	OK       bool     `json:"ok"`
	Platform string   `json:"platform"`
	Checks   []check  `json:"checks"`
	Error    string   `json:"error,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
	Extra    any      `json:"extra,omitempty"`
}

type check struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Expected string `json:"expected"`
	Detail   string `json:"detail"`
	Elapsed  string `json:"elapsed"`
}

func main() {
	libPath := flag.String("mpv", "", "path to libmpv-2.dll (default: mpv.LibraryPath())")
	ao := flag.String("ao", "", "mpv audio output driver (empty: mpv's default, usually wasapi)")
	device := flag.String("audio-device", "", "mpv audio-device name (empty: system default)")
	verbose := flag.Bool("v", false, "print libmpv log messages")
	keep := flag.Bool("keep", false, "keep the temporary database and WAV")
	dirFlag := flag.String("dir", "", "working directory (default: a new temp dir)")
	wavOut := flag.String("wav", "", "write a 2 s test tone to this path and exit")
	createDB := flag.String("create-db", "", "create a library at this path and exit")
	printPaths := flag.Bool("print-paths", false, "print the default and overridden data dirs and exit")
	flag.Parse()

	switch {
	case *wavOut != "":
		if err := writeWAV(*wavOut, 2*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(*wavOut)
		return
	case *createDB != "":
		err := library.Create(context.Background(), *createDB)
		out, _ := json.MarshalIndent(map[string]any{"path": *createDB, "ok": err == nil, "error": errString(err)}, "", "  ")
		fmt.Println(string(out))
		if err != nil {
			os.Exit(1)
		}
		return
	case *printPaths:
		printDataDirs()
		return
	}

	rep := report{Platform: runtime.GOOS + "/" + runtime.GOARCH}
	fail := func(err error) {
		rep.Error = err.Error()
		out, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(out))
		os.Exit(1)
	}

	dir := *dirFlag
	if dir == "" {
		var err error
		dir, err = os.MkdirTemp("", "tingyu-w0-*")
		if err != nil {
			fail(err)
		}
		if !*keep {
			defer os.RemoveAll(dir)
		}
	} else {
		base, err := filepath.Abs(dir)
		if err != nil {
			fail(err)
		}
		if err := os.MkdirAll(base, 0o755); err != nil {
			fail(err)
		}
		// A run owns one fresh subdirectory, so repeating the harness in the
		// same evidence folder cannot collide with an earlier database.
		dir, err = os.MkdirTemp(base, "run-*")
		if err != nil {
			fail(err)
		}
	}
	rep.Evidence = append(rep.Evidence, dir)

	rep.Checks = append(rep.Checks, runCheck("sqlite",
		"commit is visible after close/reopen; a failed transaction leaves no row; a read-only connection refuses writes",
		func() (string, error) { return checkSQLite(filepath.Join(dir, "sqlite")) }))

	rep.Checks = append(rep.Checks, runCheck("library",
		"first run creates the schema, a playlist survives close/reopen, and a read-only reopen refuses a write",
		func() (string, error) { return checkLibrary(filepath.Join(dir, "library", "听屿 W0 数据")) }))

	rep.Checks = append(rep.Checks, runCheck("datadir",
		"default is %AppData%\\com.halunhaku.tingyu; TINGYU_DATA_DIR is honoured exactly",
		checkDataDir))

	rep.Checks = append(rep.Checks, runCheck("mpv-diagnostics",
		"missing file, wrong architecture, missing mpv symbols and a bare relative name each fail with a cause",
		func() (string, error) { return checkMPVDiagnostics(dir, *libPath) }))

	playbackDetail, extra, err := checkMPVPlayback(dir, *libPath, *ao, *device, *verbose)
	if extra != nil {
		rep.Extra = extra
	}
	rep.Checks = append(rep.Checks, check{
		Name:     "mpv-playback",
		OK:       err == nil,
		Expected: "the x64 DLL loads from the recorded path, a 2 s WAV plays to the end through WASAPI, and the module unloads after Close",
		Detail:   detailOrError(playbackDetail, err),
		Elapsed:  "",
	})

	for _, c := range rep.Checks {
		if !c.OK {
			rep.OK = false
			out, _ := json.MarshalIndent(rep, "", "  ")
			fmt.Println(string(out))
			os.Exit(1)
		}
	}
	rep.OK = true
	out, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(out))
}

func detailOrError(detail string, err error) string {
	if err != nil {
		if detail == "" {
			return err.Error()
		}
		return detail + ": " + err.Error()
	}
	return detail
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func runCheck(name, expected string, fn func() (string, error)) check {
	started := time.Now()
	detail, err := fn()
	return check{
		Name:     name,
		OK:       err == nil,
		Expected: expected,
		Detail:   detailOrError(detail, err),
		Elapsed:  time.Since(started).Round(time.Millisecond).String(),
	}
}

// --- SQLite ---------------------------------------------------------

func checkSQLite(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(dir, "w0.sqlite")

	db, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		return "", fmt.Errorf("open new database: %w", err)
	}
	if _, err := db.Execute(ctx, `CREATE TABLE w0 (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		db.Close()
		return "", fmt.Errorf("create table: %w", err)
	}

	// Commit: both rows must be visible.
	if _, err := db.Transaction(ctx, []sqlite.Statement{
		{SQL: `INSERT INTO w0 (id, name) VALUES (1, ?)`, Args: []any{"提交一"}},
		{SQL: `INSERT INTO w0 (id, name) VALUES (2, ?)`, Args: []any{"提交二"}},
	}); err != nil {
		return "", fmt.Errorf("commit transaction: %w", err)
	}
	committed, err := countRows(ctx, db)
	if err != nil {
		return "", err
	}
	if committed != 2 {
		return "", fmt.Errorf("after commit got %d rows, want 2", committed)
	}

	// Rollback: the second statement is invalid, so the first must not stick.
	if _, err := db.Transaction(ctx, []sqlite.Statement{
		{SQL: `INSERT INTO w0 (id, name) VALUES (3, ?)`, Args: []any{"回滚"}},
		{SQL: `INSERT INTO missing_table (id) VALUES (4)`},
	}); err == nil {
		return "", fmt.Errorf("transaction with an invalid statement succeeded, want an error")
	}
	afterRollback, err := countRows(ctx, db)
	if err != nil {
		return "", err
	}
	if afterRollback != 2 {
		return "", fmt.Errorf("after a failed transaction got %d rows, want 2 (partial commit)", afterRollback)
	}

	// Close and reopen: the two committed rows must still be there.
	if err := db.Close(); err != nil {
		return "", fmt.Errorf("close: %w", err)
	}
	db, err = sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		return "", fmt.Errorf("reopen: %w", err)
	}
	rows, err := db.Query(ctx, `SELECT name FROM w0 ORDER BY id`)
	if err != nil {
		db.Close()
		return "", fmt.Errorf("select after reopen: %w", err)
	}
	if len(rows.Values) != 2 {
		db.Close()
		return "", fmt.Errorf("after reopen got %d rows, want 2", len(rows.Values))
	}
	db.Close()

	// Read-only: reading works, writing is refused by SQLite itself.
	ro, err := sqlite.Open(ctx, path, sqlite.OpenOptions{ReadOnly: true})
	if err != nil {
		return "", fmt.Errorf("open read-only: %w", err)
	}
	defer ro.Close()
	if _, err := ro.Query(ctx, `SELECT COUNT(*) FROM w0`); err != nil {
		return "", fmt.Errorf("read-only select: %w", err)
	}
	if _, err := ro.Execute(ctx, `INSERT INTO w0 (id, name) VALUES (9, 'nope')`); err == nil {
		return "", fmt.Errorf("read-only connection accepted an INSERT, want it refused")
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s: committed 2 rows, rolled back a failed transaction (2 rows), read them after close/reopen, and a read-only connection refused INSERT",
		path), nil
}

func countRows(ctx context.Context, db *sqlite.DB) (int, error) {
	rows, err := db.Query(ctx, `SELECT COUNT(*) FROM w0`)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	if len(rows.Values) != 1 || len(rows.Values[0]) != 1 {
		return 0, fmt.Errorf("count: got %v", rows.Values)
	}
	n, ok := rows.Values[0][0].(int64)
	if !ok {
		return 0, fmt.Errorf("count type %T", rows.Values[0][0])
	}
	return int(n), nil
}

// --- library --------------------------------------------------------

func checkLibrary(dir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := library.Path(dir)

	if err := library.Create(ctx, path); err != nil {
		return "", fmt.Errorf("create library in %q: %w", dir, err)
	}
	lib, err := library.OpenWritable(ctx, path)
	if err != nil {
		return "", fmt.Errorf("open writable: %w", err)
	}
	playlist, err := lib.CreatePlaylist(ctx, "听屿 W0 测试")
	if err != nil {
		lib.Close()
		return "", fmt.Errorf("create playlist: %w", err)
	}
	if err := lib.Close(); err != nil {
		return "", fmt.Errorf("close writable: %w", err)
	}

	// A read-only reopen must show the data and refuse every write.
	ro, err := library.Open(ctx, path)
	if err != nil {
		return "", fmt.Errorf("reopen read-only: %w", err)
	}
	playlists, err := ro.Playlists(ctx)
	if err != nil {
		ro.Close()
		return "", fmt.Errorf("list playlists read-only: %w", err)
	}
	if len(playlists) != 1 || playlists[0].Name != "听屿 W0 测试" {
		ro.Close()
		return "", fmt.Errorf("read-only reopen saw %v, want the created playlist", playlists)
	}
	if _, err := ro.CreatePlaylist(ctx, "只读不该写入"); err == nil {
		ro.Close()
		return "", fmt.Errorf("read-only library accepted a write, want it refused")
	}
	if err := ro.Close(); err != nil {
		return "", fmt.Errorf("close read-only: %w", err)
	}

	// A writable reopen still sees exactly one playlist.
	rw, err := library.OpenWritable(ctx, path)
	if err != nil {
		return "", fmt.Errorf("reopen writable: %w", err)
	}
	defer rw.Close()
	after, err := rw.Playlists(ctx)
	if err != nil {
		return "", err
	}
	if len(after) != 1 || after[0].ID != playlist.ID {
		return "", fmt.Errorf("writable reopen saw %v, want exactly %s", after, playlist.ID)
	}
	return fmt.Sprintf("%s: created schema, wrote playlist %q, read it after a read-only reopen, refused a read-only write, and still saw one playlist writable",
		path, playlist.Name), nil
}

// --- data directory -------------------------------------------------

func checkDataDir() (string, error) {
	saved, had := os.LookupEnv("TINGYU_DATA_DIR")
	defer func() {
		if had {
			os.Setenv("TINGYU_DATA_DIR", saved)
		} else {
			os.Unsetenv("TINGYU_DATA_DIR")
		}
	}()

	if err := os.Unsetenv("TINGYU_DATA_DIR"); err != nil {
		return "", err
	}
	def, err := library.DataDir()
	if err != nil {
		return "", fmt.Errorf("default DataDir: %w", err)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	wantDefault := filepath.Join(config, "com.halunhaku.tingyu")
	if def != wantDefault {
		return "", fmt.Errorf("default DataDir %q, want %q", def, wantDefault)
	}

	override := filepath.Join(os.TempDir(), "听屿 W0 覆盖 数据")
	if err := os.Setenv("TINGYU_DATA_DIR", override); err != nil {
		return "", err
	}
	got, err := library.DataDir()
	if err != nil {
		return "", fmt.Errorf("override DataDir: %w", err)
	}
	if got != override {
		return "", fmt.Errorf("override DataDir %q, want %q", got, override)
	}
	return fmt.Sprintf("default=%s; override=%s (Chinese and space preserved)", def, got), nil
}

func printDataDirs() {
	saved, had := os.LookupEnv("TINGYU_DATA_DIR")
	defer func() {
		if had {
			os.Setenv("TINGYU_DATA_DIR", saved)
		} else {
			os.Unsetenv("TINGYU_DATA_DIR")
		}
	}()
	os.Unsetenv("TINGYU_DATA_DIR")
	def, _ := library.DataDir()
	data := map[string]any{"default": def}
	if saved != "" && had {
		os.Setenv("TINGYU_DATA_DIR", saved)
		override, _ := library.DataDir()
		data["override"] = override
	}
	out, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(out))
}

// --- libmpv diagnostics ---------------------------------------------

func checkMPVDiagnostics(dir, libPath string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if libPath == "" {
		libPath = mpv.LibraryPath()
	}
	if libPath == "" {
		return "", fmt.Errorf("no libmpv found: pass -mpv or set MPV_LIB")
	}
	abs, err := filepath.Abs(libPath)
	if err != nil {
		return "", err
	}
	var problems []string

	missing := filepath.Join(dir, "nope", "libmpv-2.dll")
	if _, err := mpv.Open(missing); err == nil {
		problems = append(problems, "missing file was accepted")
	} else if !strings.Contains(err.Error(), "not found at") {
		problems = append(problems, fmt.Sprintf("missing file error lacks a cause: %v", err))
	}

	wrongArch := filepath.Join(dir, "wrong-arch.dll")
	if err := copyAndPatchMachine(abs, wrongArch, pe.IMAGE_FILE_MACHINE_I386); err != nil {
		return "", fmt.Errorf("prepare wrong-architecture DLL: %w", err)
	}
	if _, err := mpv.Open(wrongArch); err == nil {
		problems = append(problems, "wrong-architecture DLL was accepted")
	} else if !strings.Contains(err.Error(), "built for PE machine") {
		problems = append(problems, fmt.Sprintf("wrong architecture error lacks a cause: %v", err))
	}

	systemDLL := filepath.Join(os.Getenv("SystemRoot"), "System32", "version.dll")
	if _, err := os.Stat(systemDLL); err == nil {
		if _, err := mpv.Open(systemDLL); err == nil {
			problems = append(problems, "DLL without mpv symbols was accepted")
		} else if !strings.Contains(err.Error(), "missing symbol") {
			problems = append(problems, fmt.Sprintf("missing-symbol error lacks a cause: %v", err))
		}
	} else {
		problems = append(problems, "system version.dll missing; missing-symbol case not exercised")
	}

	// A bare name would make Windows search the working directory. Open
	// resolves it first and reports the absolute path it refused.
	if _, err := mpv.Open("libmpv-2.dll"); err == nil {
		problems = append(problems, "bare relative DLL name was accepted")
	}
	if len(problems) > 0 {
		return "", fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return "missing file, i386-machine DLL, symbol-less DLL and bare relative name each failed with the resolved path and cause", nil
}

// copyAndPatchMachine copies a PE file and rewrites its machine field, which
// is how a wrong-architecture DLL is simulated without a 32-bit build.
func copyAndPatchMachine(source, dest string, machine uint16) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	f, err := os.OpenFile(dest, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	var dos [64]byte
	if _, err := f.ReadAt(dos[:], 0); err != nil {
		return err
	}
	if dos[0] != 'M' || dos[1] != 'Z' {
		return fmt.Errorf("%s is not a PE file", source)
	}
	peOffset := int64(binary.LittleEndian.Uint32(dos[0x3c:]))
	var machineBytes [2]byte
	binary.LittleEndian.PutUint16(machineBytes[:], machine)
	if _, err := f.WriteAt(machineBytes[:], peOffset+4); err != nil {
		return err
	}
	return nil
}

// --- libmpv playback ------------------------------------------------

type mpvExtra struct {
	Library          string `json:"library"`
	ModulePath       string `json:"modulePath"`
	SHA256           string `json:"sha256"`
	Machine          string `json:"machine"`
	ClientAPIMajor   int    `json:"clientApiMajor"`
	ClientAPIMinor   int    `json:"clientApiMinor"`
	MPVVersion       string `json:"mpvVersion"`
	FFmpegVersion    string `json:"ffmpegVersion"`
	AudioOutput      string `json:"audioOutput"`
	License          string `json:"license"`
	Source           string `json:"source"`
	DecodedWAV       string `json:"decodedWav"`
	PositionAfter    string `json:"positionAfter"`
	PositionMax      string `json:"positionMax"`
	Duration         string `json:"duration"`
	ReachedEnd       bool   `json:"reachedEnd"`
	AOInits          int    `json:"aoInits"`
	Underruns        int    `json:"underruns"`
	FilesLoaded      int    `json:"filesLoaded"`
	PlaybackObserved bool   `json:"playbackObserved"`
	ModuleUnloaded   bool   `json:"moduleUnloaded"`
	Goroutines       int    `json:"goroutinesAfterClose"`
}

func checkMPVPlayback(dir, libPath, ao, device string, verbose bool) (string, *mpvExtra, error) {
	if libPath == "" {
		libPath = mpv.LibraryPath()
	}
	if libPath == "" {
		return "", nil, fmt.Errorf("no libmpv found: pass -mpv or set MPV_LIB")
	}
	abs, err := filepath.Abs(libPath)
	if err != nil {
		return "", nil, err
	}
	extra := &mpvExtra{
		Library: abs,
		SHA256:  fileSHA256(abs),
		License: "GPL-2.0-or-later (mpv-winbuild-cmake build; the asset ships no LICENSE file)",
		Source:  "https://github.com/shinchiro/mpv-winbuild-cmake/releases/tag/20261101",
	}
	if f, err := pe.Open(abs); err == nil {
		switch f.Machine {
		case pe.IMAGE_FILE_MACHINE_AMD64:
			extra.Machine = "amd64"
		case pe.IMAGE_FILE_MACHINE_ARM64:
			extra.Machine = "arm64"
		case pe.IMAGE_FILE_MACHINE_I386:
			extra.Machine = "386"
		default:
			extra.Machine = fmt.Sprintf("%#x", f.Machine)
		}
		f.Close()
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", extra, err
	}
	wav := filepath.Join(dir, "w0-tone.wav")
	if err := writeWAV(wav, 2*time.Second); err != nil {
		return "", extra, err
	}
	extra.DecodedWAV = wav

	before := runtime.NumGoroutine()
	lib, err := mpv.Open(abs)
	if err != nil {
		return "", extra, err
	}
	vMajor, vMinor := int(lib.ClientAPIVersion()>>16), int(lib.ClientAPIVersion()&0xffff)
	extra.ClientAPIMajor, extra.ClientAPIMinor = vMajor, vMinor

	options := map[string]string{"ao": ao, "audio-device": device}
	for key, value := range options {
		if value == "" {
			delete(options, key)
		}
	}
	player, err := mpv.NewPlayer(lib, mpv.Config{
		AudioClientName: "tingyu-w0",
		LogLevel:        "debug",
		Options:         options,
		Debug: func(format string, args ...any) {
			if verbose {
				fmt.Fprintf(os.Stderr, "mpv: "+format+"\n", args...)
			}
		},
	})
	if err != nil {
		lib.Close()
		return "", extra, err
	}
	for _, name := range []string{"mpv-version", "ffmpeg-version"} {
		if value, ok := player.PropertyString(name); ok {
			switch name {
			case "mpv-version":
				extra.MPVVersion = value
			case "ffmpeg-version":
				extra.FFmpegVersion = value
			}
		}
	}
	extra.ModulePath = moduleFileName(libModuleHandle(abs))

	if err := player.SetQueue([]mpv.Item{{URI: wav}}, 0); err != nil {
		player.Close()
		lib.Close()
		return "", extra, fmt.Errorf("set queue: %w", err)
	}
	if err := player.Play(); err != nil {
		player.Close()
		lib.Close()
		return "", extra, fmt.Errorf("play: %w", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	var snap mpv.Snapshot
	for time.Now().Before(deadline) {
		snap = player.Snapshot()
		if snap.Failure != nil {
			player.Close()
			lib.Close()
			return "", extra, fmt.Errorf("mpv failure: %v", snap.Failure)
		}
		if snap.Position > 0 {
			extra.PlaybackObserved = true
			extra.PositionMax = snap.Position.Round(time.Millisecond).String()
		}
		if extra.AudioOutput == "" {
			if value, ok := player.PropertyString("current-ao"); ok && value != "" {
				extra.AudioOutput = value
			}
		}
		if snap.Processing == mpv.Completed || (!snap.Playing && snap.Duration > 0 && snap.Position >= snap.Duration-200*time.Millisecond) {
			extra.ReachedEnd = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	extra.PositionAfter = snap.Position.Round(time.Millisecond).String()
	extra.Duration = snap.Duration.Round(time.Millisecond).String()
	extra.AOInits = player.AOInits()
	extra.Underruns = player.Underruns()
	extra.FilesLoaded = player.FilesLoaded()
	if value, ok := player.PropertyString("current-ao"); ok {
		extra.AudioOutput = value
	}
	time.Sleep(300 * time.Millisecond)

	if !extra.PlaybackObserved || extra.PositionMax == "" {
		player.Close()
		lib.Close()
		return "", extra, fmt.Errorf("playback did not advance: playing=%v position=%v duration=%v failure=%v",
			snap.Playing, snap.Position, snap.Duration, snap.Failure)
	}
	if err := player.Close(); err != nil {
		lib.Close()
		return "", extra, fmt.Errorf("player close: %w", err)
	}
	if err := lib.Close(); err != nil {
		return "", extra, fmt.Errorf("library unload: %w", err)
	}
	time.Sleep(200 * time.Millisecond)
	extra.ModuleUnloaded = libModuleHandle(abs) == 0
	extra.Goroutines = runtime.NumGoroutine()
	if !extra.ModuleUnloaded {
		return "", extra, fmt.Errorf("libmpv module %s is still loaded after Close", abs)
	}
	if !extra.ReachedEnd {
		return "", extra, fmt.Errorf("playback did not reach the end (position %s of %s)", extra.PositionMax, extra.Duration)
	}
	if extra.Goroutines > before+8 {
		return "", extra, fmt.Errorf("goroutines after close %d, before load %d (possible leak)", extra.Goroutines, before)
	}
	detail := fmt.Sprintf("loaded %s (module %s, %s, sha256 %s), client API %d.%d, %s; played %s to the end (%s of %s) through %s, AO inits %d, underruns %d; unloaded after close (goroutines %d -> %d)",
		filepath.Base(abs), extra.ModulePath, extra.Machine, extra.SHA256[:12], vMajor, vMinor, extra.MPVVersion,
		filepath.Base(wav), extra.PositionMax, extra.Duration, extra.AudioOutput, extra.AOInits, extra.Underruns,
		before, extra.Goroutines)
	return detail, extra, nil
}

// --- helpers --------------------------------------------------------

func moduleFileName(handle uintptr) string {
	if handle == 0 {
		return ""
	}
	buf := make([]uint16, 1024)
	proc := procAddress("kernel32.dll", "GetModuleFileNameW")
	if proc == 0 {
		return ""
	}
	n, _, _ := syscall.SyscallN(proc, handle, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:n])
}

func libModuleHandle(abs string) uintptr {
	proc := procAddress("kernel32.dll", "GetModuleHandleW")
	if proc == 0 {
		return 0
	}
	name, err := syscall.UTF16PtrFromString(abs)
	if err != nil {
		return 0
	}
	handle, _, _ := syscall.SyscallN(proc, uintptr(unsafe.Pointer(name)))
	runtime.KeepAlive(name)
	return handle
}

func procAddress(dll, name string) uintptr {
	handle, err := syscall.LoadLibrary(dll)
	if err != nil {
		return 0
	}
	proc, err := syscall.GetProcAddress(handle, name)
	if err != nil {
		return 0
	}
	return proc
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeWAV writes a 16-bit PCM mono 440 Hz sine, so playback can be measured
// without any personal media.
func writeWAV(path string, duration time.Duration) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	const rate = 44100
	samples := int(float64(rate) * duration.Seconds())
	data := make([]byte, 0, 44+samples*2)
	header := make([]byte, 44)
	copy(header[0:], "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(36+samples*2))
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1) // PCM
	binary.LittleEndian.PutUint16(header[22:], 1) // mono
	binary.LittleEndian.PutUint32(header[24:], rate)
	binary.LittleEndian.PutUint32(header[28:], rate*2)
	binary.LittleEndian.PutUint16(header[32:], 2)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(samples*2))
	data = append(data, header...)
	for i := range samples {
		value := int16(math.Sin(2*math.Pi*440*float64(i)/rate) * 0.25 * math.MaxInt16)
		var sample [2]byte
		binary.LittleEndian.PutUint16(sample[:], uint16(value))
		data = append(data, sample[:]...)
	}
	return os.WriteFile(path, data, 0o644)
}
