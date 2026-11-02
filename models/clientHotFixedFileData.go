package models

import (
	log "autobuildrobot/mylog"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
)

const CLIENTFILESNAME = "files"
const OriginalFILESNAME = "originalfiles"
const EncryptResFolder = "GameRes"
const OriginalResFolder = "StreamingRes"

var ClientHotFixedFileDataTempMap map[string]*UpdateDataOfProject //客户端更新数据缓存(直接用，但是用时要加锁) key:分支名称拼语言  value:热更数据缓存
var ClientHotFixedDataLock sync.RWMutex

const (
	BuildResType_Lua   = 0 //只打lua
	BuildResType_Total = 1 //打整个资源
)

// 项目更新数据
type UpdateDataOfProject struct {
	HotFiexedDatas     []*ClientHotFixedFileData //更新数据
	PreHotRes          []string                  //需要预热的资源
	UpdateSuccessFiles map[string]bool           //上传成功的文件，临时缓存
}

// 文件信息
type ClientHotFixedFileData struct {
	Name string //文件名
	MD5  string //文件MD5
	Size string //文件大小
}

func init() {
	ClientHotFixedFileDataTempMap = make(map[string]*UpdateDataOfProject)
}

// 获取热更地址
func GetHotFixedResPath(projectPath, zzzTimeStamp, platform, lan string) string {
	if zzzTimeStamp == "" {
		if lan == "" {
			//不是多语言，则直接用StreamingAssets资源
			return path.Join(projectPath, "Assets/StreamingAssets")
		} else {
			if platform == "" {
				//单平台用EncryptResFolder资源（出包资源，兼容老的打包代码）
				return path.Join(projectPath, EncryptResFolder, platform, lan)
			} else {
				//多平台，则用StreamingRes下资源
				return path.Join(projectPath, OriginalResFolder, platform, lan)
			}

		}
	} else {
		return path.Join(projectPath, EncryptResFolder, platform, lan)
	}
}

// 获取缓存资源key
func GetHotFixedMapKey(svnProjectName, platform, lan string) string {
	return fmt.Sprintf("%s|%s|%s", svnProjectName, platform, lan)
}

// 获取需要更新的文件数据
func GetNeedUpdateDatas(newFiles, oldFiles string) (err error, needUpdateHotfixedFilesDataMap map[string]*ClientHotFixedFileData) {
	newHotfixedFilesDataMap := AnslysisClientHotFixedFile(newFiles)
	if nil == newHotfixedFilesDataMap {
		log.Error("解析新的files异常：" + newFiles)
		return errors.New("解析新的files异常"), nil
	}
	oldHotfixedFilesDataMap := AnslysisClientHotFixedFile(oldFiles)
	if nil == oldHotfixedFilesDataMap {
		log.Error("解析旧的files异常：" + oldFiles)
		return errors.New("解析旧的files异常"), nil
	}

	//遍历file1文件列表，找到MD5不一致的，或者是file2不存在的文件
	needUpdateHotfixedFilesDataMap = make(map[string]*ClientHotFixedFileData)
	for fileName, newFileData := range newHotfixedFilesDataMap {
		if oldFileData, ok := oldHotfixedFilesDataMap[fileName]; ok {
			//存在的，但是MD5不一样的要上传
			if strings.Compare(oldFileData.MD5, newFileData.MD5) != 0 {
				needUpdateHotfixedFilesDataMap[fileName] = newFileData
				log.Info(fmt.Sprintf("need updata ,filename:%s,cdnmd5:%s,localmd5:%s", fileName, oldFileData.MD5, newFileData.MD5))
			}
		} else {
			//不存在的要上传
			needUpdateHotfixedFilesDataMap[fileName] = newFileData
			log.Info(fmt.Sprintf("need updata ,not exist file,filename:%s", fileName))
		}
	}
	return
}

// 通过files.txt转成该文件的文件数据
func AnslysisClientHotFixedFile(fileString string) map[string]*ClientHotFixedFileData {
	content := strings.Replace(fileString, "\r\n", "\n", -1)
	fileList := strings.Split(content, "\n")
	fileMap := make(map[string]*ClientHotFixedFileData)

	//解析files.txt
	for i := 0; i < len(fileList); i++ {
		oneFileDataStr := fileList[i]
		if strings.Compare(oneFileDataStr, "") == 0 {
			continue
		}
		arrTemp := strings.Split(oneFileDataStr, "|")
		if len(arrTemp) < 3 {
			return nil
		}
		fileMap[arrTemp[0]] = &ClientHotFixedFileData{Name: arrTemp[0], MD5: arrTemp[1], Size: arrTemp[2]}
	}

	return fileMap
}
