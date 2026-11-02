package cdn

import (
	"errors"
	"fmt"
	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/global"
	cdn "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/cdn/v1"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/cdn/v1/model"
	"io/ioutil"
	"strings"
)

//参考文档：https://sdkcenter.developer.huaweicloud.com/zh-cn?product=obs&language=go

type HuaWeiYun struct {
	bucketName      string
	accessKeyID     string
	accessKeySecret string
	endPoint        string
	obsClient       *obs.ObsClient
}

func NewHuaWeiYun(endpointOfBucket, bucketName, accessKeyID, accessKeySecret string) (error, *HuaWeiYun) {
	client, err := obs.New(accessKeyID, accessKeySecret, endpointOfBucket)
	if err != nil {
		client.Close()
		return err, nil
	}

	hwy := new(HuaWeiYun)
	hwy.endPoint = endpointOfBucket
	hwy.bucketName = bucketName
	hwy.accessKeyID = accessKeyID
	hwy.accessKeySecret = accessKeySecret
	hwy.obsClient = client
	return nil, hwy
}

func (this *HuaWeiYun) IsExistFile(remoteFilePath string) (bool, error) {
	if nil == this.obsClient {
		return false, errors.New("IsExistFile,huaweiyun obsclient not exist,please check!")
	}
	input := &obs.GetObjectMetadataInput{}
	input.Bucket = this.bucketName
	input.Key = remoteFilePath
	_, err := this.obsClient.GetObjectMetadata(input)
	if err == nil {
		return true, nil
	}

	//判断不存在
	if strings.Contains(err.Error(), "Status=404 Not Found") {
		return false, nil
	}

	//其他异常
	return false, err
}

// 上传文件
func (this *HuaWeiYun) UploadFile(localFilePath, remoteFilePath string) error {
	if nil == this.obsClient {
		return errors.New("UploadFile,huaweiyun obsclient not exist,please check!")
	}
	input := &obs.PutFileInput{}
	input.Bucket = this.bucketName
	input.Key = remoteFilePath
	input.SourceFile = localFilePath // localfile为待上传的本地文件路径，需要指定到具体的文件名
	_, err := this.obsClient.PutFile(input)
	if err != nil {
		if obsError, ok := err.(obs.ObsError); ok {
			return fmt.Errorf("errorCode:%s,msg:%s: %w", obsError.Code, obsError.Message, err)
		}
	}
	return err
}

// 下载文件
func (this *HuaWeiYun) DownFile(remoteFilePath string) (err error, data []byte) {
	if nil == this.obsClient {
		return errors.New("DownFile,huaweiyun obsclient not exist,please check!"), nil
	}
	input := &obs.GetObjectInput{}
	input.Bucket = this.bucketName
	input.Key = remoteFilePath
	output, err := this.obsClient.GetObject(input)
	defer func() {
		if output == nil {
			return
		}
		output.Body.Close()
	}()
	if nil != err {
		return err, nil
	}
	data, err = ioutil.ReadAll(output.Body)
	if err != nil {
		return err, nil
	}
	return nil, data
}

func (this *HuaWeiYun) DownFile2Local(remoteFilePath, downloadedFileName string) error {
	if nil == this.obsClient {
		return errors.New("DownFile2Local,huaweiyun obsclient not exist,please check!")
	}
	input := &obs.DownloadFileInput{}
	input.Bucket = this.bucketName
	input.Key = remoteFilePath
	input.DownloadFile = downloadedFileName // localfile为下载对象的本地文件全路径
	input.EnableCheckpoint = true           // 开启断点续传模式
	input.PartSize = 9 * 1024 * 1024        // 指定分段大小为9MB
	input.TaskNum = 5                       // 指定分段下载时的最大并发数
	_, err := this.obsClient.DownloadFile(input)
	return err
}

func (this *HuaWeiYun) DeleteFile(remoteFilePath string) error {
	if nil == this.obsClient {
		return errors.New("DeleteFile,huaweiyun obsclient not exist,please check!")
	}
	input := &obs.DeleteObjectInput{}
	input.Bucket = this.bucketName
	input.Key = remoteFilePath
	_, err := this.obsClient.DeleteObject(input)
	return err
}

func (this *HuaWeiYun) CopyFile(resPath, targetPath string) error {
	if nil == this.obsClient {
		return errors.New("CopyFile,huaweiyun obsclient not exist,please check!")
	}
	input := &obs.CopyObjectInput{}
	input.Bucket = this.bucketName
	input.Key = targetPath
	input.CopySourceBucket = this.bucketName
	input.CopySourceKey = resPath
	_, err := this.obsClient.CopyObject(input)
	return err
}

// 预热
func (this *HuaWeiYun) Refresh(cdnId string, resultFunc RefreshResult, remoteFilePathList []string) error {
	auth := global.NewCredentialsBuilder().
		WithAk(this.accessKeyID).
		WithSk(this.accessKeySecret).
		Build()

	client := cdn.NewCdnClient(
		cdn.CdnClientBuilder().
			WithEndpoint(this.endPoint).
			WithCredential(auth).
			Build())

	request := &model.CreateRefreshTasksRequest{}
	_, err := client.CreateRefreshTasks(request)
	return err
}
