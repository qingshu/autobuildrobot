package cdn

import (
	log "autobuildrobot/mylog"
	"bytes"
	"errors"
	ufsdk "github.com/ufilesdk-dev/ufile-gosdk"
	"os"
	"time"
)

//参考文档：https://github.com/ufilesdk-dev/ufile-gosdk

type UCloud struct {
	config *ufsdk.Config
}

func NewUCloud(endpointOfBucket, bucketName, accessKeyID, accessKeySecret string) (error, *UCloud) {
	config := new(ufsdk.Config)
	config.FileHost = endpointOfBucket
	config.PublicKey = accessKeyID
	config.PrivateKey = accessKeySecret
	config.BucketName = bucketName
	uCloud := new(UCloud)
	uCloud.config = config
	return nil, uCloud
}

func (this *UCloud) IsExistFile(remoteFilePath string) (bool, error) {
	if nil == this.config {
		return false, errors.New("IsExistFile,ucloud config not exist,please check!")
	}
	req, err := ufsdk.NewFileRequest(this.config, nil)
	if err != nil {
		panic(err.Error())
	}
	err = req.HeadFile(remoteFilePath)
	return nil == err, nil
}

// 上传文件
func (this *UCloud) UploadFile(localFilePath, remoteFilePath string) error {
	if nil == this.config {
		return errors.New("UploadFile,ucloud config not exist,please check!")
	}
	req, err := ufsdk.NewFileRequest(this.config, nil)
	if err != nil {
		panic(err.Error())
	}

	err = req.PutFile(localFilePath, remoteFilePath, "")
	return err
}

// 下载文件
func (this *UCloud) DownFile(remoteFilePath string) (err error, data []byte) {
	if nil == this.config {
		return errors.New("DownFile,ucloud config not exist,please check!"), nil
	}
	req, err := ufsdk.NewFileRequest(this.config, nil)
	if err != nil {
		panic(err.Error())
	}
	write := new(bytes.Buffer)
	err = req.DownloadFile(write, remoteFilePath)
	return err, write.Bytes()
}

func (this *UCloud) DownFile2Local(remoteFilePath, downloadedFileName string) error {
	if nil == this.config {
		return errors.New("DownFile2Local,ucloud config not exist,please check!")
	}
	req, err := ufsdk.NewFileRequest(this.config, nil)
	if err != nil {
		panic(err.Error())
	}

	// 下载文件
	reqUrl := req.GetPrivateURL(remoteFilePath, 10*time.Second)
	err = req.Download(reqUrl)
	if err != nil {
		log.Error(string(req.DumpResponse(true)))
		return err
	}

	// 保存到本地
	f, err := os.OpenFile(downloadedFileName, os.O_CREATE|os.O_WRONLY, 0755)
	defer f.Close()
	if err != nil {
		log.Error("创建文件失败，错误信息为：", err.Error())
		return err
	}
	_, err = f.WriteString(string(req.LastResponseBody))
	if err != nil {
		log.Error("下载数据保存到本地文件失败：", err.Error())
	} else {
		log.Info("下载数据保存到本地文件", downloadedFileName, "成功")
	}
	return err
}

func (this *UCloud) DeleteFile(remoteFilePath string) error {
	if nil == this.config {
		return errors.New("DeleteFile,ucloud config not exist,please check!")
	}

	req, err := ufsdk.NewFileRequest(this.config, nil)
	if err != nil {
		panic(err.Error())
	}
	return req.DeleteFile(remoteFilePath)
}

func (this *UCloud) CopyFile(resPath, targetPath string) error {
	if nil == this.config {
		return errors.New("CopyFile,ucloud config not exist,please check!")
	}

	req, err := ufsdk.NewFileRequest(this.config, nil)
	if err != nil {
		panic(err.Error())
	}
	return req.Copy(targetPath, this.config.BucketName, resPath)
}

// 预热
func (this *UCloud) Refresh(cdnId string, resultFunc RefreshResult, remoteFilePathList []string) error {
	return errors.New("未实现预热功能")
}
