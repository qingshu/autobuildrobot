package models

import (
	log "autobuildrobot/mylog"
	"autobuildrobot/tool"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// 游戏服务器进程配置
type GameSvrProgressModel struct {
	SvrProgressName       string `json:"SvrProgressName"`       //服务进程名称
	UpdateSvrToolFileName string `json:"UpdateSvrToolFileName"` //构建工具名称
	SvrRootFolder         string `json:"SvrRootFolder"`         //服务根目录
	SvrProgressDirName    string `json:"SvrProgressDirName"`    //服务进程文件夹名
	ZipFileNameWithoutExt string `json:"ZipFileNameWithoutExt"` //服务进程压缩文件名字
	ZipFileList           string `json:"ZipFileList"`           //要压缩的文件list,用竖线分割
	ZipDirList            string `json:"ZipDirList"`            //要压缩的文件夹列表，用竖线分割
	IsDoNotCompile        bool   `json:"IsDoNotCompile"`        //是否不用编译
}

var lastSvrProgressConfigFileName string                  //上一次的服务进程配置数据文件名（基本一个项目一个文件）
var svrProgressConfigMap map[string]*GameSvrProgressModel //服务进程配置字典，key 服务进程名 value:服务进程配置
var svrProgressDataLock sync.Mutex

// 有就更新，没有则添加
func UpdateSvrProgressData(projectName, svrConfig string) (result string) {
	svrProgressDataLock.Lock()
	defer svrProgressDataLock.Unlock()

	//先获取数据
	var fileName string
	fileName, svrProgressConfigMap = getProjectSvrProgressData(projectName)

	//再更新数据
	svrArr := strings.Split(svrConfig, ";")
	for _, svr := range svrArr {
		if svr == "" {
			continue
		}
		svrModel := new(GameSvrProgressModel)
		tool.UnmarshJson([]byte(svr), &svrModel)
		if svrModel.SvrProgressName == "" {
			errMsg := "svr配置工程名不能为空：" + svr
			log.Error(errMsg)
			result += (errMsg + "\n")
			continue
		}

		//删除配置
		if strings.Contains(svrModel.SvrProgressName, "-") {
			//负号作为删除标记吧
			delBranch := strings.ReplaceAll(svrModel.SvrProgressName, "-", "")
			delete(svrProgressConfigMap, delBranch)
			continue
		}

		//增加或修改
		if _svrModel, ok := svrProgressConfigMap[svrModel.SvrProgressName]; ok {
			//已存在，如果数据为空则用老数据
			if svrModel.SvrProgressDirName == "" {
				svrModel.SvrProgressDirName = _svrModel.SvrProgressDirName
			}
			if svrModel.ZipFileList == "" {
				svrModel.ZipFileList = _svrModel.ZipFileList
			}
			if svrModel.ZipDirList == "" {
				svrModel.ZipDirList = _svrModel.ZipDirList
			}
			if svrModel.ZipFileNameWithoutExt == "" {
				svrModel.ZipFileNameWithoutExt = _svrModel.ZipFileNameWithoutExt
			}
			if svrModel.SvrRootFolder == "" {
				svrModel.SvrRootFolder = _svrModel.SvrRootFolder
			}
			if svrModel.UpdateSvrToolFileName == "" {
				svrModel.UpdateSvrToolFileName = _svrModel.UpdateSvrToolFileName
			}
		}
		svrProgressConfigMap[svrModel.SvrProgressName] = svrModel
	}

	//编码并存储
	tool.SaveGobFile(fileName, svrProgressConfigMap)
	if result != "" {
		return
	}
	return "更新svr配置成功"
}

// 获取一个项目所有服务进程配置信息
func QueryProgressDataOfOneProject(projectName, searchValue string) (result string) {
	svrProgressDataLock.Lock()
	defer svrProgressDataLock.Unlock()
	_, svrProgressConfigMap = getProjectSvrProgressData(projectName)
	if len(svrProgressConfigMap) <= 0 {
		return "当前没有服务器进程配置信息，请配置：\n" + GetSvrProgressConfigHelp()
	}

	tpl := GameSvrProgressModel{}
	for _, v := range svrProgressConfigMap {
		if !JudgeIsSearchAllParam(searchValue) && !strings.Contains(v.SvrProgressName, searchValue) {
			continue
		}
		tpl.SvrProgressName = v.SvrProgressName
		tpl.SvrProgressDirName = v.SvrProgressDirName
		tpl.IsDoNotCompile = v.IsDoNotCompile
		tpl.ZipFileList = v.ZipFileList
		tpl.ZipDirList = v.ZipDirList
		tpl.ZipFileNameWithoutExt = v.ZipFileNameWithoutExt
		tpl.SvrRootFolder = v.SvrRootFolder
		tpl.UpdateSvrToolFileName = v.UpdateSvrToolFileName
		result += fmt.Sprintln(tool.MarshalJson(tpl) + "\n")
	}
	if result == "" {
		return "当前没有符合条件的服务器进程配置信息，请配置：\n" + GetSvrProgressConfigHelp()
	} else {
		return "\n***********************以下是服务器进程配置数据***********************\n" + result
	}
}

// 获取服务进程配置帮助提示
func GetSvrProgressConfigHelp() string {
	tpl := GameSvrProgressModel{
		SvrProgressName:       "游戏服务进程名",
		UpdateSvrToolFileName: "构建执行文件名，如果没有配置则默认用项目配置中的文件",
		SvrRootFolder:         "游戏服务根文件夹（有些svn根目录下不是直接游戏服务，可能有其他文件夹，此时就要指明游戏根目录文件夹）",
		SvrProgressDirName:    "服务进程文件夹名",
		ZipFileNameWithoutExt: "编译后压缩上传的不带后缀的压缩文件名",
		ZipDirList:            "要压缩上传的所有文件夹名，多个用竖线分割",
		ZipFileList:           "要压缩上传的所有文件名，多个用竖线分割，如果目标平台多个，那就编译出的可执行文件都配上吧，如果没有构建就不会打包进去",
	}
	return fmt.Sprintf("一套完整服务器代码可能包括游戏服、跨服、后台等，游戏服务进程配置就是在更新服务器时告诉构建其服务对应哪个文件夹代码，哪些文件要打包\n配置例子：\n【%s：%s】 \n如多个配置用分号分割",
		commandName[CommandType_SvrProgressConfig], tool.MarshalJson(tpl))
}

// 获取服务进程配置数据
func GetSvrProgressData(projectName, svrProgressName string) (err error, rootFolder, dirName, zipFileNameWithoutExt, zipFileList, zipDirList string, isDoNotCompile bool) {
	if svrProgressName == "" {
		err = errors.New("游戏服务配置名不能为空！")
		return
	}

	svrProgressDataLock.Lock()
	defer svrProgressDataLock.Unlock()
	_, svrProgressConfigMap = getProjectSvrProgressData(projectName)
	if _svrModel, ok := svrProgressConfigMap[svrProgressName]; ok {
		return nil, _svrModel.SvrRootFolder, _svrModel.SvrProgressDirName, _svrModel.ZipFileNameWithoutExt, _svrModel.ZipFileList, _svrModel.ZipDirList, _svrModel.IsDoNotCompile
	} else {
		err = errors.New(svrProgressName + "svrProgress配置不存在，请添加！")
		return
	}
}

// 获取服务进程配置数据
func GetSvrToolName(projectName, svrProgressName string) (svrToolName string) {
	if svrProgressName == "" {
		return
	}

	svrProgressDataLock.Lock()
	defer svrProgressDataLock.Unlock()
	_, svrProgressConfigMap = getProjectSvrProgressData(projectName)
	if _svrModel, ok := svrProgressConfigMap[svrProgressName]; ok {
		return _svrModel.UpdateSvrToolFileName
	} else {
		return
	}
}

// 根据项目名获取svr文件名和数据
func getProjectSvrProgressData(projectName string) (string, map[string]*GameSvrProgressModel) {
	svrDataFileName := "svrProgress.gob"
	fileName := tool.CalcMd5(projectName) + svrDataFileName
	if fileName == lastSvrProgressConfigFileName {
		return fileName, svrProgressConfigMap
	}
	svrProgressConfigMap = make(map[string]*GameSvrProgressModel)
	tool.ReadGobFile(fileName, &svrProgressConfigMap)
	lastSvrProgressConfigFileName = fileName
	return fileName, svrProgressConfigMap
}
