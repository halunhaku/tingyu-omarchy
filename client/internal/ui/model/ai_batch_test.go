package model

import (
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"testing"
)

type batchActions struct {
	fakeMetadata
	starts   int
	applied  []AIBatchRow
	cancels  int
	tagsOnly bool
}

func (a *batchActions) StartAIBatch(id uint64, tagsOnly bool)     { a.starts++; a.tagsOnly = tagsOnly }
func (a *batchActions) ApplyAIBatch(id uint64, rows []AIBatchRow) { a.applied = rows }
func (a *batchActions) CancelAIBatch()                            { a.cancels++ }
func TestAIBatchPreviewConfirmAndCancel(t *testing.T) {
	s := New(nil, nil, "", nil)
	a := &batchActions{}
	s.SetMetadataActions(a)
	s.BeginAIBatch(false)
	if a.starts != 0 {
		t.Fatal("started without config")
	}
	config := metadata.DefaultAISettings()
	config.Enabled = true
	s.SetAISettings(config, "")
	s.BeginAIBatch(true)
	s.BeginAIBatch(false)
	if a.starts != 1 || !a.tagsOnly {
		t.Fatal("scope or duplicate start")
	}
	s.CancelBatchAI()
	if a.cancels != 1 {
		t.Fatal("cancel not forwarded")
	}
	id := s.AIBatch().RequestID
	rows := []AIBatchRow{
		{Track: library.Track{ID: "1"}, Suggestion: metadata.AIResult{Title: "one"}, Selected: true, Status: "待确认"},
		{Track: library.Track{ID: "2"}, Suggestion: metadata.AIResult{Title: "two"}, Selected: false, Status: "待确认"},
		{Track: library.Track{ID: "3"}, Selected: true, Status: "未识别到歌曲"},
	}
	s.SetAIBatch(id, AIBatchState{Rows: rows})
	if len(a.manuals) != 0 || len(a.applied) != 0 {
		t.Fatal("preview wrote data")
	}
	s.ConfirmAIBatch()
	if len(a.applied) != 1 || a.applied[0].Track.ID != "1" || !s.AIBatch().Applying {
		t.Fatal("selection not respected")
	}
	s.SetAIBatch(id, AIBatchState{Message: "stale"})
	if !s.AIBatch().Running {
		t.Fatal("stale result accepted")
	}
}
