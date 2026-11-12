package model

import (
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"testing"
)

type aiActions struct {
	fakeMetadata
	tested        metadata.AISettings
	testCalls     int
	recognized    library.Track
	recognitionID uint64
}

func (a *aiActions) TestAIConnection(settings metadata.AISettings, id uint64) {
	a.tested = settings
	a.testCalls++
}
func (a *aiActions) RecognizeAI(track library.Track, id uint64) {
	a.recognized = track
	a.recognitionID = id
}

func TestAIConnectionTestsDraftWithoutSaving(t *testing.T) {
	s := New(nil, nil, "", nil)
	a := &aiActions{}
	s.SetMetadataActions(a)
	saved := metadata.DefaultAISettings()
	s.SetAISettings(saved, "")
	draft := metadata.AISettings{BaseURL: "http://localhost/v1", Model: "draft", APIKey: "test-only"}
	s.TestAI(draft)
	s.TestAI(draft)
	if a.testCalls != 1 || a.tested != draft || s.AI() != saved || len(a.ai) != 0 || !s.AITest().Running {
		t.Fatal("draft was saved or request duplicated")
	}
	s.FinishAITest(s.AITest().RequestID+1, "stale")
	if !s.AITest().Running {
		t.Fatal("stale response changed state")
	}
	s.FinishAITest(s.AITest().RequestID, "success")
	if s.AITest().Running || s.AITest().Message != "success" {
		t.Fatal("test result missing")
	}
}

func TestAIRecognitionNeedsConfirmationBeforeSearchOrWrite(t *testing.T) {
	s := New(nil, nil, "", nil)
	a := &aiActions{}
	s.SetMetadataActions(a)
	config := metadata.DefaultAISettings()
	config.Enabled = true
	s.SetAISettings(config, "")
	original := library.Track{ID: "one", Title: "noisy.mp3"}
	s.OpenMatch(original)
	s.SetMatchCandidates(nil, "")
	s.RecognizeMatchAI()
	id := a.recognitionID
	s.ApplyMatch()
	s.SearchMatch()
	if !s.Match().AIWorking || len(a.manuals) != 0 || len(a.matchLoads) != 1 {
		t.Fatal("operation ran during recognition")
	}
	result := &metadata.AIResult{Title: "Recognized Live", Artist: "Artist", Album: "Album"}
	s.FinishAIRecognition(id, result, "")
	if s.Match().Title != original.Title || len(a.matchLoads) != 1 || len(a.manuals) != 0 {
		t.Fatal("preview modified metadata")
	}
	s.UseAIRecognition()
	if len(a.matchLoads) != 2 || a.matchLoads[1].Title != result.Title || a.matchLoads[1].ID != original.ID || len(a.manuals) != 0 {
		t.Fatal("confirmation did not search without writing")
	}
	s.CloseMatch()
	s.OpenMatch(library.Track{ID: "two", Title: "Other"})
	s.FinishAIRecognition(id, result, "")
	if s.Match().AISuggestion != nil {
		t.Fatal("stale recognition leaked into new dialog")
	}
}
