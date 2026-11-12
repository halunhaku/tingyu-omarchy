package components

import (
	"github.com/egoist/mygo/ui"
	"github.com/halunhaku/tingyu/client/internal/ui/model"
	"github.com/halunhaku/tingyu/client/internal/ui/tokens"
)

func LyricSearchDialog(c *ui.Context, d Design, s *model.State) {
	v := s.LyricSearch()
	if !v.Open {
		return
	}
	d.Overlay(c, &v.Open, func() {
		ui.Column(c).Width(720).MaxHeight(680).Radius(d.Radius(tokens.RadiusXL)).Material(d.RegularGlass()).Border(1, d.P.Border).Padding(tokens.Space16).Gap(tokens.Space8).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tokens.Space8).Children(func() {
				d.Text(c, tokens.H3, "搜索歌词").TextColor(d.P.TextPrimary)
				ui.Spacer(c)
				if b := d.Button(c, "关闭歌词窗口"); b.Clicked() {
					s.CloseLyricSearch()
				}
			})
			d.Text(c, tokens.Small, "目标歌曲："+v.Track.Title+" · "+v.Track.Artist).TextColor(d.P.TextSecondary).Wrap()
			ui.Scroll(c).Height(460).Children(func() {
				ui.Column(c).FillWidth().Gap(tokens.Space8).Children(func() {
					ui.Row(c).Gap(tokens.Space8).Children(func() {
						matchField(c, d, "搜索歌名", "歌名", &v.Title)
						matchField(c, d, "搜索歌手", "歌手", &v.Artist)
						matchField(c, d, "搜索专辑", "可留空", &v.Album)
					})
					ui.Row(c).Gap(tokens.Space8).Wrap().Children(func() {
						if b := d.Button(c, "重新搜索歌词").Disabled(v.Searching || v.Loading || v.Saving); b.Clicked() {
							s.SearchLyrics()
						}
						if b := d.Button(c, "导入 LRC / TXT").Disabled(v.Loading || v.Saving); b.Clicked() {
							s.ImportLyrics()
						}
						if v.Searching || v.Loading {
							ui.Spinner(c)
						}
					})
					if v.Message != "" {
						d.Text(c, tokens.Small, v.Message).TextColor(d.P.TextSecondary).Wrap()
					}
					if len(v.Candidates) > 0 {
						ui.Scroll(c).Height(140).Children(func() {
							ui.Column(c).FillWidth().Gap(3).Children(func() {
								for i, hit := range v.Candidates {
									i, hit := i, hit
									ui.Row(c).FillWidth().Gap(tokens.Space8).Padding(4).Children(func() {
										d.Tag(c, providerLabel(hit.Candidate.Provider))
										ui.Column(c).Grow(1).Gap(2).Children(func() {
											d.Text(c, tokens.Small, hit.Candidate.Title+" · "+hit.Candidate.Artist).TextColor(d.P.TextPrimary).Wrap()
											detail := hit.Candidate.Album
											if hit.Candidate.Duration > 0 {
												detail += " · " + model.FormatDuration(hit.Candidate.Duration)
											}
											d.Text(c, tokens.Small, detail+" · "+hit.Note).TextColor(d.P.TextTertiary).Wrap()
										})
										label := "预览歌词"
										if v.Selected == i {
											label = "已选候选"
										}
										if b := d.Button(c, label).Disabled(v.Loading || v.Saving); b.Clicked() {
											s.PreviewLyricCandidate(i)
										}
									})
								}
							})
						})
					}
					d.Text(c, tokens.Small, "歌词预览 / 粘贴编辑（带时间戳会同步高亮，纯文本可直接显示）").TextColor(d.P.TextTertiary).Wrap()
					ui.TextArea(c, &v.Text).Label("歌词内容").Placeholder("可在这里粘贴 LRC 或纯文本歌词").FillWidth().Height(160).Disabled(v.Loading || v.Saving)
				})
			})
			ui.Row(c).Gap(tokens.Space8).AlignItems(ui.Center).Children(func() {
				d.Text(c, tokens.Small, "仅保存歌词；已有歌词将被替换，音乐文件不变。").TextColor(d.P.TextTertiary).Wrap()
				ui.Spacer(c)
				if b := d.PrimaryButton(c, "保存歌词").Disabled(v.Loading || v.Saving); b.Clicked() {
					s.SaveLyrics()
				}
			})
		})
	})
}
