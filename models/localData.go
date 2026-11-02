package models

import (
	"autobuildrobot/tool"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

var localDataFileName = "localData.gob" //本地数据文件
var localData *LocalData                //本地数据
var localDataLock sync.Mutex

type LocalData struct {
	BuildVersion map[int]int //打包构建版本号，key:指令枚举 value:对应的构建版本号
}

func init() {
	//获取本地数据
	localData = new(LocalData)
	localData.BuildVersion = make(map[int]int)
	tool.ReadGobFile(localDataFileName, localData)
}

// 获取构建版本号
func GetBuildVersion(commandType int) (buildVersion int, ok bool) {
	localDataLock.Lock()
	defer localDataLock.Unlock()
	if buildVersion, _ok := localData.BuildVersion[commandType]; _ok {
		localData.BuildVersion[commandType]++
		tool.SaveGobFile(localDataFileName, localData)
		return buildVersion, _ok
	}
	return 0, false
}

// 获取所有版本号信息
func GetAllBuildVersionInfo() string {
	localDataLock.Lock()
	defer localDataLock.Unlock()
	result := ""
	for index, v := range localData.BuildVersion {
		result += fmt.Sprintf("%s：%d\n", GetCommandNameByType(index), v)
	}
	return result
}

// 解析并保存构建版本号
func SaveBuildVersion(buildVersionInfo string) (result string) {
	localDataLock.Lock()
	defer localDataLock.Unlock()
	buildVersionArr := strings.Split(buildVersionInfo, ";")
	for _, buildVersion := range buildVersionArr {
		buildVersionInfos := strings.Split(buildVersion, ",")
		if len(buildVersionInfos) < 2 {
			result = "输入信息不合法，打包命令枚举和版本号以逗号分割，如设置打安卓QC包版本号为1则：8,1"
			continue
		}
		packCommandType, _ := strconv.Atoi(buildVersionInfos[0])
		version, _ := strconv.Atoi(buildVersionInfos[1])
		localData.BuildVersion[packCommandType] = version
	}

	//编码并存储
	tool.SaveGobFile(localDataFileName, localData)
	return
}
