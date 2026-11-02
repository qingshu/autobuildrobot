package taskflow

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

var (
	ErrClosed       = errors.New("taskflow: engine closed")
	ErrQueueFull    = errors.New("taskflow: queue full")
	ErrTaskNotFound = errors.New("taskflow: task not found or already finished")
)

type Result struct {
	TaskID     string
	WorkerID   int
	Name       string
	Err        error
	Duration   time.Duration
	PanicStack []byte
}

type Config struct {
	Workers   int //工作线程数量
	QueueSize int //共享通道“长”度
}

// request keeps execution data separate from a reusable task definition.
// Data is passed as-is; callers must synchronize shared mutable values.
type request struct {
	onResult func(Result)
	id       string
	ctx      context.Context
	task     Task
	data     any
}

type Engine struct {
	nextID      uint64
	active      map[string]context.CancelFunc
	mu          sync.Mutex
	closed      bool
	queue       chan request
	taskCtx     context.Context
	cancelTasks context.CancelFunc
	workers     sync.WaitGroup
	done        chan struct{}
}

// New starts the workers. Call SubmitWithCallback to enqueue tasks.
func New(config Config) (*Engine, error) {
	if config.Workers <= 0 || config.QueueSize <= 0 {
		return nil, fmt.Errorf("workers and queue size must be positive")
	}
	tc, cancel := context.WithCancel(context.Background())
	e := &Engine{queue: make(chan request, config.QueueSize),
		taskCtx: tc, cancelTasks: cancel,
		done: make(chan struct{}), active: make(map[string]context.CancelFunc)}
	for id := 1; id <= config.Workers; id++ {
		e.workers.Add(1)
		go (&Worker{id: id, engine: e}).run()
	}
	return e, nil
}

// SubmitWithCallback registers completion before the task can start.
// It is nonblocking and returns ErrQueueFull on overflow. Callbacks may run
// concurrently and must return promptly. A nil callback is allowed.
func (e *Engine) SubmitWithCallback(task Task, data any, onResult func(Result)) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return "", ErrClosed
	}
	if task == nil {
		return "", fmt.Errorf("nil task")
	}
	e.nextID++
	id := fmt.Sprintf("task-%d", e.nextID)
	ctx, cancel := context.WithCancel(e.taskCtx)
	e.active[id] = cancel
	select {
	case e.queue <- request{id: id, ctx: ctx, task: task, data: data, onResult: onResult}:
		return id, nil
	default:
		delete(e.active, id)
		cancel()
		return "", ErrQueueFull
	}
}

// Cancel signals cancellation; it does not wait for execution to finish.
// Queued tasks stay in the channel until dequeued, then report cancellation
// without calling Run. Repeated cancellation is safe while the task is active.
func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	cancel, ok := e.active[id]
	e.mu.Unlock()
	if !ok {
		return ErrTaskNotFound
	}
	cancel()
	return nil
}

func (e *Engine) finish(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cancel, ok := e.active[id]; ok {
		delete(e.active, id)
		cancel()
	}
}

// Close stops admission; workers drain accepted tasks.
// It is safe to call multiple times and concurrently with SubmitWithCallback.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.closed = true
	close(e.queue)
	go func() {
		e.workers.Wait()
		e.cancelTasks()
		close(e.done)
	}()
}

// Shutdown drains tasks until ctx expires, then signals task cancellation.
// Cancellation is cooperative: timeout does not guarantee every goroutine has exited.
func (e *Engine) Shutdown(ctx context.Context) error {
	e.Close()
	select {
	case <-e.done:
		return nil
	case <-ctx.Done():
		e.cancelTasks()
		return ctx.Err()
	}
}

// Done closes once workers and their callbacks have returned.
func (e *Engine) Done() <-chan struct{} { return e.done }

type Worker struct {
	id     int
	engine *Engine
}

func (w *Worker) run() {
	defer w.engine.workers.Done()
	for item := range w.engine.queue {
		result := w.execute(item)
		log.Printf("taskflow task=%s name=%s duration=%s error=%v", result.TaskID, result.Name, result.Duration, result.Err)
		if len(result.PanicStack) > 0 {
			log.Printf("%s", result.PanicStack)
		}
		notifyResult(item.onResult, result)
	}
}

func notifyResult(callback func(Result), result Result) {
	if callback == nil {
		return
	}
	defer func() {
		if value := recover(); value != nil {
			log.Printf("taskflow callback panic for %s: %v\n%s", result.TaskID, value, debug.Stack())
		}
	}()
	callback(result)
}

func (w *Worker) execute(item request) (r Result) {
	r.TaskID = item.id
	defer w.engine.finish(item.id)
	r.WorkerID = w.id
	start := time.Now()
	defer func() {
		r.Duration = time.Since(start)
		if value := recover(); value != nil {
			r.Err = fmt.Errorf("task panic: %v", value)
			r.PanicStack = debug.Stack()
		}
	}()
	r.Name = item.task.Name()
	if err := item.ctx.Err(); err != nil {
		r.Err = err
		return
	}
	r.Err = item.task.Run(item.ctx, item.data)
	return
}
