package task

import (
	"autobuildrobot/models"
	"context"
	"testing"
)

func TestShellCommandsRejectMissingParameters(t *testing.T) {
	for _, command := range []int{models.CommandType_AutoBuildClient, models.CommandType_SvnMerge, models.CommandType_UpdateAndRestartSvr, models.CommandType_BuildPbMsg, models.CommandType_CloseSvr, models.CommandType_UpdateTable} {
		if buildFuncs[command] == nil {
			t.Fatalf("command %d is not registered", command)
		}
		for _, params := range [][]string{nil, {""}, {" "}, {"", "service"}, {"project", " "}} {
			// Single-argument commands allow an optional empty extra parameter.
			if (command == models.CommandType_UpdateTable || command == models.CommandType_BuildPbMsg) && len(params) == 2 && params[0] != "" {
				continue
			}
			if _, err := buildFuncs[command](&Param{CmdType: command, CmdParams: params}); err == nil {
				t.Fatalf("command %d accepted %q", command, params)
			}
		}
		if command == models.CommandType_AutoBuildClient || command == models.CommandType_SvnMerge || command == models.CommandType_UpdateAndRestartSvr || command == models.CommandType_CloseSvr {
			if _, err := buildFuncs[command](&Param{CmdType: command, CmdParams: []string{"project"}}); err == nil {
				t.Fatalf("command %d accepted missing second argument", command)
			}
		}
	}
	if _, err := getShellTask(nil); err == nil {
		t.Fatal("accepted nil parameters")
	}
	if _, err := getShellTask(&Param{CmdType: models.CommandType_Help}); err == nil {
		t.Fatal("accepted unsupported command")
	}
}

func TestShellBuildRejectsInvalidRuntimeData(t *testing.T) {
	for _, data := range []any{nil, Param{}, (*Param)(nil), shellRequest{}, shellRequest{script: "script.sh", root: "root"}} {
		if err := shellBuild(context.Background(), data); err == nil {
			t.Fatalf("accepted %T", data)
		}
	}
}
