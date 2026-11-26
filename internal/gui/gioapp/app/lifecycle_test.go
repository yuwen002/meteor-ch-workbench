package app

import (
	"context"
	"testing"
	"time"
)

func TestGoWorkerWaitsForTaskCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	state := &workspace{lifecycleCtx: ctx}
	started := make(chan struct{})
	release := make(chan struct{})
	state.goWorker(func() {
		close(started)
		<-release
	})
	<-started

	waited := make(chan struct{})
	go func() {
		state.workers.Wait()
		close(waited)
	}()
	select {
	case <-waited:
		t.Fatal("Wait returned before the background task exited")
	case <-time.After(20 * time.Millisecond):
	}

	close(release)
	select {
	case <-waited:
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after the background task exited")
	}
}

func TestGoWorkerDoesNotStartAfterLifecycleCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state := &workspace{lifecycleCtx: ctx}

	started := false
	state.goWorker(func() { started = true })
	state.workers.Wait()
	if started {
		t.Fatal("background task started after lifecycle cancellation")
	}
}

func TestLifecycleParentUsesWorkspaceContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	state := &workspace{lifecycleCtx: ctx}

	if got := state.lifecycleParent(); got != ctx {
		t.Fatal("lifecycleParent did not return the workspace lifecycle context")
	}
	if got := (*workspace)(nil).lifecycleParent(); got == nil {
		t.Fatal("lifecycleParent returned a nil fallback context")
	}
}
