package app

import (
	"context"
	"testing"
)

func TestExplorerRequestStateBeginCancelsPreviousRequest(t *testing.T) {
	var state explorerRequestState
	firstCtx, firstCancel := context.WithCancel(context.Background())
	firstSeq := state.Begin(firstCancel)
	if firstCtx.Err() != nil {
		t.Fatal("first request canceled immediately")
	}

	secondCtx, secondCancel := context.WithCancel(context.Background())
	secondSeq := state.Begin(secondCancel)
	defer secondCancel()

	if firstCtx.Err() != context.Canceled {
		t.Fatal("beginning a new request must cancel the previous request")
	}
	if secondCtx.Err() != nil {
		t.Fatal("new request must remain active")
	}
	if secondSeq <= firstSeq || state.Sequence() != secondSeq {
		t.Fatalf("sequence after Begin = %d, first=%d second=%d", state.Sequence(), firstSeq, secondSeq)
	}
}

func TestExplorerRequestStateCancelInvalidatesPendingResponse(t *testing.T) {
	var state explorerRequestState
	ctx, cancel := context.WithCancel(context.Background())
	requestSeq := state.Begin(cancel)

	state.Cancel()
	if ctx.Err() != context.Canceled {
		t.Fatal("Cancel must cancel the active request")
	}
	if state.Sequence() <= requestSeq {
		t.Fatalf("Cancel must invalidate sequence: current=%d request=%d", state.Sequence(), requestSeq)
	}
	if state.cancel != nil {
		t.Fatal("Cancel must clear the cancel function")
	}
}

func TestExplorerRequestStateCanBeginAfterCancel(t *testing.T) {
	var state explorerRequestState
	state.Cancel()
	ctx, cancel := context.WithCancel(context.Background())
	seq := state.Begin(cancel)
	defer cancel()

	if seq != state.Sequence() {
		t.Fatalf("Begin sequence = %d, current = %d", seq, state.Sequence())
	}
	if ctx.Err() != nil {
		t.Fatal("request after Cancel must remain active")
	}
}
