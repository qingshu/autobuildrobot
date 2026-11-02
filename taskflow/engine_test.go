package taskflow

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSequenceStopsOnFailure(t *testing.T) {
	want := errors.New("build failed")
	ran := false
	s := Sequence{Title: "root", Children: []Task{
		Func{Title: "fail", Action: func(context.Context, any) error { return want }},
		Func{Title: "skip", Action: func(context.Context, any) error { ran = true; return nil }},
	}}
	if err := s.Run(context.Background(), nil); !errors.Is(err, want) || ran {
		t.Fatalf("error=%v subsequent task ran=%v", err, ran)
	}
}

func TestPerSubmissionCallbackAndCallbackPanic(t *testing.T) {
	results := make(chan Result, 2)
	e, err := New(Config{Workers: 1, QueueSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	var called atomic.Int32
	id, err := e.SubmitWithCallback(Func{Title: "panic task", Action: func(context.Context, any) error { panic("task failure") }}, nil, func(r Result) {
		results <- r
		called.Add(1)
		if r.Err == nil || r.TaskID == "" {
			t.Error("missing result details")
		}
		panic("notification failed")
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.SubmitWithCallback(Func{Title: "next", Action: func(context.Context, any) error { return nil }}, nil, func(r Result) { results <- r }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if called.Load() != 1 {
		t.Fatal("completion callback lost")
	}
	if first := <-results; first.TaskID != id || first.Err == nil {
		t.Fatal("wrong completion")
	}
	if next := <-results; next.Err != nil {
		t.Fatal("worker stopped after callback panic")
	}
}

func TestDrainAndPanicIsolation(t *testing.T) {
	var results, failures atomic.Int32
	e, err := New(Config{Workers: 1, QueueSize: 4})
	onResult := func(r Result) {
		results.Add(1)
		if r.Err != nil {
			failures.Add(1)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range []Task{
		Func{Title: "panic", Action: func(context.Context, any) error { panic("oops") }},
		Func{Title: "next", Action: func(context.Context, any) error { return nil }},
	} {
		if _, err := e.SubmitWithCallback(job, nil, onResult); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if results.Load() != 2 || failures.Load() != 1 {
		t.Fatal("lost result or worker")
	}
	if _, err := e.SubmitWithCallback(Func{}, nil, nil); !errors.Is(err, ErrClosed) {
		t.Fatal("accepted after close")
	}
}

func TestConcurrentCloseAndSubmit(t *testing.T) {
	e, _ := New(Config{Workers: 2, QueueSize: 8})
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = e.SubmitWithCallback(Func{Title: "noop", Action: func(context.Context, any) error { return nil }}, nil, nil)
			}
		}()
	}
	e.Close()
	wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownCancelsOnDeadline(t *testing.T) {
	started := make(chan struct{})
	e, _ := New(Config{Workers: 1, QueueSize: 1})
	_, _ = e.SubmitWithCallback(Func{Title: "wait", Action: func(ctx context.Context, data any) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}, nil, nil)
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if !errors.Is(e.Shutdown(ctx), context.DeadlineExceeded) {
		t.Fatal("expected deadline")
	}
	select {
	case <-e.Done():
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestExecutionDataThroughSubmitAndNestedSequence(t *testing.T) {
	type params struct{ Version int }
	want := &params{Version: 42}
	seen := make(chan any, 2)
	leaf := Func{Title: "leaf", Action: func(_ context.Context, data any) error {
		seen <- data
		return nil
	}}
	e, err := New(Config{Workers: 1, QueueSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close(); <-e.Done() })
	if _, err := e.SubmitWithCallback(Sequence{Title: "root", Children: []Task{
		Sequence{Title: "nested", Children: []Task{leaf, leaf}},
	}}, want, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case got := <-seen:
			if got != want {
				t.Fatalf("got %v, want original data", got)
			}
		case <-time.After(time.Second):
			t.Fatal("missing task data")
		}
	}
}

func TestCancelRunningAndQueuedTasks(t *testing.T) {
	results := make(chan Result, 3)
	e, err := New(Config{Workers: 1, QueueSize: 3})
	onResult := func(r Result) { results <- r }
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close(); <-e.Done() })
	started := make(chan struct{})
	runningID, err := e.SubmitWithCallback(Func{Title: "running", Action: func(ctx context.Context, _ any) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}, nil, onResult)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("not started")
	}
	var queuedRan atomic.Bool
	queuedID, err := e.SubmitWithCallback(Func{Title: "queued", Action: func(context.Context, any) error {
		queuedRan.Store(true)
		return nil
	}}, nil, onResult)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Cancel(queuedID); err != nil {
		t.Fatal(err)
	}
	if err := e.Cancel(queuedID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SubmitWithCallback(Func{Title: "unaffected", Action: func(context.Context, any) error { return nil }}, nil, onResult); err != nil {
		t.Fatal(err)
	}
	if err := e.Cancel(runningID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if queuedRan.Load() {
		t.Fatal("canceled queued task executed")
	}
	for i := 0; i < 3; i++ {
		r := <-results
		if r.TaskID == runningID || r.TaskID == queuedID {
			if !errors.Is(r.Err, context.Canceled) {
				t.Fatalf("expected cancellation: %+v", r)
			}
		} else if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	if !errors.Is(e.Cancel(runningID), ErrTaskNotFound) {
		t.Fatal("finished task retained")
	}
	if !errors.Is(e.Cancel("unknown"), ErrTaskNotFound) {
		t.Fatal("unknown ID accepted")
	}
}
