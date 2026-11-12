package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"github.com/halunhaku/tingyu/client/internal/ui/model"
)

type filenameRecognizer interface {
	CleanFilenames(context.Context, []metadata.AIInput) (map[string]metadata.AIResult, error)
}

// runAIRecognition is read-only. Stable per-track ids map replies even if the
// model reorders them; unknown ids and missing rows are never guessed.
func runAIRecognition(ctx context.Context, tracks []library.Track, client filenameRecognizer, publish func(model.AIBatchState)) {
	view := model.AIBatchState{Running: true, Total: len(tracks)}
	for _, track := range tracks {
		view.Rows = append(view.Rows, model.AIBatchRow{Track: track, Status: "等待识别"})
	}
	report := func() {
		snapshot := view
		snapshot.Rows = append([]model.AIBatchRow(nil), view.Rows...)
		publish(snapshot)
	}
	report()
	for start := 0; start < len(tracks); start += 8 {
		if ctx.Err() != nil {
			break
		}
		end := min(start+8, len(tracks))
		inputs := []metadata.AIInput{}
		for i := start; i < end; i++ {
			track := tracks[i]
			inputs = append(inputs, metadata.AIInput{ID: track.ID, Filename: strings.Join([]string{track.Title, track.Artist, track.Album}, " | ")})
			view.Rows[i].Status = "正在识别"
		}
		view.Message = fmt.Sprintf("正在识别第 %d–%d 首；尚未写入曲库", start+1, end)
		report()
		requestCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		hits, err := client.CleanFilenames(requestCtx, inputs)
		cancel()
		if ctx.Err() != nil {
			break
		}
		for i := start; i < end; i++ {
			row := &view.Rows[i]
			view.Done++
			if err != nil {
				row.Status = "识别失败：" + metadata.AIErrorMessage(err)
				continue
			}
			if hit, ok := hits[row.Track.ID]; ok && strings.TrimSpace(hit.Title) != "" {
				row.Suggestion = hit
				row.Selected = true
				row.Status = "待确认"
			} else {
				row.Status = "未识别到歌曲，请单独检查"
			}
		}
		if err != nil {
			view.Message = "批量识别已暂停：" + metadata.AIErrorMessage(err)
			break
		}
		report()
	}
	view.Running = false
	if ctx.Err() != nil {
		view.Message = "批量识别已停止；已得到的建议仍可查看和确认"
	} else if view.Done == view.Total {
		view.Message = "识别完成；请核对建议并勾选，再确认搜索补全。未修改曲库"
	}
	for i := range view.Rows {
		if view.Rows[i].Status == "等待识别" || view.Rows[i].Status == "正在识别" {
			view.Rows[i].Status = "未处理，可重新识别"
		}
	}
	report()
}

// share the enrichment gate so automatic/bulk passes cannot overlap this job.
func (a *application) beginAIBatch(id uint64, applying bool) (context.Context, func(), bool) {
	fail := func(message string) {
		v := *a.model.State().AIBatch()
		v.Running, v.Applying, v.Message = false, false, message
		a.model.State().SetAIBatch(id, v)
	}
	if a.lib == nil || a.enrich == nil {
		fail("本次构建未接入元数据补全")
		return nil, nil, false
	}
	if applying && !a.lib.Writable() {
		fail("只读模式：不能写回曲库")
		return nil, nil, false
	}
	if !a.enrichActive.CompareAndSwap(false, true) {
		v := *a.model.State().AIBatch()
		v.Running = false
		v.Applying = false
		v.Message = "已有补全任务正在运行，请待其结束或取消后重试"
		a.model.State().SetAIBatch(id, v)
		return nil, nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	a.enrichMu.Lock()
	a.enrichCancel = cancel
	a.enrichMu.Unlock()
	finish := func() {
		cancel()
		a.enrichMu.Lock()
		a.enrichCancel = nil
		a.enrichMu.Unlock()
		a.enrichActive.Store(false)
	}
	return ctx, finish, true
}

func (a *application) StartAIBatch(id uint64, tagsOnly bool) {
	settings := a.model.State().AI()
	if err := metadata.ValidateAISettings(settings); err != nil {
		a.model.State().SetAIBatch(id, model.AIBatchState{Message: err.Error()})
		return
	}
	if !settings.Ready() {
		a.model.State().SetAIBatch(id, model.AIBatchState{Message: "请先启用并保存 AI 配置"})
		return
	}
	ctx, finish, ok := a.beginAIBatch(id, false)
	if !ok {
		return
	}
	go func() {
		defer finish()
		publish := func(view model.AIBatchState) { a.updateWindow(func() { a.model.State().SetAIBatch(id, view) }) }
		targets, err := a.lib.EnrichmentTargets(ctx, 0)
		if err != nil {
			publish(model.AIBatchState{Message: "读取待识别歌曲失败，请检查曲库后重试"})
			return
		}
		tracks := []library.Track{}
		for _, target := range targets {
			if tagsOnly && !target.NeedsTags {
				continue
			}
			if !target.NeedsTags && !target.NeedsCover && !target.NeedsLyrics {
				continue
			}
			tracks = append(tracks, target.Track)
		}
		if len(tracks) == 0 {
			publish(model.AIBatchState{Message: "当前范围没有需要识别的歌曲"})
			return
		}
		runAIRecognition(ctx, tracks, metadata.NewAIClient(settings, a.metadataHTTP), publish)
	}()
}

func (a *application) CancelAIBatch() { a.CancelEnrichment() }

func (a *application) ApplyAIBatch(id uint64, rows []model.AIBatchRow) {
	original := *a.model.State().AIBatch()
	ctx, finish, ok := a.beginAIBatch(id, true)
	if !ok {
		return
	}
	go func() {
		defer finish()
		view := original
		view.Running = true
		view.Applying = true
		view.Done = 0
		view.Total = len(rows)
		view.Rows = append([]model.AIBatchRow(nil), original.Rows...)
		publish := func() {
			snapshot := view
			snapshot.Rows = append([]model.AIBatchRow(nil), view.Rows...)
			a.updateWindow(func() { a.model.State().SetAIBatch(id, snapshot); a.model.Invalidate() })
		}
		changed, failed, lyrics := 0, 0, 0
		for _, row := range rows {
			if ctx.Err() != nil {
				break
			}
			view.Message = "正在搜索补全：" + row.Suggestion.Title
			publish()
			current, found, err := a.lib.Track(ctx, row.Track.ID)
			status := ""
			if err != nil || !found {
				status = "已跳过：歌曲不存在或读取失败"
			} else if current.Title != row.Track.Title || current.Artist != row.Track.Artist || current.Album != row.Track.Album {
				status = "已跳过：识别后信息已修改，请重新识别"
			} else {
				result := a.enrich.EnrichRecognized(ctx, current, row.Suggestion)
				if result.Err != nil {
					if errors.Is(result.Err, library.ErrEnrichmentChanged) {
						status = "已跳过：搜索期间信息已修改，请重新识别"
					} else {
						status = "写入失败，请重试"
						failed++
					}
				} else {
					if result.Changed {
						changed++
					}
					if result.Fields.Lyrics != "" {
						lyrics++
					}
					status = "已保存并搜索补全"
					if strings.TrimSpace(current.Lyrics) == "" && result.Fields.Lyrics == "" {
						status = "已保存歌曲信息；仍未取得歌词，可稍后重试"
					}
				}
			}
			if ctx.Err() != nil {
				break
			}
			view.Done++
			for i := range view.Rows {
				if view.Rows[i].Track.ID == row.Track.ID {
					view.Rows[i].Status = status
					view.Rows[i].Selected = false
					break
				}
			}
			publish()
		}
		view.Running = false
		view.Applying = false
		view.Message = fmt.Sprintf("搜索补全完成：处理 %d / %d 首，变更 %d 首，新增歌词 %d 首，写入失败 %d 首", view.Done, view.Total, changed, lyrics, failed)
		if ctx.Err() != nil {
			view.Message = fmt.Sprintf("已停止：处理 %d / %d 首；已写入的结果保留，其余建议仍可确认", view.Done, view.Total)
		}
		publish()
	}()
}
