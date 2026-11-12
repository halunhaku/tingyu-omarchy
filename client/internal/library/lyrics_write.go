package library

import (
	"context"
	"errors"
	"strings"
)

// ReplaceLyrics updates only lyrics, and refuses to overwrite a newer lyric
// written since the editor opened. Metadata edits do not affect this operation.
func (l *Library) ReplaceLyrics(ctx context.Context, id, text, previous string) (bool, error) {
	if !l.writes {
		return false, errors.New("只读模式不能保存歌词")
	}
	if strings.TrimSpace(text) == "" || len(text) > 2<<20 || strings.ContainsRune(text, 0) {
		return false, errors.New("歌词为空或格式无效")
	}
	if text == previous {
		return false, nil
	}
	result, err := l.db.Execute(ctx, `UPDATE tracks SET lyrics = ? WHERE id = ? AND COALESCE(lyrics,'') = ?`, text, id, previous)
	if err != nil {
		return false, err
	}
	if result.Changes == 0 {
		return false, errors.New("歌曲已删除或歌词已更新，请重新打开歌词窗口后重试")
	}
	l.bump(id)
	return true, nil
}
