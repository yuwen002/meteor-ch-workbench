package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEnqueueQueryEventAppliesBackpressure(t *testing.T) {
	queue := make(chan queryEvent, 1)
	first := queryEvent{seq: 1}
	second := queryEvent{seq: 2}
	queue <- first

	done := make(chan error, 1)
	go func() {
		done <- enqueueQueryEvent(context.Background(), queue, second)
	}()

	select {
	case err := <-done:
		t.Fatalf("enqueueQueryEvent() returned before queue space was available: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	gotFirst := <-queue
	if gotFirst.seq != first.seq {
		t.Fatalf("first event seq = %d, want %d", gotFirst.seq, first.seq)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("enqueueQueryEvent() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("enqueueQueryEvent() did not resume after queue space became available")
	}

	gotSecond := <-queue
	if gotSecond.seq != second.seq {
		t.Fatalf("second event seq = %d, want %d", gotSecond.seq, second.seq)
	}
}

func TestEnqueueQueryEventStopsWhenContextIsCanceled(t *testing.T) {
	queue := make(chan queryEvent, 1)
	queue <- queryEvent{seq: 1}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- enqueueQueryEvent(ctx, queue, queryEvent{seq: 2})
	}()

	select {
	case err := <-done:
		t.Fatalf("enqueueQueryEvent() returned before cancellation: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("enqueueQueryEvent() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("enqueueQueryEvent() did not stop after context cancellation")
	}
}

func TestQueryResultQueueCapacityBoundsPendingEvents(t *testing.T) {
	if queryResultQueueCapacity != 8 {
		t.Fatalf("queryResultQueueCapacity = %d, want 8", queryResultQueueCapacity)
	}
	queue := make(chan queryEvent, queryResultQueueCapacity)
	for i := 0; i < queryResultQueueCapacity; i++ {
		if err := enqueueQueryEvent(context.Background(), queue, queryEvent{seq: uint64(i + 1)}); err != nil {
			t.Fatalf("enqueue event %d: %v", i, err)
		}
	}
	if len(queue) != queryResultQueueCapacity {
		t.Fatalf("queued events = %d, want %d", len(queue), queryResultQueueCapacity)
	}
}

func TestEnqueueQueryCompletionDeliversTerminalEventAfterQueueUnblocks(t *testing.T) {
	queue := make(chan queryEvent, 1)
	queue <- queryEvent{seq: 1}
	go func() {
		time.Sleep(10 * time.Millisecond)
		<-queue
	}()

	want := queryEvent{seq: 2, done: true}
	if err := enqueueQueryCompletion(queue, want); err != nil {
		t.Fatalf("enqueue terminal event: %v", err)
	}
	got := <-queue
	if got.seq != want.seq || !got.done {
		t.Fatalf("terminal event = %+v, want %+v", got, want)
	}
}

func TestQueryEventIsApplicableRejectsStaleOrClosedTab(t *testing.T) {
	tests := []struct {
		name      string
		current   bool
		tabExists bool
		want      bool
	}{
		{name: "current query and existing tab", current: true, tabExists: true, want: true},
		{name: "stale query", current: false, tabExists: true, want: false},
		{name: "closed tab", current: true, tabExists: false, want: false},
		{name: "stale query and closed tab", current: false, tabExists: false, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := queryEventIsApplicable(tc.current, tc.tabExists); got != tc.want {
				t.Fatalf("queryEventIsApplicable(%t, %t) = %t, want %t", tc.current, tc.tabExists, got, tc.want)
			}
		})
	}
}
