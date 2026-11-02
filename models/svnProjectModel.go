package models

import (
	log "autobuildrobot/mylog"
	"autobuildrobot/tool"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// svn工程，区别于ProjectModel，一个ProjectModel对应多个SvnProjectModel
type SvnProjectModel struct {
	ProjectName        string `json:"ProjectName"`        //工程名称
	ProjectPath        string `json:"ProjectPath"`        //工程地址
	SvnUrl             string `json:"SvnUrl"`             //svn地址
	SvnExternalKeyword string `json:"SvnExternalKeyword"` //外链关键字
	LastGetSvnLogTime  int64  `json:"-"`                  //上次获取svn日志时间
}

var lastProjectFileName string                //上一个项目的svn工程数据文件名（基本一个项目一个svn工程数据文件）
var svnProjectMap map[string]*SvnProjectModel //项目分支配置字典，key 分支名 value:项目分支
var mergeFlags = []string{"合并到", "合并"}        //项目合并标识，按顺序分割获取两个分支
var svnProjectDataLock sync.Mutex

// 有就更新，没有则添加
func UpdateSvnProject(projectName, svnProjectConfig string) (result string) {
	svnProjectDataLock.Lock()
	defer svnProjectDataLock.Unlock()

	//先获取数据
	var fileName string
	fileName, svnProjectMap = getSvnProjectsDataByProjectName(projectName)

	//再更新数据
	avnProjectArr := strings.Split(svnProjectConfig, ";")
	for _, svnProject := range avnProjectArr {
		if svnProject == "" {
			continue
		}
		svnProjectModel := new(SvnProjectModel)
		tool.UnmarshJson([]byte(svnProject), &svnProjectModel)
		if svnProjectModel.ProjectName == "" {
			log.Error("svn工程名不能为空：", svnProject)
			continue
		}
		if strings.Contains(svnProjectModel.ProjectName, "-") {
			//负号作为删除标记吧
			delSvnProject := strings.ReplaceAll(svnProjectModel.ProjectName, "-", "")
			if strings.Contains(delSvnProject, "*") {
				delSvnProject = strings.ReplaceAll(delSvnProject, "*", "")
				delKeys := make([]string, 0)
				for k, _ := range svnProjectMap {
					if strings.Contains(k, delSvnProject) {
						delKeys = append(delKeys, k)
					}
				}
				for _, k := range delKeys {
					delete(svnProjectMap, k)
				}
			} else {
				delete(svnProjectMap, delSvnProject)
			}
			continue
		}
		if oldSvnProject, ok := svnProjectMap[svnProjectModel.ProjectName]; ok {
			//存在则LastGetSvnLogTime不能被修改
			svnProjectModel.LastGetSvnLogTime = oldSvnProject.LastGetSvnLogTime

			//如果没有数据则用老的赋值
			if svnProjectModel.ProjectPath == "" {
				svnProjectModel.ProjectPath = oldSvnProject.ProjectPath
			}
			if svnProjectModel.SvnUrl == "" {
				svnProjectModel.SvnUrl = oldSvnProject.SvnUrl
			}
		}
		svnProjectMap[svnProjectModel.ProjectName] = svnProjectModel
	}

	//编码并存储
	tool.SaveGobFile(fileName, svnProjectMap)
	result = "更新svn工程配置成功"
	return
}

// 获取一个项目所有分支配置信息
func QuerySvnProjectsDataByProject(projectName, searchValue string) (result string) {
	svnProjectDataLock.Lock()
	defer svnProjectDataLock.Unlock()

	_, svnProjectMap = getSvnProjectsDataByProjectName(projectName)
	if len(svnProjectMap) <= 0 {
		return "当前没有svn工程信息，请配置：\n" + GetSvnProjectConfigHelp()
	}

	for _, v := range svnProjectMap {
		if !JudgeIsSearchAllParam(searchValue) && !strings.Contains(v.ProjectName, searchValue) {
			continue
		}
		result += fmt.Sprintln(tool.MarshalJson(v) + "\n")
	}
	if result == "" {
		return "当前没有符合条件的svn工程配置信息，请配置：\n" + GetSvnProjectConfigHelp()
	} else {
		return "\n***********************以下是svn工程配置数据***********************\n" + result
	}
}

// 获取svn工程配置帮助提示
func GetSvnProjectConfigHelp() string {
	tpl := SvnProjectModel{
		ProjectName:        "svn工程名称",
		ProjectPath:        "工程的绝对路径",
		SvnExternalKeyword: "工程包含的外链关键字，用来构建的时候判断外链有没有修改，目前只简单考虑表格外链情况",
	}
	return fmt.Sprintf("svn工程配置是整个构建的最核心配置，它告诉基本所有操作需要的工程位置和svn地址\n配置例子：\n【%s：%s】 \n其中路径不能有反斜杠,用/，如多个配置用分号分割",
		commandName[CommandType_SvnProjectConfig], tool.MarshalJson(tpl))
}

// 获取合并指令帮助
func GetMergeCommandHelp() string {
	return fmt.Sprintf(`目前前后端都定为5大分支:
1、临时开发分支（跨版本迭代），
2、开发分支（常规迭代开发）
3、策划分支（开发完成一个功能直接合并给策划验收）
4、测试分支（一个迭代所有功能策划验收完成后由策划分支合并到测试分支，测试分支会比开发晚一个迭代）
5、发版分支（测试验收完毕合并到发版分支准备对外）
更详细流程看https://www.kdocs.cn/l/spWN1ZyWsEPr?f=131
例：【%s：开发分支合并到策划分支】，开发分支和策划分支都是指令【%s】的ProjectName\n`,
		commandName[CommandType_SvnMerge], commandName[CommandType_SvnProjectConfig])
}

// 获取客户端构建帮助
func GetClientBuildCommandHelp() string {
	return fmt.Sprintf(`根据参数，执行打lua资源、打整个资源，出白包、以及各个渠道包
例：【%s：外网测试包,BuildLuaCode，0】或【%s：外网测试包,0，0】
参数1是指令【%s】配置的ProjectName
参数2是指令【%s】配置的AutoBuildMethodList方法数组中某个构建方法或其索引
参数3是语言枚举
参数4选填，目前只有固定dev表示是development build，不填则表示默认的release build`,
		commandName[CommandType_AutoBuildClient], commandName[CommandType_AutoBuildClient], commandName[CommandType_SvnProjectConfig], commandName[CommandType_ProjectConfig])
}

// 判断工程是否存在
func JudgeSvnProjectIsExist(projectName, svnProjectName string) bool {
	svnProjectDataLock.Lock()
	defer svnProjectDataLock.Unlock()
	projectModel := getSvnProjectData(projectName, svnProjectName)
	return nil != projectModel
}

// 获取svn地址
func GetSvnProjectInfo(projectName, svnProjectName string) (err error, projectPath, svnUrl, svnExternalKeyword string) {
	svnProjectDataLock.Lock()
	defer svnProjectDataLock.Unlock()
	svnProjectModel := getSvnProjectData(projectName, svnProjectName)
	if nil == svnProjectModel {
		return errors.New(fmt.Sprintf("不存在svn%s工程，请添加", svnProjectName)), "", "", ""
	}
	return nil, svnProjectModel.ProjectPath, svnProjectModel.SvnUrl, svnProjectModel.SvnExternalKeyword
}

// 获取上次获取svn日志时间
func GetSvnLogTime(projectName, svnProjectName string) int64 {
	svnProjectDataLock.Lock()
	defer svnProjectDataLock.Unlock()
	svnProjectModel := getSvnProjectData(projectName, svnProjectName)
	if nil == svnProjectModel {
		log.Error("获取上次获取svn日志时间，不存在工程，请添加")
		return 0
	}
	return svnProjectModel.LastGetSvnLogTime
}

// 保存获取svn日志时间
func SaveSvnLogTime(projectName, svnProjectName string, getLogTime int64) {
	svnProjectDataLock.Lock()
	defer svnProjectDataLock.Unlock()
	var fileName string
	fileName, svnProjectMap = getSvnProjectsDataByProjectName(projectName)
	if _, ok := svnProjectMap[svnProjectName]; ok {
		svnProjectMap[svnProjectName].LastGetSvnLogTime = getLogTime
		tool.SaveGobFile(fileName, svnProjectMap)
	} else {
		log.Error("保存svn获取时间失败，不存在svn工程")
	}
}

// 获取工程配置
func getSvnProjectData(projectName, svnProjectName string) *SvnProjectModel {
	if projectName == "" || svnProjectName == "" {
		return nil
	}
	_, svnProjectMap = getSvnProjectsDataByProjectName(projectName)
	if _svnProjectModel, ok := svnProjectMap[svnProjectName]; ok {
		return _svnProjectModel
	} else {
		log.Error(svnProjectName, "svn工程配置不存在，请添加")
		return nil
	}
}

// 根据webHook获取该项目svn工程数据文件名和数据
func getSvnProjectsDataByProjectName(projectName string) (string, map[string]*SvnProjectModel) {
	svnProjectDataFileName := "svnProject.gob"
	fileName := tool.CalcMd5(projectName) + svnProjectDataFileName
	if fileName == lastProjectFileName {
		return fileName, svnProjectMap
	}
	svnProjectMap = make(map[string]*SvnProjectModel)
	tool.ReadGobFile(fileName, &svnProjectMap)
	lastProjectFileName = fileName
	return fileName, svnProjectMap
}
