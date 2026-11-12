package model

import (
	"github.com/halunhaku/tingyu/client/internal/library"
	"testing"
)

func TestMatchSearchUsesEditedFieldsWithoutWriting(t *testing.T) {
	actions := &fakeMetadata{}
	s := newMetadataState(actions)
	original := library.Track{ID: "original-id", Title: "Original", Artist: "Original Artist", Album: "Original Album"}
	s.OpenMatch(original)
	s.SetMatchCandidates([]MatchCandidate{{Title: "Old candidate", CoverURL: "https://example.invalid/old.jpg"}}, "")
	s.UseMatchCandidate(0)
	s.Match().Title = " New Song "
	s.Match().Artist = " New Artist "
	s.Match().Album = " New Album "
	before := s.Match().RequestID
	s.SearchMatch()
	if len(actions.matchLoads) != 2 {
		t.Fatalf("loads=%d", len(actions.matchLoads))
	}
	query := actions.matchLoads[1]
	if query.ID != original.ID || query.Title != "New Song" || query.Artist != "New Artist" || query.Album != "New Album" {
		t.Fatalf("query=%+v", query)
	}
	if s.Match().Track != original {
		t.Fatalf("search changed write target: %+v", s.Match().Track)
	}
	if !s.Match().Loading || s.Match().Selected != -1 || len(s.Match().Candidates) != 0 || s.Match().RequestID <= before {
		t.Fatalf("match=%+v", s.Match())
	}
	s.ApplyMatch()
	s.SearchMatch()
	if len(actions.manuals) != 0 || len(actions.matchLoads) != 2 {
		t.Fatal("search must not write or duplicate an in-flight request")
	}
	s.SetMatchCandidatesForRequest(s.Match().RequestID, []MatchCandidate{{Title: "New Song", Artist: "New Artist", Album: "New Album", CoverURL: "https://example.invalid/new.jpg"}}, "")
	if s.Match().Title != " New Song " {
		t.Fatal("search response replaced edited input")
	}
	s.UseMatchCandidate(0)
	s.ApplyMatch()
	if len(actions.manuals) != 1 || actions.manuals[0].title != "New Song" || actions.manuals[0].cover != "https://example.invalid/new.jpg" {
		t.Fatalf("writes=%+v", actions.manuals)
	}
}

func TestMatchSearchRejectsEmptyTitleAndStaleReply(t *testing.T) {
	actions := &fakeMetadata{}
	s := newMetadataState(actions)
	s.OpenMatch(library.Track{ID: "one", Title: "First"})
	old := s.Match().RequestID
	s.CloseMatch()
	s.OpenMatch(library.Track{ID: "two", Title: "Second"})
	current := s.Match().RequestID
	s.SetMatchCandidatesForRequest(old, []MatchCandidate{{Title: "Stale"}}, "")
	if !s.Match().Loading || len(s.Match().Candidates) != 0 {
		t.Fatal("stale response changed reopened dialog")
	}
	s.SetMatchCandidatesForRequest(current, nil, "")
	s.Match().Title = "  "
	s.SearchMatch()
	if s.Match().Error == "" || s.Match().Loading || len(actions.matchLoads) != 2 {
		t.Fatalf("empty title was searched: %+v", s.Match())
	}
	s.Match().Title = "Second"
	s.Match().Artist = ""
	s.Match().Album = ""
	s.SearchMatch()
	if len(actions.matchLoads) != 3 || actions.matchLoads[2].Artist != "" || actions.matchLoads[2].Album != "" {
		t.Fatal("cleared search terms were not respected")
	}
}
