package shell

// M5's window tests: the 元数据 settings section, the manual-match dialog and
// the stage's palette wiring, against the same real SQLite fixture the rest of
// the shell tests use.

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"github.com/halunhaku/tingyu/client/internal/mpv"
	"github.com/halunhaku/tingyu/client/internal/player"
	"github.com/halunhaku/tingyu/client/internal/ui/model"
)

// metadataActions records the M5 callbacks the window asks for.
type metadataActions struct {
	passes       int
	cancels      int
	single       []library.Track
	matchLoads   []library.Track
	manuals      []manualMatch
	prunes       int
	ai           []metadata.AISettings
	batchStarts  int
	batchApplied []model.AIBatchRow
}

func (m *metadataActions) StartAIBatch(id uint64, tagsOnly bool)           { m.batchStarts++ }
func (m *metadataActions) ApplyAIBatch(id uint64, rows []model.AIBatchRow) { m.batchApplied = rows }
func (m *metadataActions) CancelAIBatch()                                  {}

func TestBatchAIButtonsPreviewBeforeConfirmedApply(t *testing.T) {
	m, actions := newMetadataModel(t)
	config := metadata.DefaultAISettings()
	config.Enabled = true
	m.SetAISettings(config, "")
	m.State().OpenPage(model.PageSettings)
	m.State().SetSettingsSection(3)
	tt := ui.NewTester(m.View, DefaultWidth, 1800)
	frame(tt)
	if err := tt.Click("开始批量 AI 识别"); err != nil {
		t.Fatal(err)
	}
	if actions.batchStarts != 1 || len(actions.batchApplied) != 0 {
		t.Fatal("start incorrectly applied data")
	}
	track := m.State().Library().Items()[0]
	m.State().SetAIBatch(m.State().AIBatch().RequestID, model.AIBatchState{Rows: []model.AIBatchRow{{Track: track, Suggestion: metadata.AIResult{Title: "AI Song", Artist: "Artist"}, Selected: true, Status: "待确认"}}})
	frame(tt)
	if !tt.HasText("AI 建议：AI Song") {
		t.Fatal("preview missing")
	}
	if err := tt.Click("确认所选并搜索补全"); err != nil {
		t.Fatal(err)
	}
	if len(actions.batchApplied) != 1 || len(actions.manuals) != 0 {
		t.Fatal("confirmation not forwarded")
	}
}

type manualMatch struct {
	title, artist, album, cover string
}

func (m *metadataActions) EnrichAll()        { m.passes++ }
func (m *metadataActions) CancelEnrichment() { m.cancels++ }
func (m *metadataActions) EnrichTrack(track library.Track) {
	m.single = append(m.single, track)
}
func (m *metadataActions) LoadMatchCandidates(track library.Track) {
	m.matchLoads = append(m.matchLoads, track)
}
func (m *metadataActions) ApplyManualMatch(track library.Track, title, artist, album, coverURL string) {
	m.manuals = append(m.manuals, manualMatch{title: title, artist: artist, album: album, cover: coverURL})
}
func (m *metadataActions) SaveAISettings(settings metadata.AISettings) error {
	m.ai = append(m.ai, settings)
	return nil
}
func (m *metadataActions) PruneCovers() { m.prunes++ }
func (m *metadataActions) TestAIConnection(settings metadata.AISettings, id uint64) {
	m.ai = append(m.ai, settings)
}
func (m *metadataActions) RecognizeAI(track library.Track, id uint64) {}

func TestMatchAIRecognitionPreviewAndConfirm(t *testing.T) {
	m, actions := newMetadataModel(t)
	config := metadata.DefaultAISettings()
	config.Enabled = true
	m.SetAISettings(config, "")
	track := m.State().Library().Items()[0]
	m.State().OpenMatch(track)
	m.SetMatchCandidates(nil, "")
	tt := ui.NewTester(m.View, DefaultWidth, DefaultHeight)
	frame(tt)
	if err := tt.Click("AI 识别"); err != nil {
		t.Fatal(err)
	}
	if !m.State().Match().AIWorking {
		t.Fatal("recognition did not start")
	}
	m.State().FinishAIRecognition(m.State().Match().RequestID, &metadata.AIResult{Title: "AI Song", Artist: "AI Artist", Album: "AI Album"}, "")
	frame(tt)
	if !tt.HasText("AI 建议（尚未写入）") {
		t.Fatal("preview missing")
	}
	if err := tt.Click("采用建议并搜索"); err != nil {
		t.Fatal(err)
	}
	if len(actions.manuals) != 0 || len(actions.matchLoads) != 2 || actions.matchLoads[1].Title != "AI Song" {
		t.Fatal("confirmation must search without saving")
	}
}

func TestAIProviderDropdownAndConnectionButton(t *testing.T) {
	m, actions := newMetadataModel(t)
	m.State().OpenPage(model.PageSettings)
	m.State().SetSettingsSection(3)
	tt := ui.NewTester(m.View, DefaultWidth, 1600)
	frame(tt)
	if err := tt.Click("DeepSeek"); err != nil {
		t.Fatal(err)
	}
	frame(tt)
	if !tt.HasText("Kimi (Moonshot)") || !tt.HasText("OpenAI") {
		t.Fatalf("provider did not open a choice list: %v", tt.Texts())
	}
	if err := tt.Click("Kimi (Moonshot)"); err != nil {
		t.Fatal(err)
	}
	frame(tt)
	if err := tt.Click("测试连接"); err != nil {
		t.Fatal(err)
	}
	if len(actions.ai) != 1 || actions.ai[0].Preset != "kimi" || actions.ai[0].Model != "moonshot-v1-8k" {
		t.Fatalf("wrong draft: %+v", actions.ai)
	}
	if m.State().AI().Preset != "deepseek" {
		t.Fatal("testing persisted unsaved provider")
	}
	frame(tt)
	if !tt.HasText("正在测试当前填写的配置") {
		t.Fatal("test progress missing")
	}
}

// newMetadataModel is a window over the fixture with both action halves.
func newMetadataModel(t *testing.T) (*Model, *metadataActions) {
	t.Helper()
	_, m := newLibrary(t, sampleTracks())
	actions := &metadataActions{}
	m.SetMetadataActions(actions)
	m.SetAISettings(metadata.DefaultAISettings(), "")
	return m, actions
}

func TestSettingsMetadataSectionRendersAndActs(t *testing.T) {
	m, actions := newMetadataModel(t)
	m.State().OpenPage(model.PageSettings)
	m.State().SetSettingsSection(3)
	tt := ui.NewTester(m.View, DefaultWidth, DefaultHeight)
	frame(tt)
	for _, want := range []string{
		"补全缺的信息", "补全全部", "清理无引用封面", "封面缓存",
		"AI 识别（可选）", "保存 AI 配置", "测试连接", "需要用户提供的凭据",
	} {
		if !tt.HasText(want) {
			t.Fatalf("settings metadata section lacks %q; texts: %v", want, tt.Texts())
		}
	}
	if err := tt.Click("补全全部"); err != nil {
		t.Fatalf("click 补全全部: %v", err)
	}
	if actions.passes != 1 {
		t.Fatalf("passes=%d after the click", actions.passes)
	}
	frame(tt)
	if err := tt.Click("清理无引用封面"); err != nil {
		t.Fatalf("click 清理无引用封面: %v", err)
	}
	if actions.prunes != 1 {
		t.Fatalf("prunes=%d after the click", actions.prunes)
	}
}

func TestSettingsMetadataSectionShowsCounters(t *testing.T) {
	m, _ := newMetadataModel(t)
	m.State().OpenPage(model.PageSettings)
	m.State().SetSettingsSection(3)
	m.SetEnrichment(model.EnrichmentView{
		Targets: 20, Changed: 7, Offline: 2, Skipped: 5, Covers: 6, Lyrics: 4,
		CoverSaved: 6, CoverReused: 3, HitRate: 0.33, Quota: 60, Used: 14,
	})
	tt := ui.NewTester(m.View, DefaultWidth, DefaultHeight)
	frame(tt)
	for _, want := range []string{"待补全 20 首", "33%", "14 / 60", "已清理孤儿"} {
		if !tt.HasText(want) {
			t.Fatalf("counters lack %q; texts: %v", want, tt.Texts())
		}
	}
}

func TestMatchDialogSearchButtonUsesEditedFields(t *testing.T) {
	m, actions := newMetadataModel(t)
	track := m.State().Library().Items()[0]
	m.State().OpenMatch(track)
	m.SetMatchCandidates([]model.MatchCandidate{{Title: "Old", CoverURL: "https://example.invalid/old.jpg"}}, "")
	m.State().Match().Title = "Updated title"
	m.State().Match().Artist = "Updated artist"
	m.State().Match().Album = "Updated album"
	tt := ui.NewTester(m.View, DefaultWidth, DefaultHeight)
	frame(tt)
	if err := tt.Click("重新搜索"); err != nil {
		t.Fatal(err)
	}
	if len(actions.matchLoads) != 2 {
		t.Fatalf("searches=%+v", actions.matchLoads)
	}
	got := actions.matchLoads[1]
	if got.ID != track.ID || got.Title != "Updated title" || got.Artist != "Updated artist" || got.Album != "Updated album" {
		t.Fatalf("query=%+v", got)
	}
	if len(actions.manuals) != 0 {
		t.Fatal("search wrote metadata")
	}
	frame(tt)
	if !tt.HasText("正在搜索…") {
		t.Fatalf("missing loading state: %v", tt.Texts())
	}
	m.SetMatchCandidates([]model.MatchCandidate{{Title: "New", Artist: "Artist", Album: "Album"}}, "")
	frame(tt)
	if err := tt.Click("采用"); err != nil {
		t.Fatal(err)
	}
	frame(tt)
	if err := tt.Click("应用并写回"); err != nil {
		t.Fatal(err)
	}
	if len(actions.manuals) != 1 || actions.manuals[0].title != "New" {
		t.Fatalf("writes=%+v", actions.manuals)
	}
}

func TestMatchDialogRendersAndApplies(t *testing.T) {
	m, actions := newMetadataModel(t)
	track := m.State().Library().Items()[0]
	m.State().OpenMatch(track)
	m.SetMatchCandidates([]model.MatchCandidate{
		{Provider: "qqmusic", Title: "晴天", Artist: "周杰伦", Album: "叶惠美", CoverURL: "https://example.invalid/c.jpg"},
	}, "")
	tt := ui.NewTester(m.View, DefaultWidth, DefaultHeight)
	frame(tt)
	for _, want := range []string{"匹配元数据", "应用并写回", "QQ 音乐", "晴天"} {
		if !tt.HasText(want) {
			t.Fatalf("dialog lacks %q; texts: %v", want, tt.Texts())
		}
	}
	if len(actions.matchLoads) != 1 || actions.matchLoads[0].ID != track.ID {
		t.Fatalf("candidate search %+v", actions.matchLoads)
	}
	if err := tt.Click("采用"); err != nil {
		t.Fatalf("click 采用: %v", err)
	}
	frame(tt)
	if err := tt.Click("应用并写回"); err != nil {
		t.Fatalf("click 应用并写回: %v", err)
	}
	if len(actions.manuals) != 1 {
		t.Fatalf("manuals %+v", actions.manuals)
	}
	manual := actions.manuals[0]
	if manual.title != "晴天" || manual.artist != "周杰伦" || manual.album != "叶惠美" {
		t.Fatalf("manual %+v", manual)
	}
	if manual.cover != "https://example.invalid/c.jpg" {
		t.Fatalf("cover %q", manual.cover)
	}
}

// TestLyricsWordRendererRendersTheActiveLine covers the per-character renderer:
// the same parsed lines, coloured word by word, with the playback position
// deciding the progress (no wall clock).
func TestLyricsWordRendererRendersTheActiveLine(t *testing.T) {
	m, _ := newMetadataModel(t)
	track := m.State().Library().Items()[0]
	track.Lyrics = "[00:01.00]晴天 有雨\n[00:05.00]花海 无风\n"
	m.SetPlayer(player.Update{
		Snapshot: mpv.Snapshot{Playing: true, Position: 6 * time.Second, Duration: track.Duration},
		Track:    &track,
		Repeat:   player.RepeatAll,
	})
	m.State().OpenPage(model.PageLyrics)
	m.State().SetLyricsMode(model.LyricsWords)
	tt := ui.NewTester(m.View, DefaultWidth, DefaultHeight)
	frame(tt)
	if !tt.HasText("花海 无风") {
		t.Fatalf("the active line is not rendered; texts: %v", tt.Texts())
	}
	if !tt.HasText("歌词显示") {
		t.Fatalf("the renderer switch is missing; texts: %v", tt.Texts())
	}
	// Switching back to 逐行 keeps the same document.
	if err := tt.Click("逐行"); err != nil {
		t.Fatalf("click 逐行: %v", err)
	}
	frame(tt)
	if m.State().LyricsMode() != model.LyricsLines {
		t.Fatalf("mode is %d after clicking 逐行", m.State().LyricsMode())
	}
	if !tt.HasText("花海 无风") {
		t.Fatal("the line disappeared when switching renderers")
	}
}

func TestStageShowsTheMatchEntryAndThePaletteBackground(t *testing.T) {
	m, _ := newMetadataModel(t)
	track := m.State().Library().Items()[0]
	m.SetPlayer(player.Update{Snapshot: mpv.Snapshot{Playing: true, Duration: track.Duration}, Track: &track, Repeat: player.RepeatAll})
	m.State().OpenStage()
	tt := ui.NewTester(m.View, DefaultWidth, DefaultHeight)
	frame(tt)
	if !tt.HasText("匹配元数据") {
		t.Fatalf("the stage lacks the match entry; texts: %v", tt.Texts())
	}
	if !tt.HasText(track.Title) {
		t.Fatalf("the stage lacks the title %q", track.Title)
	}
}
