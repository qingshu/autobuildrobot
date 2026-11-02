package task

import (
	"autobuildrobot/models"
	"reflect"
	"testing"
)

func TestMergeTaskAndPermissionParametersMatch(t *testing.T) {
	for _, raw := range []string{"开发分支合并到策划分支", "开发分支合并策划分支", "开发分支,策划分支", "开发分支，策划分支", "开发分支合并到策划分支,mf,合并日志", "开发分支,策划分支,mf,合并日志"} {
		p := analysisOneCommand("project", "分支合并："+raw)
		if p == nil {
			t.Fatalf("failed parsing %q", raw)
		}
		err, permissionParams := models.AnalysisParam(p.RawParams, models.CommandType_SvnMerge)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.CmdParams) < 2 || p.CmdParams[0] != "开发分支" || p.CmdParams[1] != "策划分支" || !reflect.DeepEqual(p.CmdParams, permissionParams) {
			t.Fatalf("inconsistent merge parameters for %q: task=%q permission=%q", raw, p.CmdParams, permissionParams)
		}
	}
}
