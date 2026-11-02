package models

import (
	log "autobuildrobot/mylog"
	"autobuildrobot/tool"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// 项目配置
type ProjectModel struct {
	Manager                   string   `json:"Manager"`                   //管理员
	UnopenCommandNameList     []string `json:"UnopenCommandNameList"`     //未开放的指令名字列表
	ClientEnginePath          string   `json:"ClientEnginePath"`          //客户端引擎地址
	TempBanNormalUserCommands []string `json:"TempBanNormalUserCommands"` //临时禁止普通成员指令数组（如发版本时）
	ExternalRelativeAddrList  []string `json:"ExternalRelativeAddrList"`  //工程外链相对地址
	ReadExcelToolFileName     string   `json:"ReadExcelToolFileName"`     //读表工具文件名
	UpdateProtoToolFileName   string   `json:"UpdateProtoToolFileName"`   //更新消息码工具文件名
	UpdateSvrToolFileName     string   `json:"UpdateSvrToolFileName"`     //更新服务器工具名
}

var projectFileName = "projectData.gob"
var projectsMap map[string]*ProjectModel
var projectDataLock sync.Mutex

func init() {
	projectsMap = make(map[string]*ProjectModel)
	tool.ReadGobFile(projectFileName, &projectsMap)
}

// 有就更新，没有则添加
func UpdateProject(projectName, projectConfig string) (result string, err error) {
	projectDataLock.Lock()
	defer projectDataLock.Unlock()

	//解析数据
	projectModel := new(ProjectModel)
	projectModel.TempBanNormalUserCommands = make([]string, 0)
	projectModel.UnopenCommandNameList = make([]string, 0)
	err = tool.UnmarshJson([]byte(projectConfig), &projectModel)
	if nil != err {
		return
	}

	//更新或新增
	if _, ok := projectsMap[projectName]; ok {
		//存在则更新
		if "" != projectModel.Manager {
			projectsMap[projectName].Manager = projectModel.Manager
		}
		if "" != projectModel.ClientEnginePath {
			projectsMap[projectName].ClientEnginePath = projectModel.ClientEnginePath
		}
		if "" != projectModel.ReadExcelToolFileName {
			projectsMap[projectName].ReadExcelToolFileName = projectModel.ReadExcelToolFileName
		}
		if "" != projectModel.UpdateSvrToolFileName {
			projectsMap[projectName].UpdateSvrToolFileName = projectModel.UpdateSvrToolFileName
		}
		if "" != projectModel.UpdateProtoToolFileName {
			projectsMap[projectName].UpdateProtoToolFileName = projectModel.UpdateProtoToolFileName
		}
		if nil != projectModel.TempBanNormalUserCommands && len(projectModel.TempBanNormalUserCommands) > 0 {
			projectsMap[projectName].TempBanNormalUserCommands = projectModel.TempBanNormalUserCommands
		}
		if nil != projectModel.UnopenCommandNameList && len(projectModel.UnopenCommandNameList) > 0 {
			projectsMap[projectName].UnopenCommandNameList = projectModel.UnopenCommandNameList
		}
	} else {
		projectsMap[projectName] = projectModel
	}

	//编码并存储
	tool.SaveGobFile(projectFileName, projectsMap)
	result = "更新项目配置成功"
	return
}

// 获取项目配置数据
func GetProjectData(projectName string) (result string) {
	projectDataLock.Lock()
	defer projectDataLock.Unlock()
	if _project, ok := projectsMap[projectName]; ok {
		return tool.MarshalJson(_project)
	}

	//如果不存在项目，则输出默认值
	return "项目不存在，请添加：" + GetProjectConfigHelp()
}

// 获取项目配置帮助信息
func GetProjectConfigHelp() (result string) {
	project := new(ProjectModel)
	project.Manager = "项目管理员名字"
	project.ClientEnginePath = "项目客户端引擎（如unity）路径"
	project.ReadExcelToolFileName = "项目打表工具执行文件，如ReadExcel.sh"
	project.UpdateSvrToolFileName = "项目更新服务器执行文件"
	project.UpdateProtoToolFileName = "项目更新消息码执行文件"
	project.UnopenCommandNameList = make([]string, 0)
	project.TempBanNormalUserCommands = make([]string, 0)
	return fmt.Sprintf(`例：【%s：%s】\n其中，UnopenCommandNameList是不开放的指令数组,TempBanNormalUserCommands是临时要禁用的指令名称或者项目名称
AutoBuildMethodList对应客户端AutoBuild.cs定义的构建方法数组,第一个一定是构建lua包，第二个一定是打整个资源包，之后打apk包自定义\n如多个配置用分号分割`, commandName[CommandType_ProjectConfig], tool.MarshalJson(project))
}

// 获取项目管理员
func GetProjectManager(projectName string) string {
	projectDataLock.Lock()
	defer projectDataLock.Unlock()
	if project, ok := projectsMap[projectName]; ok {
		return project.Manager
	}
	log.Error("项目管理员不存在,请配置！！！")
	return ""
}

// 获取项目客户端引擎（Unity）路径
func GetProjectClientEnginePath(projectName string) (error, string) {
	projectDataLock.Lock()
	defer projectDataLock.Unlock()
	if project, ok := projectsMap[projectName]; ok {
		return nil, project.ClientEnginePath
	}
	return errors.New("项目客户端引擎路径失败，项目不存在,请添加！！！"), ""
}

// 获取执行工具文件名
func GetExecToolFileName(projectName string, commandParams []string, commandType int) string {
	if commandType == CommandType_UpdateAndRestartSvr && len(commandParams) > 1 {
		//先在游戏服务配置中获取构建工具，如果有配置则用这里的
		svrToolName := GetSvrToolName(projectName, commandParams[1])
		if svrToolName != "" {
			return svrToolName
		}
	}

	//在获取项目中配置的工具名
	projectDataLock.Lock()
	defer projectDataLock.Unlock()
	if project, ok := projectsMap[projectName]; ok {
		//目前基本就打表、打消息码及更新服务器差异比较大，需要不同执行文件
		if commandType == CommandType_UpdateTable {
			return project.ReadExcelToolFileName
		} else if commandType == CommandType_UpdateAndRestartSvr {
			return project.UpdateSvrToolFileName
		} else if commandType == CommandType_BuildPbMsg {
			return project.UpdateProtoToolFileName
		} else {
			return ""
		}
	}
	log.Error("获取指令执行文件，项目不存在！！")
	return ""
}

// 判断是否为管理员
func JudgeIsManager(projectName, userName string) bool {
	projectDataLock.Lock()
	defer projectDataLock.Unlock()
	if project, ok := projectsMap[projectName]; ok {
		for _, name := range strings.Split(project.Manager, "|") {
			if userName != "" && strings.TrimSpace(name) == userName {
				return true
			}
		}
	}
	return false
}

// 获取不开放的指令
func GetUnopenCommandList(projectName string) (commandList []string) {
	projectDataLock.Lock()
	defer projectDataLock.Unlock()
	commandList = make([]string, 0)
	if project, ok := projectsMap[projectName]; ok {
		return project.UnopenCommandNameList
	}
	return commandList
}

// 判断指令是否被禁止
func JudgeCommandIsBan(projectName, userName, commandName, svnProject string) bool {
	projectDataLock.Lock()
	defer projectDataLock.Unlock()
	if project, ok := projectsMap[projectName]; ok {
		if strings.Contains(project.Manager, userName) {
			return false
		}
		for _, v := range project.TempBanNormalUserCommands {
			if v == "" {
				continue
			}
			if v == commandName || v == svnProject {
				return true
			}
		}
	}
	return false
}
