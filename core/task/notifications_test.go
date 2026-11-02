package task

import (
	"autobuildrobot/taskflow"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCallbackPanicSendsIndependentDingAlert(t *testing.T) {
	for _, code := range []int{0, 123} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			var requests atomic.Int32
			var text string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				var payload struct {
					Text struct {
						Content string `json:"content"`
					} `json:"text"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				text = payload.Text.Content
				fmt.Fprintf(w, `{"errcode":%d,"errmsg":"test"}`, code)
			}))
			defer server.Close()
			e, err := taskflow.New(taskflow.Config{Workers: 1, QueueSize: 2})
			if err != nil {
				t.Fatal(err)
			}
			var callbacks, next atomic.Int32
			id, err := e.SubmitWithCallback(taskflow.Func{Title: "build", Action: func(context.Context, any) error { return nil }}, nil,
				taskflow.WithCallbackPanicHandler(func(taskflow.Result) {
					callbacks.Add(1)
					panic("broken notification")
				}, callbackPanicReporter(server.URL, "project", "executor", "")))
			if err != nil {
				t.Fatal(err)
			}
			_, err = e.SubmitWithCallback(taskflow.Func{Title: "next", Action: func(context.Context, any) error { return nil }}, nil, func(taskflow.Result) { next.Add(1) })
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := e.Shutdown(ctx); err != nil {
				t.Fatal(err)
			}
			if callbacks.Load() != 1 || requests.Load() != 1 || next.Load() != 1 {
				t.Fatal("retried callback, duplicate alert, or lost worker")
			}
			if !strings.Contains(text, id) || !strings.Contains(text, "broken notification") {
				t.Fatalf("missing alert details: %q", text)
			}
		})
	}
}
