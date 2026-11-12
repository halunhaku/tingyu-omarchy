package metadata

import (
	"golang.org/x/text/encoding/simplifiedchinese"
	textunicode "golang.org/x/text/encoding/unicode"
	"testing"
)

func TestLyricsFileEncodingsAndInvalidInput(t *testing.T) {
	text := "[00:01.00]中文歌词\n"
	gb, _ := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(text))
	utf16, _ := textunicode.UTF16(textunicode.LittleEndian, textunicode.UseBOM).NewEncoder().Bytes([]byte(text))
	for _, data := range [][]byte{[]byte(text), append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...), gb, utf16} {
		decoded, err := DecodeLyricsFile(data)
		if err != nil || decoded != text {
			t.Fatalf("decode=%q err=%v", decoded, err)
		}
	}
	for _, data := range [][]byte{nil, []byte{0, 1, 0, 3}, make([]byte, (2<<20)+1)} {
		if _, err := DecodeLyricsFile(data); err == nil {
			t.Fatal("invalid file accepted")
		}
	}
}
