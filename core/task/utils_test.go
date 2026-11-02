package task

import (
	"autobuildrobot/models"
	"context"
	"strconv"
	"testing"
	"time"
)

func TestExecCommandRejectedRequestCompletes(t *testing.T) {
	var messages []string
	done := ExecCommand("project", "", "不存在的指令", "", func(msg, _ string) { messages = append(messages, msg) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("rejected request did not complete")
	}
	if len(messages) != 1 {
		t.Fatalf("expected one error notification, got %d", len(messages))
	}
}

func TestExpandCommands(t *testing.T) {
	lookup := func(_, name string) string {
		switch name {
		case "release":
			return "客户端构建：工程A,方法A->nested"
		case "nested":
			return "客户端构建:工程B,方法B"
		case "loop":
			return "loop"
		}
		return ""
	}
	got, err := expandCommands("project", "release", lookup, map[string]bool{}, 0)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %v, %v", got, err)
	}
	if got[1].CmdParams[0] != "工程B" {
		t.Fatal("nested parameters lost")
	}
	for _, input := range []string{"loop", "未知", "客户端构建:a,b->", ""} {
		if _, err := expandCommands("project", input, lookup, map[string]bool{}, 0); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestCommandDefinitionsAndRegistration(t *testing.T) {
	seen := make(map[string]bool)
	for command := 0; command < models.CommandType_Max; command++ {
		name := models.GetCommandNameByType(command)
		if name == "" || seen[name] {
			t.Fatalf("missing or duplicate command name: %d %q", command, name)
		}
		seen[name] = true
		if buildFuncs[command] == nil {
			t.Fatalf("command %d is not registered", command)
		}
		for _, input := range []string{name, strconv.Itoa(command)} {
			p := analysisOneCommand("project", input)
			if p == nil || p.CmdType != command {
				t.Fatalf("command %q resolved to %+v", input, p)
			}
		}
	}
}

func TestNumericCommandAndMissingSeparator(t *testing.T) {
	p := analysisOneCommand("project", "7:工程,方法")
	if p == nil || p.CmdType != models.CommandType_AutoBuildClient {
		t.Fatal("numeric command mismatch")
	}
	if p := analysisOneCommand("project", "客户端构建"); p == nil || len(p.CmdParams) != 0 {
		t.Fatal("missing separator not parsed safely")
	}
}

func TestInvalidUnityInputs(t *testing.T) {
	for _, p := range []*Param{nil, {CmdType: models.CommandType_AutoBuildClient}, {CmdType: models.CommandType_AutoBuildClient, CmdParams: []string{"project"}}} {
		if _, err := getShellTask(p); err == nil {
			t.Fatal("invalid parameters accepted")
		}
	}
	for _, data := range []any{nil, Param{}, (*Param)(nil), shellRequest{command: models.CommandType_AutoBuildClient}, shellRequest{command: models.CommandType_AutoBuildClient, script: "autoBuildClient.sh", root: ".", args: []string{"engine"}}} {
		if err := shellBuild(context.Background(), data); err == nil {
			t.Fatal("invalid runtime data accepted")
		}
	}
	if _, err := GetTaskForUser("project", "", "", "客户端构建:a,b", nil); err == nil {
		t.Fatal("empty executor authorized")
	}
}
