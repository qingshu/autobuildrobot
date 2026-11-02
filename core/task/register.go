package task

import "autobuildrobot/models"

func init() {
	//指令执行函数
	buildFuncs[models.CommandType_Help] = getHelpTask
	buildFuncs[models.CommandType_StopBuild] = getStopTask
	registerOperation(models.CommandType_CheckOutSvnProject, CheckOutSvnProject)
	registerOperation(models.CommandType_PrintHotfixResList, PrintHotfixResList)
	registerOperation(models.CommandType_UploadHotfixRes2Test, UploadHotfixRes2Test)
	registerOperation(models.CommandType_UploadHotfixRes2Release, UploadHotfixRes2Release)
	registerOperation(models.CommandType_ListSvnLog, ListAllSvnLog)
	registerOperation(models.CommandType_BackupHotfixRes, BackupHotfixRes)
	buildFuncs[models.CommandType_CloseRobot] = getCloseRobotTask
	buildFuncs[models.CommandType_BackupSvn] = getBackupSvnTask
	buildFuncs[models.CommandType_DelHotfixRes] = getDelHotfixResTask
	for _, command := range []int{models.CommandType_AutoBuildClient, models.CommandType_SvnMerge, models.CommandType_UpdateAndRestartSvr,
		models.CommandType_BuildPbMsg, models.CommandType_CloseSvr, models.CommandType_UpdateTable} {
		buildFuncs[command] = getShellTask
	}
	for _, command := range []int{models.CommandType_ProjectConfig, models.CommandType_SvnProjectConfig, models.CommandType_CdnConfig,
		models.CommandType_SvrProgressConfig, models.CommandType_SvrMachineConfig, models.CommandType_User, models.CommandType_UserGroup, models.CommandType_TemplateCmd} {
		buildFuncs[command] = getConfigTask
	}
}
