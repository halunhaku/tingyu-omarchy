package metadata

import (
	"bytes"
	"errors"
	"golang.org/x/text/encoding/simplifiedchinese"
	textunicode "golang.org/x/text/encoding/unicode"
	"strings"
	"unicode/utf8"
)

func DecodeLyricsFile(data []byte) (string, error) {
	if len(data) > 2<<20 {
		return "", errors.New("歌词文件超过 2 MB")
	}
	var text string
	var err error
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		decoded, e := textunicode.UTF16(textunicode.LittleEndian, textunicode.ExpectBOM).NewDecoder().Bytes(data)
		text, err = string(decoded), e
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		decoded, e := textunicode.UTF16(textunicode.BigEndian, textunicode.ExpectBOM).NewDecoder().Bytes(data)
		text, err = string(decoded), e
	default:
		data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
		if utf8.Valid(data) {
			text = string(data)
		} else {
			decoded, e := simplifiedchinese.GB18030.NewDecoder().Bytes(data)
			text, err = string(decoded), e
		}
	}
	if err != nil || strings.ContainsRune(text, 0) || strings.ContainsRune(text, utf8.RuneError) {
		return "", errors.New("无法读取歌词编码，请使用 UTF-8、UTF-16 或 GB18030 文本")
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(text) == "" {
		return "", errors.New("歌词文件为空")
	}
	return text, nil
}
