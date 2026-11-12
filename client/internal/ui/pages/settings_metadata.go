package pages

// The settings page's 元数据 section (plan §6 M5): what the enrichment pipeline
// has done, the cover cache's counters, the orphan sweep, and the optional AI
// fallback's configuration. The four HTTP sources need no credentials; the AI
// key is the only one the user has to provide, and it lives in the Keychain.

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/halunhaku/tingyu/client/internal/metadata"
	"github.com/halunhaku/tingyu/client/internal/ui/components"
	"github.com/halunhaku/tingyu/client/internal/ui/model"
	"github.com/halunhaku/tingyu/client/internal/ui/tokens"
)

// aiForm is the editable copy of the AI settings; it starts from the saved
// values and is committed by the 保存 button, so a Keychain read failure never
// leaves the page empty.
type aiForm struct {
	Loaded  bool
	Enabled bool
	Preset  string
	BaseURL string
	Model   string
	APIKey  string
	SavedAt string
}

func aiFormOf(s *model.State) *aiForm {
	form, _ := s.Transient("aiForm").(*aiForm)
	if form == nil {
		form = &aiForm{}
		s.SetTransient("aiForm", form)
	}
	saved := s.AI()
	signature := fmt.Sprintf("%v|%s|%s|%s|%s", saved.Enabled, saved.Preset, saved.BaseURL, saved.Model, saved.MaskKey())
	if !form.Loaded || form.SavedAt != signature {
		form.Loaded = true
		form.Enabled = saved.Enabled
		form.Preset = saved.Preset
		form.BaseURL = saved.BaseURL
		form.Model = saved.Model
		form.APIKey = saved.APIKey
		form.SavedAt = signature
	}
	return form
}

func metadataSettings(c *ui.Context, d components.Design, s *model.State) {
	view := s.Enrichment()
	components.SectionTitle(c, d, "补全缺的信息",
		"从 QQ 音乐、网易云、iTunes 与 LRCLIB 补全歌曲信息。AI 识别单独操作：先预览建议，确认后再搜索匹配。")

	if !s.MetadataReady() {
		components.Notice(c, d, "本次构建未接入元数据补全：仅可用于查看已入库的元数据。", nil)
	}

	card := ui.Column(c).FillWidth().Gap(tokens.Space10).Padding(tokens.Space16).
		Radius(tokens.RadiusLG).Background(d.P.SurfaceRaised).Border(1, d.P.Border)
	card.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tokens.Space8).Children(func() {
			d.Text(c, tokens.Body, "补全").TextColor(d.P.TextPrimary)
			ui.Spacer(c)
			if view.Running {
				ui.Spinner(c)
				d.Textf(c, tokens.Overline, "%d / %d", view.Done, view.Total).TextColor(d.P.TextSecondary)
			} else {
				d.Textf(c, tokens.Overline, "待补全 %d 首", view.Targets).TextColor(d.P.TextTertiary)
			}
		})
		progress := "尚无补全记录。点「补全全部」会依次处理缺标签、缺封面或缺歌词的曲目。"
		if view.Running {
			progress = "正在处理：" + view.Current
		} else if view.Message != "" {
			progress = view.Message
		} else if view.Done > 0 {
			progress = fmt.Sprintf("上一轮处理 %d 首，变更 %d 首。", view.Done, view.Changed)
		}
		d.Text(c, tokens.Small, progress).TextColor(d.P.TextSecondary).Wrap()
		ui.Row(c).Gap(tokens.Space8).Wrap().Children(func() {
			if view.Running {
				if b := d.Button(c, "取消补全"); b.Clicked() {
					s.CancelEnrichment()
				}
			} else if b := d.PrimaryButton(c, "补全全部"); b.Clicked() {
				s.BeginEnrichment()
			}
			if b := d.Button(c, "清理无引用封面"); b.Clicked() {
				s.PruneCovers()
			}
			if b := d.Button(c, "补全当前播放"); b.Clicked() {
				if track := s.Player().Track; track != nil {
					s.EnrichOne(*track)
				}
			}
		})
		ui.Row(c).Gap(tokens.Space16).Wrap().Children(func() {
			stat := func(label, value string) {
				ui.Column(c).Gap(2).Children(func() {
					d.Text(c, tokens.Overline, label).TextColor(d.P.TextTertiary)
					d.Text(c, tokens.Body, value).TextColor(d.P.TextPrimary)
				})
			}
			stat("变更 / 无变化", fmt.Sprintf("%d / %d", view.Changed, view.Skipped))
			stat("离线跳过", fmt.Sprint(view.Offline))
			stat("失败", fmt.Sprint(view.Failed))
			stat("封面落库", fmt.Sprint(view.Covers))
			stat("歌词落库", fmt.Sprint(view.Lyrics))
			stat("自动补全配额", fmt.Sprintf("%d / %d", view.Used, view.Quota))
		})
		if view.Stopped == "quota" {
			d.Text(c, tokens.Small, "自动补全已达每会话上限；点击「补全全部」可继续，手动操作不受此配额限制。").
				TextColor(d.P.Warning).Wrap()
		}
	})

	cache := ui.Column(c).FillWidth().Gap(tokens.Space8).Padding(tokens.Space16).
		Radius(tokens.RadiusLG).Background(d.P.SurfaceRaised).Border(1, d.P.Border)
	cache.Children(func() {
		d.Text(c, tokens.Body, "封面缓存").TextColor(d.P.TextPrimary)
		d.Text(c, tokens.Small,
			"封面按内容命名落盘，二进制不入库；删除来源或曲目后可以清理没有引用的封面，共享的封面会保留。").
			TextColor(d.P.TextSecondary).Wrap()
		ui.Row(c).Gap(tokens.Space16).Wrap().Children(func() {
			stat := func(label, value string) {
				ui.Column(c).Gap(2).Children(func() {
					d.Text(c, tokens.Overline, label).TextColor(d.P.TextTertiary)
					d.Text(c, tokens.Body, value).TextColor(d.P.TextPrimary)
				})
			}
			stat("命中率", fmt.Sprintf("%.0f%%", view.HitRate*100))
			stat("写入", fmt.Sprint(view.CoverSaved))
			stat("命中（已在盘上）", fmt.Sprint(view.CoverReused))
			stat("已清理孤儿", fmt.Sprint(view.CoverPruned))
			stat("抓取失败", fmt.Sprint(view.FetcherFails))
		})
	})

	form := aiFormOf(s)
	ai := ui.Column(c).FillWidth().Gap(tokens.Space10).Padding(tokens.Space16).
		Radius(tokens.RadiusLG).Background(d.P.SurfaceRaised).Border(1, d.P.Border)
	ai.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tokens.Space8).Children(func() {
			d.Text(c, tokens.Body, "AI 识别（可选）").TextColor(d.P.TextPrimary)
			ui.Spacer(c)
			enabled := form.Enabled
			if sw := ui.Switch(c, &enabled); sw.Changed() {
				form.Enabled = enabled
			}
		})
		d.Text(c, tokens.Small,
			"在歌曲的匹配窗口点击「AI 识别」，或使用下方批量识别。先查看建议，再确认搜索和写回。不会随启动或「补全全部」自动调用 AI。连接测试会向所填模型发送一次简短请求，可能产生少量费用。").
			TextColor(d.P.TextSecondary).Wrap()
		if s.AIError() != "" {
			components.Notice(c, d, s.AIError(), nil)
		}
		providerRow(c, d, s, form)
		ui.Row(c).Gap(tokens.Space8).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				d.Text(c, tokens.Overline, "接口地址（OpenAI 兼容）").TextColor(d.P.TextTertiary)
				ui.TextInput(c, &form.BaseURL).Placeholder("https://api.deepseek.com/v1").
					Label("接口地址").FillWidth()
			})
			ui.Column(c).Width(220).Gap(4).Children(func() {
				d.Text(c, tokens.Overline, "模型").TextColor(d.P.TextTertiary)
				ui.TextInput(c, &form.Model).Placeholder("deepseek-chat").Label("模型").FillWidth()
			})
		})
		ui.Column(c).Gap(4).Children(func() {
			d.Text(c, tokens.Overline, "API 密钥（安全保存在系统钥匙串）").
				TextColor(d.P.TextTertiary)
			ui.TextInput(c, &form.APIKey).Password().Placeholder("sk-…").Label("API 密钥").FillWidth()
		})
		ui.Row(c).AlignItems(ui.Center).Gap(tokens.Space8).Children(func() {
			d.Text(c, tokens.Overline, "需要用户提供的凭据：AI 服务的 API Key；本地 Ollama 可不填。其他元数据源无需账号。").
				TextColor(d.P.TextTertiary).Wrap()
			ui.Spacer(c)
			if b := d.Button(c, "测试连接").Disabled(s.AITest().Running); b.Clicked() {
				s.TestAI(metadata.AISettings{Preset: form.Preset, BaseURL: form.BaseURL, Model: form.Model, APIKey: form.APIKey})
			}
			if b := d.Button(c, "保存 AI 配置"); b.Clicked() {
				next := metadata.AISettings{
					Enabled: form.Enabled,
					Preset:  form.Preset,
					BaseURL: form.BaseURL,
					Model:   form.Model,
					APIKey:  form.APIKey,
				}
				s.SaveAISettings(next)
			}
		})
		if test := s.AITest(); test.Message != "" {
			d.Text(c, tokens.Small, test.Message).TextColor(d.P.TextSecondary).Wrap()
		}
		batchAISettings(c, d, s)
	})
}

// providerRow cycles the provider preset and fills the two defaults.
func providerRow(c *ui.Context, d components.Design, s *model.State, form *aiForm) {
	preset := model.AIPreset(form.Preset)
	ui.Row(c).AlignItems(ui.Center).Gap(tokens.Space8).Children(func() {
		d.Text(c, tokens.Overline, "服务商").TextColor(d.P.TextTertiary)
		names := make([]string, len(metadata.AIPresets))
		for i, p := range metadata.AIPresets {
			names[i] = p.DisplayName
		}
		selected := preset.DisplayName
		if selectBox := ui.Select(c, &selected, names).Label("服务商"); selectBox.Changed() {
			for _, p := range metadata.AIPresets {
				if p.DisplayName == selected {
					form.Preset, form.BaseURL, form.Model = p.Name, p.BaseURL, p.Model
					break
				}
			}
		}
		d.Textf(c, tokens.Overline, "%d 个预设", len(metadata.AIPresets)).TextColor(d.P.TextTertiary)
	})
}
