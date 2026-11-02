package taskflow

import (
	log "autobuildrobot/mylog"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// 日志聚合器
type LogAggregator struct {
	mu      sync.Mutex
	pending strings.Builder
	count   int
	closed  bool

	maxLines int
	maxBytes int
	// send must return promptly and must not call this aggregator's methods.
	// Calls are serialized. A panicking batch is logged, not automatically retried.
	send func(string)

	stop chan struct{}
	done chan struct{}
}

func NewLogAggregator(interval time.Duration, maxLines, maxBytes int, send func(string)) *LogAggregator {
	if maxLines <= 0 || maxBytes <= 0 || send == nil || interval <= 0 {
		panic("无效的日志聚合器参数！")
	}
	l := &LogAggregator{
		maxLines: maxLines,
		maxBytes: maxBytes,
		send:     send,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go l.run(interval)
	return l
}

func (l *LogAggregator) Write(line string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return false
	}
	l.pending.WriteString(line)
	l.count++

	if l.count >= l.maxLines || l.pending.Len() >= l.maxBytes {
		l.flushLocked()
	}
	return true
}

func (l *LogAggregator) Stop() {
	func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if !l.closed {
			l.closed = true
			close(l.stop)
			l.flushLocked()
		}
	}()
	<-l.done
}

func (l *LogAggregator) run(interval time.Duration) {
	defer close(l.done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			func() {
				l.mu.Lock()
				defer l.mu.Unlock()
				if !l.closed {
					l.flushLocked()
				}
			}()
		}
	}
}

// flushLocked requires mu. Remove the batch before invoking user code so a
// panic after a successful send cannot cause the same batch to be sent again.
func (l *LogAggregator) flushLocked() {
	if l.pending.Len() <= 0 {
		return
	}
	msg := l.pending.String()
	l.pending.Reset()
	l.count = 0
	defer func() {
		if value := recover(); value != nil {
			log.Error(fmt.Sprintf("LogAggregator send panic: %v\n%s", value, debug.Stack()))
		}
	}()
	l.send(msg)
}
