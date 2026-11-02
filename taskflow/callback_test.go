package taskflow

import (
	"context"
	"testing"
	"time"
)

func TestPanicReporterFailureDoesNotStopWorker(t *testing.T) {
	e, err := New(Config{Workers: 1, QueueSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	called := make(chan struct{}, 1)
	_, err = e.SubmitWithCallback(Func{Title: "first", Action: func(context.Context, any) error { return nil }}, nil,
		WithCallbackPanicHandler(func(Result) { panic("callback") }, func(r Result, value any, stack []byte) {
			if r.TaskID == "" || value != "callback" || len(stack) == 0 {
				t.Error("missing panic details")
			}
			panic("reporter also failed")
		}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.SubmitWithCallback(Func{Title: "next", Action: func(context.Context, any) error { return nil }}, nil, func(Result) { called <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	default:
		t.Fatal("worker lost after reporter panic")
	}
}
