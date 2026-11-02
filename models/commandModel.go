package models

import (
	"errors"

	"strconv"
	"strings"
)

// 指令类型
const (
	CommandType_Help                    = iota //帮助
	CommandType_ProjectConfig                  //项目
	CommandType_SvnProjectConfig               //svn工程
	CommandType_CdnConfig                      //cdn
	CommandType_CheckOutSvnProject             //检出svn工程
	CommandType_SvnMerge                       //分支合并
	CommandType_UpdateTable                    //更新表格
	CommandType_AutoBuildClient                //客户端自动构建
	CommandType_PrintHotfixResList             //输出热更资源列表
	CommandType_UploadHotfixRes2Test           //上传测试热更资源
	CommandType_UploadHotfixRes2Release        //上传正式热更资源
	CommandType_BackupHotfixRes                //备份热更资源
	CommandType_DelHotfixRes                   //删除热更资源
	CommandType_SvrProgressConfig              //游戏服务进程
	CommandType_SvrMachineConfig               //游戏服主机
	CommandType_UpdateAndRestartSvr            //更新并重启服务器
	CommandType_CloseSvr                       //关闭服务器
	CommandType_BuildPbMsg                     //构建消息码
	CommandType_ListSvnLog                     //列出svn日志
	CommandType_UserGroup                      //用户组
	CommandType_User                           //用户
	CommandType_CloseRobot                     //关闭机器人
	CommandType_TemplateCmd                    //模板指令
	CommandType_BackupSvn                      //备份svn
	CommandType_StopBuild                      //停止构建
	CommandType_Max
)

var commandName [CommandType_Max]string //指令名字

func init() {

	//初始化指令名字
	commandName[CommandType_Help] = "帮助"
	commandName[CommandType_ProjectConfig] = "项目"
	commandName[CommandType_SvnProjectConfig] = "svn工程"
	commandName[CommandType_CdnConfig] = "cdn"
	commandName[CommandType_CheckOutSvnProject] = "检出svn"
	commandName[CommandType_SvnMerge] = "分支合并"
	commandName[CommandType_AutoBuildClient] = "客户端构建"
	commandName[CommandType_PrintHotfixResList] = "输出热更资源"
	commandName[CommandType_UploadHotfixRes2Test] = "上传热更资源到测试"
	commandName[CommandType_UploadHotfixRes2Release] = "上传热更资源到正式"
	commandName[CommandType_BackupHotfixRes] = "备份热更资源"
	commandName[CommandType_DelHotfixRes] = "删除热更资源"
	commandName[CommandType_SvrProgressConfig] = "游戏服务"
	commandName[CommandType_SvrMachineConfig] = "服务器主机"
	commandName[CommandType_UpdateAndRestartSvr] = "更新服务器"
	commandName[CommandType_BuildPbMsg] = "更新消息码"
	commandName[CommandType_CloseSvr] = "关闭服务器"
	commandName[CommandType_ListSvnLog] = "输出svn日志"
	commandName[CommandType_UserGroup] = "用户组"
	commandName[CommandType_User] = "用户"
	commandName[CommandType_UpdateTable] = "更新表格"
	commandName[CommandType_CloseRobot] = "关闭自动构建机器人"
	commandName[CommandType_TemplateCmd] = "模板指令"
	commandName[CommandType_BackupSvn] = "备份svn"
	commandName[CommandType_StopBuild] = "停止构建"

}

// 获取指令名字
func GetCommandNameByType(commandType int) string {
	if commandType < CommandType_Help || commandType >= CommandType_Max {
		return "不存在指令类型：" + strconv.Itoa(commandType)
	}
	return commandName[commandType]
}

// 判断是否需要项目权限
func JudgeIsNeedProjectPermission(commandType int) bool {
	if commandType == CommandType_SvnMerge || commandType == CommandType_UpdateTable ||
		commandType == CommandType_AutoBuildClient || commandType == CommandType_PrintHotfixResList ||
		commandType == CommandType_UploadHotfixRes2Test || commandType == CommandType_UploadHotfixRes2Release ||
		commandType == CommandType_BackupHotfixRes || commandType == CommandType_UpdateAndRestartSvr ||
		commandType == CommandType_DelHotfixRes || commandType == CommandType_CheckOutSvnProject {
		return true
	}
	return false
}

// 判断是否只有超级用户才有得权限
func JudgeIsOnlySuperUserCmd(commandType int) bool {
	if commandType == CommandType_BackupSvn ||
		commandType == CommandType_CloseRobot {
		return true
	}
	return false
}

// 解析参数
func AnalysisParam(requestParam string, commandType int) (err error, params []string) {
	//不需要参数
	if commandType == CommandType_Help ||
		commandType == CommandType_CloseRobot ||
		commandType == CommandType_BackupSvn {
		return
	}

	//参数不足
	if requestParam == "" {
		if commandType == CommandType_ProjectConfig || commandType == CommandType_SvnProjectConfig ||
			commandType == CommandType_CdnConfig || commandType == CommandType_SvrProgressConfig ||
			commandType == CommandType_SvrMachineConfig || commandType == CommandType_UpdateAndRestartSvr ||
			commandType == CommandType_CloseSvr || commandType == CommandType_BuildPbMsg ||
			commandType == CommandType_User || commandType == CommandType_UserGroup {
			//可以不用参数
			return nil, nil
		}
		return errors.New("参数不足，请输入帮助获取提示！！！"), nil
	}

	if commandType == CommandType_SvnMerge {
		//特殊处理项目合并，需要两个参数
		requestParam = strings.ReplaceAll(requestParam, "，", ",")
		requestParam = strings.ReplaceAll(requestParam, "；", ";")
		params := strings.Split(requestParam, ",")
		for _, flag := range mergeFlags {
			// Only inspect the branch expression, not the optional commit message.
			branches := strings.SplitN(params[0], flag, 2)
			if len(branches) >= 2 && branches[0] != "" && branches[1] != "" {
				return nil, append(branches, params[1:]...)
			}
		}
		// Also accept the source,target syntax used by task commands.
		if len(params) >= 2 && strings.TrimSpace(params[0]) != "" && strings.TrimSpace(params[1]) != "" {
			return nil, params
		}
		return errors.New("获取合并分支失败！"), nil
	} else {
		//中文逗号和分号全部替换成英文逗号和分号，根据逗号分割参数
		requestParam = strings.ReplaceAll(requestParam, "，", ",")
		requestParam = strings.ReplaceAll(requestParam, "；", ";")
		requestParams := strings.Split(requestParam, ",")
		return nil, requestParams
	}
}

// 判断参数是否获取所有配置信息
func JudgeIsSearchAllParam(commandParams string) bool {
	return commandParams == "" || commandParams == "全部" || commandParams == "所有" || strings.ToLower(commandParams) == "all"
}
