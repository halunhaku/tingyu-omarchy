package model

import (
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"strings"
)

type LyricActions interface {
	SearchLyricCandidates(library.Track, metadata.LyricsQuery, uint64)
	LoadLyricPreview(metadata.LyricsQuery, metadata.LyricCandidate, uint64)
	ImportLyricFile(uint64)
	SaveLyricText(library.Track, string, uint64)
}
type LyricSearchState struct {
	RequestID                  uint64
	Open                       bool
	Track                      library.Track
	Title, Artist, Album       string
	Searching, Loading, Saving bool
	Candidates                 []metadata.LyricCandidate
	Selected                   int
	Text, Message              string
}

func (s *State) LyricSearch() *LyricSearchState { return &s.lyricSearch }
func (s *State) OpenLyricSearch(track library.Track) {
	if s.lyricSearch.Saving {
		return
	}
	s.CloseMatch()
	if text, ok := s.lyricOverrides[track.ID]; ok {
		track.Lyrics = text
	}
	s.lyricSearch = LyricSearchState{RequestID: s.lyricSearch.RequestID + 1, Open: true, Track: track, Title: track.Title, Artist: track.Artist, Album: track.Album, Selected: -1, Text: track.Lyrics}
	s.SearchLyrics()
}
func (s *State) CloseLyricSearch()          { s.lyricSearch.Open = false }
func (s *State) lyricActions() LyricActions { a, _ := s.metadata().(LyricActions); return a }
func (s *State) lyricQuery() metadata.LyricsQuery {
	v := s.lyricSearch
	return metadata.LyricsQuery{Title: strings.TrimSpace(v.Title), Artist: strings.TrimSpace(v.Artist), Album: strings.TrimSpace(v.Album), Duration: v.Track.Duration}
}
func (s *State) SearchLyrics() {
	v := &s.lyricSearch
	if !v.Open || v.Loading || v.Saving || v.Searching {
		return
	}
	if strings.TrimSpace(v.Title) == "" {
		v.Message = "请填写歌名后搜索"
		return
	}
	a := s.lyricActions()
	if a == nil {
		v.Message = "本次构建未接入歌词搜索"
		return
	}
	v.RequestID++
	v.Searching = true
	v.Candidates = nil
	v.Selected = -1
	v.Message = "正在搜索多个歌词来源…"
	a.SearchLyricCandidates(v.Track, s.lyricQuery(), v.RequestID)
}
func (s *State) SetLyricCandidates(id uint64, hits []metadata.LyricCandidate, message string) {
	v := &s.lyricSearch
	if !v.Open || id != v.RequestID {
		return
	}
	v.Searching = false
	v.Candidates = hits
	v.Message = message
}
func (s *State) PreviewLyricCandidate(index int) {
	v := &s.lyricSearch
	if !v.Open || v.Loading || v.Saving || index < 0 || index >= len(v.Candidates) {
		return
	}
	a := s.lyricActions()
	if a == nil {
		return
	}
	v.RequestID++
	v.Searching = false
	v.Loading = true
	v.Selected = index
	v.Text = ""
	v.Message = "正在加载歌词预览…"
	a.LoadLyricPreview(s.lyricQuery(), v.Candidates[index], v.RequestID)
}
func (s *State) SetLyricText(id uint64, text, message string) {
	v := &s.lyricSearch
	if !v.Open || id != v.RequestID {
		return
	}
	v.Loading = false
	v.Message = message
	if strings.TrimSpace(text) != "" {
		v.Text = text
	}
}
func (s *State) ImportLyrics() {
	v := &s.lyricSearch
	if !v.Open || v.Loading || v.Saving {
		return
	}
	a := s.lyricActions()
	if a == nil {
		return
	}
	v.RequestID++
	v.Searching = false
	v.Loading = true
	v.Selected = -1
	v.Message = "请选择 LRC 或文本文件；导入后可预览再保存"
	a.ImportLyricFile(v.RequestID)
}
func (s *State) SaveLyrics() {
	v := &s.lyricSearch
	if !v.Open || v.Loading || v.Saving {
		return
	}
	if strings.TrimSpace(v.Text) == "" {
		v.Message = "请先选择、导入或粘贴歌词"
		return
	}
	if len(v.Text) > 2<<20 || strings.ContainsRune(v.Text, 0) {
		v.Message = "歌词内容无效或超过 2 MB，请检查后重试"
		return
	}
	a := s.lyricActions()
	if a == nil {
		v.Message = "本次构建未接入歌词保存"
		return
	}
	v.RequestID++
	v.Searching = false
	v.Saving = true
	v.Message = "正在保存歌词…"
	a.SaveLyricText(v.Track, v.Text, v.RequestID)
}
func (s *State) LyricsSaved(id uint64, message string, success bool) {
	v := &s.lyricSearch
	if id != v.RequestID {
		return
	}
	v.Saving = false
	v.Message = message
	if success {
		v.Track.Lyrics = v.Text
		if s.lyricOverrides == nil {
			s.lyricOverrides = map[string]string{}
		}
		s.lyricOverrides[v.Track.ID] = v.Text
		if s.player.Track != nil && s.player.Track.ID == v.Track.ID {
			track := *s.player.Track
			track.Lyrics = v.Text
			s.player.Track = &track
		}
		s.Invalidate()
		s.SetNotice(message)
	}
}
