package taskflow

import (
	"context"
	"testing"
)

func TestBoundTaskRunsInnerWithBoundData(t *testing.T) {
	calls := 0
	job := BindTask(Func{Title: "bound", Action: func(_ context.Context, data any) error {
		calls++
		if data != 42 {
			t.Fatalf("got %v", data)
		}
		return nil
	}}, 42)
	if err := job.Run(context.Background(), "ignored"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}
