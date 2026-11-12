package pages

import (
	"github.com/egoist/mygo/ui"
	"github.com/halunhaku/tingyu/client/internal/ui/components"
	"github.com/halunhaku/tingyu/client/internal/ui/model"
	"github.com/halunhaku/tingyu/client/internal/ui/tokens"
)

func batchAISettings(c *ui.Context, d components.Design, s *model.State) {
	v := s.AIBatch()
	ui.Column(c).FillWidth().Gap(tokens.Space8).Children(func() {
		d.Text(c, tokens.Body, "批量 AI 识别").TextColor(d.P.TextPrimary)
		d.Text(c, tokens.Small, "默认识别缺歌手、专辑、封面或歌词的歌曲。先查看建议，可取消勾选；确认后才搜索并写回。会调用已保存的 AI 服务，可能产生费用；AI 不生成歌词。").TextColor(d.P.TextSecondary).Wrap()
		scope, _ := s.Transient("aiBatchScope").(string)
		if scope == "" {
			scope = "全部信息不完整的歌曲"
		}
		ui.Row(c).Gap(tokens.Space8).Wrap().Children(func() {
			if sel := ui.Select(c, &scope, []string{"全部信息不完整的歌曲", "仅歌手或专辑缺失的歌曲"}).Label("批量识别范围").Disabled(v.Running); sel.Changed() {
				s.SetTransient("aiBatchScope", scope)
			}
			if b := d.Button(c, "开始批量 AI 识别").Disabled(v.Running); b.Clicked() {
				s.BeginAIBatch(scope == "仅歌手或专辑缺失的歌曲")
			}
			if v.Running {
				if b := d.Button(c, "取消批量 AI"); b.Clicked() {
					s.CancelBatchAI()
				}
			}
		})
		if v.Message != "" {
			d.Text(c, tokens.Small, v.Message).TextColor(d.P.TextSecondary).Wrap()
		}
		if v.Total > 0 {
			d.Textf(c, tokens.Small, "进度 %d / %d", v.Done, v.Total).TextColor(d.P.TextTertiary)
		}
		if len(v.Rows) > 0 {
			ui.Scroll(c).Height(280).Children(func() {
				ui.Column(c).FillWidth().Gap(tokens.Space8).Children(func() {
					for i := range v.Rows {
						row := &v.Rows[i]
						ui.Column(c).FillWidth().Gap(3).Children(func() {
							ui.Row(c).Gap(tokens.Space8).Children(func() {
								ui.Checkbox(c, &row.Selected, "采用建议").Disabled(v.Running || row.Suggestion.Title == "" || row.Status != "待确认")
								d.Text(c, tokens.Small, row.Track.Title).TextColor(d.P.TextPrimary).Wrap()
							})
							d.Text(c, tokens.Small, "原信息："+row.Track.Artist+" / "+row.Track.Album).TextColor(d.P.TextTertiary).Wrap()
							if row.Suggestion.Title != "" {
								d.Text(c, tokens.Small, "AI 建议："+row.Suggestion.Title+" / "+row.Suggestion.Artist+" / "+row.Suggestion.Album).TextColor(d.P.TextSecondary).Wrap()
							}
							d.Text(c, tokens.Small, row.Status).TextColor(d.P.TextTertiary).Wrap()
						})
					}
				})
			})
			count := 0
			for _, r := range v.Rows {
				if r.Selected && r.Status == "待确认" {
					count++
				}
			}
			ui.Row(c).Gap(tokens.Space8).Wrap().Children(func() {
				if b := d.Button(c, "全选建议").Disabled(v.Running); b.Clicked() {
					for i := range v.Rows {
						v.Rows[i].Selected = v.Rows[i].Suggestion.Title != "" && v.Rows[i].Status == "待确认"
					}
				}
				if b := d.Button(c, "取消全选").Disabled(v.Running); b.Clicked() {
					for i := range v.Rows {
						v.Rows[i].Selected = false
					}
				}
				if b := d.PrimaryButton(c, "确认所选并搜索补全").Disabled(v.Running || count == 0); b.Clicked() {
					s.ConfirmAIBatch()
				}
			})
		}
	})
}
