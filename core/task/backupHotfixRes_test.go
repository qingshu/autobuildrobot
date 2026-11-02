package task

import (
	"autobuildrobot/models"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMaintenanceCancellationAndDisabledDelete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, command := range []int{models.CommandType_CloseRobot, models.CommandType_BackupSvn, models.CommandType_DelHotfixRes} {
		if buildFuncs[command] == nil {
			t.Fatalf("missing command %d", command)
		}
		job, err := buildFuncs[command](&Param{CmdType: command})
		if err != nil {
			t.Fatal(err)
		}
		if err := job.Run(ctx, nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled operation ran: %v", err)
		}
	}
	job, err := getDelHotfixResTask(&Param{CmdType: models.CommandType_DelHotfixRes})
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "暂不实现") {
		t.Fatalf("delete must remain disabled: %v", err)
	}
	for _, builder := range []GetBuildFunc{getCloseRobotTask, getBackupSvnTask, getDelHotfixResTask} {
		if _, err := builder(nil); err == nil {
			t.Fatal("accepted nil")
		}
		if _, err := builder(&Param{CmdType: models.CommandType_Help}); err == nil {
			t.Fatal("accepted unsupported command")
		}
	}
}
