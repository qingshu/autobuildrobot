package taskflow

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func stopLogs(t *testing.T, l *LogAggregator) {
	t.Helper()
	done := make(chan struct{})
	go func() { l.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop deadlocked")
	}
}

func TestLogAggregatorFinalFlushAndRepeatedStop(t *testing.T) {
	var messages []string
	l := NewLogAggregator(time.Hour, 100, 1024, func(s string) { messages = append(messages, s) })
	if !l.Write("first\n") || !l.Write("tail") {
		t.Fatal("write rejected")
	}
	stopLogs(t, l)
	stopLogs(t, l)
	if len(messages) != 1 || messages[0] != "first\ntail" {
		t.Fatalf("messages=%v", messages)
	}
	if l.Write("after stop") {
		t.Fatal("accepted after stop")
	}
}

func TestLogAggregatorThresholdPanicDoesNotRetryBatch(t *testing.T) {
	var calls atomic.Int32
	l := NewLogAggregator(time.Hour, 1, 1024, func(string) {
		if calls.Add(1) == 1 {
			panic("sent, then panic")
		}
	})
	l.Write("first")
	l.Write("second")
	stopLogs(t, l)
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestLogAggregatorTimerPanicKeepsRunning(t *testing.T) {
	called := make(chan struct{}, 2)
	l := NewLogAggregator(5*time.Millisecond, 100, 1024, func(string) {
		called <- struct{}{}
		panic("timer send failed")
	})
	defer stopLogs(t, l)
	for i := 0; i < 2; i++ {
		l.Write("batch")
		select {
		case <-called:
		case <-time.After(time.Second):
			t.Fatal("timer stopped after panic")
		}
	}
}

func TestLogAggregatorStopFlushPanic(t *testing.T) {
	var calls atomic.Int32
	l := NewLogAggregator(time.Hour, 100, 1024, func(string) { calls.Add(1); panic("final flush failed") })
	l.Write("tail")
	stopLogs(t, l)
	stopLogs(t, l)
	if calls.Load() != 1 {
		t.Fatal("final batch retried")
	}
}

func TestLogAggregatorConcurrentStopAndWrite(t *testing.T) {
	var accepted, sent atomic.Int32
	l := NewLogAggregator(time.Hour, 1, 1024, func(string) { sent.Add(1) })
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if l.Write("line") {
					accepted.Add(1)
				}
			}
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); l.Stop() }()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent stop deadlocked")
	}
	if accepted.Load() != sent.Load() {
		t.Fatalf("accepted=%d sent=%d", accepted.Load(), sent.Load())
	}
}
