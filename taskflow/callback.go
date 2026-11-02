package taskflow

import "runtime/debug"

// WithCallbackPanicHandler gives each submission its own independent panic
// reporter. The original callback is never retried. If the reporter itself
// panics, the worker's outer recovery logs it and keeps the worker alive.
func WithCallbackPanicHandler(callback func(Result), report func(Result, any, []byte)) func(Result) {
	return func(result Result) {
		defer func() {
			if value := recover(); value != nil {
				stack := debug.Stack()
				if report != nil {
					report(result, value, stack)
				} else {
					panic(value)
				}
			}
		}()
		if callback != nil {
			callback(result)
		}
	}
}
