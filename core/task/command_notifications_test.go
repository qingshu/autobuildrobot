package task

import (
	"autobuildrobot/taskflow"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCommandNotificationsForExpandedTemplate(t *testing.T) {
	lookup := func(_, name string) string {
		if name == "更新外网策划包" {
			return "更新表格:研发表格->分支合并:开发分支合并到策划分支->客户端构建:外网策划包,1,0->输出热更资源:外网策划包,0->上传热更资源到测试:外网策划包,0"
		}
		return ""
	}
	params, err := expandCommands("project", "更新外网策划包", lookup, map[string]bool{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	type message struct{ text, phone string }
	var messages []message
	send := func(text, phone string) { messages = append(messages, message{text, phone}) }
	sequence := taskflow.Sequence{}
	for i, p := range params {
		display := fmt.Sprintf("指令%d:%s", i+1, p.RawParams)
		job := taskflow.Func{Title: display, Action: func(context.Context, any) error { send("执行期间日志", ""); return nil }}
		sequence.Children = append(sequence.Children, withCommandNotifications(job, display, i+1, len(params), "builderA", "13800000000", send))
	}
	if err := sequence.Run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 15 {
		t.Fatalf("expected 15 messages, got %d", len(messages))
	}
	for i := range params {
		start, log, finish := messages[i*3], messages[i*3+1], messages[i*3+2]
		if !strings.Contains(start.text, fmt.Sprintf("开始执行%d/5", i+1)) || start.phone != "" || log.text != "执行期间日志" || !strings.Contains(finish.text, "完毕！") || finish.phone != "13800000000" {
			t.Fatalf("incorrect command notifications: %v", messages[i*3:i*3+3])
		}
	}
}

func TestCommandNotificationsFailureAndCancellation(t *testing.T) {
	for _, failure := range []error{errors.New("构建失败"), context.Canceled, context.DeadlineExceeded} {
		var messages []string
		var phones []string
		send := func(msg, phone string) { messages = append(messages, msg); phones = append(phones, phone) }
		job := withCommandNotifications(taskflow.Func{Title: "first", Action: func(context.Context, any) error { return failure }}, "更新表格:研发表格", 1, 2, "builderA", "13800000000", send)
		sequence := taskflow.Sequence{Children: []taskflow.Task{job, taskflow.Func{Action: func(context.Context, any) error { t.Fatal("ran after failure"); return nil }}}}
		if err := sequence.Run(context.Background(), nil); !errors.Is(err, failure) {
			t.Fatalf("lost error: %v", err)
		}
		if len(messages) != 2 || strings.Contains(messages[1], "完毕") || phones[1] != "13800000000" {
			t.Fatalf("incorrect failure notification: %v %v", messages, phones)
		}
	}
}
