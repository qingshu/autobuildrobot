package cdn

const (
	CdnType_ALiYun    = 0
	CdnType_HuaWeiYun = 1
	CdnType_Aws       = 2
	CdnType_TXYun     = 3
	CdnType_UCloud    = 4
	CdnType_Max
)

type RefreshResult func(msg string)

type IcdnClient interface {
	IsExistFile(remoteFilePath string) (bool, error)
	UploadFile(localFilePath, remoteFilePath string) error
	DownFile(remoteFilePath string) (err error, data []byte)
	DownFile2Local(remoteFilePath, downloadedFileName string) error
	DeleteFile(remoteFilePath string) error
	CopyFile(resPath, targetPath string) error
	Refresh(cdnId string, resultFunc RefreshResult, remoteFilePathList []string) error //预热
}
