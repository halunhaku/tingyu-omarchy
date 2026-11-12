package app

import (
	"context"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *application) lyricBrowser() *metadata.LyricBrowser {
	http := &metadata.HTTP{Client: a.metadataHTTP, UA: metadata.DefaultUserAgent}
	qq, netease := metadata.NewQQMusic(http), metadata.NewNetEase(http)
	return &metadata.LyricBrowser{LRCLIB: metadata.NewLRCLIB(http), Sources: []metadata.LyricSource{{Searcher: netease, Lyrics: netease}, {Searcher: qq, Lyrics: qq}}}
}
func (a *application) SearchLyricCandidates(track library.Track, query metadata.LyricsQuery, id uint64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		hits, failed := a.lyricBrowser().Search(ctx, query)
		message := fmt.Sprintf("找到 %d 条候选；选择一条预览，确认后保存", len(hits))
		if len(hits) == 0 {
			message = "未找到候选，可修改歌名/歌手重搜，或导入、粘贴歌词"
		}
		if len(failed) > 0 {
			labels := []string{}
			for _, source := range failed {
				switch source {
				case "qqmusic":
					labels = append(labels, "QQ 音乐")
				case "netease":
					labels = append(labels, "网易云")
				default:
					labels = append(labels, "LRCLIB")
				}
			}
			message += "；暂时无法查询：" + strings.Join(labels, "、")
		}
		a.updateWindow(func() { a.model.State().SetLyricCandidates(id, hits, message) })
	}()
}
func (a *application) LoadLyricPreview(query metadata.LyricsQuery, hit metadata.LyricCandidate, id uint64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		text, err := a.lyricBrowser().Preview(ctx, query, hit)
		message := "预览已加载；可编辑或直接保存，仅修改歌词"
		if err != nil {
			message = "加载歌词失败：" + metadata.AIErrorMessage(err)
		} else if strings.TrimSpace(text) == "" {
			message = "这个候选没有可获取的歌词，请选择其他候选"
		}
		a.updateWindow(func() { a.model.State().SetLyricText(id, text, message) })
	}()
}
func readLyricFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".lrc" && ext != ".txt" {
		return "", fmt.Errorf("请选择 .lrc 或 .txt 歌词文件")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("无法打开歌词文件，请检查文件位置和读取权限")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if err != nil {
		return "", fmt.Errorf("读取歌词文件失败，请重试")
	}
	return metadata.DecodeLyricsFile(data)
}
func (a *application) ImportLyricFile(id uint64) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: "导入歌词", Filters: []mygo.FileFilter{{Name: "歌词文件", Extensions: []string{"lrc", "txt"}}}})
		text, message := "", "已取消导入；当前编辑内容保留"
		if err != nil {
			message = "无法打开文件选择器，请重试，或直接粘贴歌词"
		} else if len(paths) > 0 {
			var readErr error
			text, readErr = readLyricFile(paths[0])
			if readErr != nil {
				message = readErr.Error()
			} else {
				message = "已导入 " + filepath.Base(paths[0]) + "；请预览后点击保存歌词"
			}
		}
		a.updateWindow(func() { a.model.State().SetLyricText(id, text, message) })
	}()
}
func (a *application) SaveLyricText(track library.Track, text string, id uint64) {
	go func() {
		message, success := "歌词保存失败，请检查曲库后重试", false
		if a.lib == nil {
			message = "曲库未就绪，无法保存歌词"
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			changed, err := a.lib.ReplaceLyrics(ctx, track.ID, text, track.Lyrics)
			if err != nil {
				message = "保存歌词失败：" + err.Error()
			} else {
				success = true
				if changed {
					message = "歌词已保存；歌名、歌手、专辑和封面保持原值"
				} else {
					message = "歌词与已保存内容一致"
				}
			}
		}
		a.updateWindow(func() { a.model.State().LyricsSaved(id, message, success) })
	}()
}
