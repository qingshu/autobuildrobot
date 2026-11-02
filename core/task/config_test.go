package task

import (
	"autobuildrobot/models"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestTemplateConfigContainsCommandSequence(t *testing.T) {
	raw := `模板指令：{"Name":"更新外网策划包","Cmd":"更新表格：研发表格->分支合并：开发分支合并到策划分支->客户端构建：外网策划包,1,0->输出热更资源：外网策划包,0->上传热更资源到测试：外网策划包,0","Describe":"更新并上传热更资源到公司外网中文测试包"}`
	err, params := getTaskParams("project", raw)
	if err != nil || len(params) != 1 {
		t.Fatalf("template config rejected: %v, %v", params, err)
	}
	if params[0].CmdType != models.CommandType_TemplateCmd {
		t.Fatal("wrong command type")
	}
	var template models.CmdTemplateModel
	if err := json.Unmarshal([]byte(params[0].RawParams), &template); err != nil {
		t.Fatal(err)
	}
	lookup := func(_, name string) string {
		if name == template.Name {
			return template.Cmd
		}
		return ""
	}
	commands, err := expandCommands("project", template.Name, lookup, map[string]bool{}, 0)
	if err != nil || len(commands) != 5 {
		t.Fatalf("template sequence lost: %v, %v", commands, err)
	}
	if got := commands[1].CmdParams; len(got) != 2 || got[0] != "开发分支" || got[1] != "策划分支" {
		t.Fatalf("template merge branches not parsed: %q", got)
	}
	for _, input := range []string{raw + "->帮助", "帮助->" + raw, `模板指令:{"Name":"例","Cmd":"帮助","Describe":"含转义引号\"->内容"}->帮助`} {
		if err, _ := getTaskParams("project", input); err == nil {
			t.Fatalf("accepted external config sequence: %s", input)
		}
	}
	for _, input := range []string{raw, `模板指令:{"Name":"例","Cmd":"帮助->帮助","Describe":"引号\"->内容"}`, `模板指令:{"Name":"例","Cmd":"帮助->帮助","Describe":"反斜杠\\"}`} {
		if err, _ := getTaskParams("project", input); err != nil {
			t.Fatalf("rejected JSON string separator: %v", err)
		}
	}
}

func TestConfigRawParameters(t *testing.T) {
	raw := "{\"Name\":\"说明：甲，乙；丙\",\n\"Value\":\"a,b\"}"
	for _, separator := range []string{":", "："} {
		p := analysisOneCommand("project", "项目"+separator+raw)
		if p == nil || p.RawParams != strings.ReplaceAll(raw, "：", ":") {
			t.Fatalf("raw config changed: %+v", p)
		}
	}
	params, err := expandCommands("project", "svn工程：条件：甲，乙", func(string, string) string { return "" }, map[string]bool{}, 0)
	if err != nil || len(params) != 1 || params[0].RawParams != "条件:甲，乙" {
		t.Fatalf("query changed: %v %v", params, err)
	}
}

func TestConfigTasksDeferChangesAndHonorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, command := range []int{models.CommandType_ProjectConfig, models.CommandType_SvnProjectConfig, models.CommandType_CdnConfig, models.CommandType_SvrProgressConfig, models.CommandType_SvrMachineConfig, models.CommandType_User, models.CommandType_UserGroup, models.CommandType_TemplateCmd} {
		if buildFuncs[command] == nil {
			t.Fatalf("missing command %d", command)
		}
		job, err := getConfigTask(&Param{CmdType: command, RawParams: "{invalid JSON"})
		if err != nil {
			t.Fatalf("construction executed update: %v", err)
		}
		if err := job.Run(ctx, nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled task ran: %v", err)
		}
	}
	if _, err := getConfigTask(nil); err == nil {
		t.Fatal("accepted nil")
	}
	if _, err := getConfigTask(&Param{CmdType: models.CommandType_Help}); err == nil {
		t.Fatal("accepted non-config command")
	}
}
