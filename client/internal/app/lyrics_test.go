package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLyricsFileWithChinesePathAndSizeLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "中文 歌词.LRC")
	if err := os.WriteFile(path, []byte("[00:01]歌词"), 0600); err != nil {
		t.Fatal(err)
	}
	text, err := readLyricFile(path)
	if err != nil || text != "[00:01]歌词" {
		t.Fatalf("read=%q err=%v", text, err)
	}
	invalid := filepath.Join(t.TempDir(), "file.exe")
	_ = os.WriteFile(invalid, []byte("lyrics"), 0600)
	if _, err := readLyricFile(invalid); err == nil {
		t.Fatal("invalid extension accepted")
	}
	_ = os.WriteFile(path, make([]byte, (2<<20)+100), 0600)
	if _, err := readLyricFile(path); err == nil {
		t.Fatal("oversize accepted")
	}
}
