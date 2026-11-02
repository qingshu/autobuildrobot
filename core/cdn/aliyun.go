package cdn

import (
	log "autobuildrobot/mylog"
	"errors"
	cdn20180510 "github.com/alibabacloud-go/cdn-20180510/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"io/ioutil"
)

//参考文档：https://help.aliyun.com/document_detail/32145.html

type ALiYun struct {
	endpoint        string
	accessKeyId     string
	accessKeySecret string
	bucketName      string
	bucket          *oss.Bucket
}

func NewALiYun(endpointOfBucket, bucketName, accessKeyID, accessKeySecret string) (error, *ALiYun) {
	client, err := oss.New(endpointOfBucket, accessKeyID, accessKeySecret)
	if err != nil {
		log.Error(err)
		return err, nil
	}
	// Get Bucket
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		log.Error(err)
		return err, nil
	}
	aly := new(ALiYun)
	aly.endpoint = endpointOfBucket
	aly.accessKeyId = accessKeyID
	aly.accessKeySecret = accessKeySecret
	aly.bucketName = bucketName
	aly.bucket = bucket
	return nil, aly
}

func (this *ALiYun) IsExistFile(remoteFilePath string) (bool, error) {
	if nil == this.bucket {
		return false, errors.New("IsExistFile,aliyun bucket not exist,please check!")
	}
	return this.bucket.IsObjectExist(remoteFilePath)
}

// 上传文件
func (this *ALiYun) UploadFile(localFilePath, remoteFilePath string) error {
	if nil == this.bucket {
		return errors.New("UploadFile,aliyun bucket not exist,please check!")
	}
	return this.bucket.PutObjectFromFile(remoteFilePath, localFilePath)
}

// 下载文件
func (this *ALiYun) DownFile(remoteFilePath string) (err error, data []byte) {
	if nil == this.bucket {
		return errors.New("DownFile,aliyun bucket not exist,please check!"), nil
	}
	body, err := this.bucket.GetObject(remoteFilePath)
	defer func() {
		if body == nil {
			return
		}
		body.Close()
	}()
	if nil != err {
		return err, nil
	}
	data, err = ioutil.ReadAll(body)
	if err != nil {
		return err, nil
	}
	return nil, data
}

func (this *ALiYun) DownFile2Local(remoteFilePath, downloadedFileName string) error {
	if nil == this.bucket {
		return errors.New("DownFile2Local,aliyun bucket not exist,please check!")
	}
	return this.bucket.GetObjectToFile(remoteFilePath, downloadedFileName)
}

func (this *ALiYun) DeleteFile(remoteFilePath string) error {
	if nil == this.bucket {
		return errors.New("DeleteFile,aliyun bucket not exist,please check!")
	}
	return this.bucket.DeleteObject(remoteFilePath)
}

func (this *ALiYun) CopyFile(resPath, targetPath string) error {
	if nil == this.bucket {
		return errors.New("CopyFile,aliyun bucket not exist,please check!")
	}
	_, err := this.bucket.CopyObject(resPath, targetPath)
	return err
}

// 预热
func (this *ALiYun) Refresh(cdnId string, resultFunc RefreshResult, remoteFilePathList []string) error {
	config := &openapi.Config{
		// 您的AccessKey ID
		AccessKeyId: &this.accessKeyId,
		// 您的AccessKey Secret
		AccessKeySecret: &this.accessKeySecret,
		Endpoint:        &this.endpoint,
	}
	client, _err := cdn20180510.NewClient(config)
	if _err != nil {
		return _err
	}

	pushObjectCacheRequest := &cdn20180510.PushObjectCacheRequest{}
	_, _err = client.PushObjectCache(pushObjectCacheRequest)
	if _err != nil {
		return _err
	}
	return _err
}
