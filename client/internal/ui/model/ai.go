package model

import (
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
)

// AIActions is independent of ordinary metadata lookup.
type AIActions interface {
	TestAIConnection(metadata.AISettings, uint64)
	RecognizeAI(library.Track, uint64)
}
type AITestState struct {
	RequestID uint64
	Running   bool
	Message   string
}

func (s *State) AITest() AITestState { return s.aiTest }
func (s *State) TestAI(next metadata.AISettings) {
	if s.aiTest.Running {
		return
	}
	actions, ok := s.metadata().(AIActions)
	if !ok {
		s.aiTest.Message = "本次构建未接入 AI 测试"
		return
	}
	s.aiTest.RequestID++
	s.aiTest.Running, s.aiTest.Message = true, "正在测试当前填写的配置…"
	actions.TestAIConnection(next, s.aiTest.RequestID)
}
func (s *State) FinishAITest(id uint64, message string) {
	if id != s.aiTest.RequestID {
		return
	}
	s.aiTest.Running, s.aiTest.Message = false, message
}
func (s *State) RecognizeMatchAI() {
	m := &s.match
	if !m.Open || m.Loading || m.Busy || m.AIWorking {
		return
	}
	if !s.ai.Ready() {
		m.Error = "请先在设置中启用并保存 AI 识别配置"
		return
	}
	actions, ok := s.metadata().(AIActions)
	if !ok {
		m.Error = "本次构建未接入 AI 识别"
		return
	}
	m.RequestID++
	m.AIWorking, m.AISuggestion, m.Error, m.Message = true, nil, "", "正在识别，尚未修改曲库…"
	track := m.Track
	track.Title, track.Artist, track.Album = m.Title, m.Artist, m.Album
	actions.RecognizeAI(track, m.RequestID)
}
func (s *State) FinishAIRecognition(id uint64, result *metadata.AIResult, message string) {
	if !s.match.Open || id != s.match.RequestID {
		return
	}
	s.match.AIWorking, s.match.Message = false, ""
	s.match.AISuggestion, s.match.Error = result, message
}
func (s *State) UseAIRecognition() {
	m := &s.match
	if m.AISuggestion == nil || m.Loading || m.Busy || m.AIWorking {
		return
	}
	m.Title, m.Artist, m.Album = m.AISuggestion.Title, m.AISuggestion.Artist, m.AISuggestion.Album
	m.AISuggestion = nil
	s.SearchMatch()
}
