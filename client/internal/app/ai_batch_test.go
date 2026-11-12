package app

import (
	"context"
	"fmt"
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"github.com/halunhaku/tingyu/client/internal/ui/model"
	"testing"
)

type batchRecognizer struct {
	calls  int
	fail   bool
	cancel context.CancelFunc
}

func (r *batchRecognizer) CleanFilenames(ctx context.Context, inputs []metadata.AIInput) (map[string]metadata.AIResult, error) {
	r.calls++
	if r.cancel != nil {
		r.cancel()
		return nil, context.Canceled
	}
	if r.fail {
		return nil, &metadata.UpstreamError{Provider: "ai", Status: 401}
	}
	hits := map[string]metadata.AIResult{"unknown-id": {Title: "Must not attach"}}
	for i := len(inputs) - 1; i >= 0; i-- {
		if inputs[i].ID != "3" {
			hits[inputs[i].ID] = metadata.AIResult{Title: "recognized-" + inputs[i].ID}
		}
	}
	return hits, nil
}
func batchTracks() []library.Track {
	tracks := []library.Track{}
	for i := 0; i < 10; i++ {
		tracks = append(tracks, library.Track{ID: fmt.Sprint(i), Title: "original"})
	}
	return tracks
}
func TestBatchAIMapsIDsAndRetainsImmutableSnapshots(t *testing.T) {
	client := &batchRecognizer{}
	snapshots := []model.AIBatchState{}
	runAIRecognition(context.Background(), batchTracks(), client, func(v model.AIBatchState) { snapshots = append(snapshots, v) })
	final := snapshots[len(snapshots)-1]
	if client.calls != 2 || final.Running || final.Done != 10 || len(final.Rows) != 10 {
		t.Fatalf("batch=%+v calls=%d", final, client.calls)
	}
	for i, row := range final.Rows {
		if i == 3 {
			if row.Selected || row.Suggestion.Title != "" {
				t.Fatal("missing result was guessed")
			}
			continue
		}
		if row.Track.Title != "original" || row.Suggestion.Title != "recognized-"+row.Track.ID || !row.Selected {
			t.Fatal("wrong row or original changed")
		}
	}
	if snapshots[0].Rows[0].Suggestion.Title != "" {
		t.Fatal("published snapshot mutated")
	}
}
func TestBatchAIStopsOnAuthenticationFailure(t *testing.T) {
	client := &batchRecognizer{fail: true}
	var final model.AIBatchState
	runAIRecognition(context.Background(), batchTracks(), client, func(v model.AIBatchState) { final = v })
	if client.calls != 1 || final.Running || final.Done != 8 || final.Rows[8].Selected || final.Rows[8].Status != "未处理，可重新识别" {
		t.Fatalf("failed batch=%+v calls=%d", final, client.calls)
	}
}
func TestBatchAICancellationDoesNotPublishUnconfirmedReply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &batchRecognizer{cancel: cancel}
	var final model.AIBatchState
	runAIRecognition(ctx, batchTracks(), client, func(v model.AIBatchState) { final = v })
	if final.Running || final.Done != 0 || client.calls != 1 {
		t.Fatal("cancellation did not stop")
	}
	for _, row := range final.Rows {
		if row.Selected || row.Suggestion.Title != "" {
			t.Fatal("cancelled reply accepted")
		}
	}
}
