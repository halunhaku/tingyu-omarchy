package model

import (
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
)

type AIBatchRow struct {
	Track      library.Track
	Suggestion metadata.AIResult
	Selected   bool
	Status     string
}
type AIBatchState struct {
	RequestID   uint64
	Running     bool
	Applying    bool
	Done, Total int
	Message     string
	Rows        []AIBatchRow
}
type AIBatchActions interface {
	StartAIBatch(uint64, bool)
	ApplyAIBatch(uint64, []AIBatchRow)
	CancelAIBatch()
}

func (s *State) AIBatch() *AIBatchState { return &s.aiBatch }
func (s *State) BeginAIBatch(tagsOnly bool) {
	if s.aiBatch.Running {
		return
	}
	if !s.ai.Ready() {
		s.aiBatch.Message = "请先启用并保存 AI 配置，再开始批量识别"
		return
	}
	a, ok := s.metadata().(AIBatchActions)
	if !ok {
		s.aiBatch.Message = "本次构建未接入批量 AI 识别"
		return
	}
	s.aiBatch = AIBatchState{RequestID: s.aiBatch.RequestID + 1, Running: true, Message: "正在读取待识别歌曲；识别阶段不会写入曲库…"}
	a.StartAIBatch(s.aiBatch.RequestID, tagsOnly)
}
func (s *State) SetAIBatch(id uint64, view AIBatchState) {
	if id != s.aiBatch.RequestID {
		return
	}
	view.RequestID = id
	view.Rows = append([]AIBatchRow(nil), view.Rows...)
	s.aiBatch = view
}
func (s *State) ConfirmAIBatch() {
	if s.aiBatch.Running {
		return
	}
	a, ok := s.metadata().(AIBatchActions)
	if !ok {
		return
	}
	rows := []AIBatchRow{}
	for _, row := range s.aiBatch.Rows {
		if row.Selected && row.Suggestion.Title != "" && row.Status == "待确认" {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		s.aiBatch.Message = "请先勾选需要采用的识别建议"
		return
	}
	s.aiBatch.RequestID++
	s.aiBatch.Running, s.aiBatch.Applying = true, true
	s.aiBatch.Done, s.aiBatch.Total = 0, len(rows)
	s.aiBatch.Message = "正在使用已确认的建议搜索并补全；已完成的写入会保留"
	a.ApplyAIBatch(s.aiBatch.RequestID, rows)
}
func (s *State) CancelBatchAI() {
	if a, ok := s.metadata().(AIBatchActions); ok && s.aiBatch.Running {
		s.aiBatch.Message = "正在取消；已完成的结果会保留…"
		a.CancelAIBatch()
	}
}
