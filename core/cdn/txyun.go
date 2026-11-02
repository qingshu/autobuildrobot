package cdn

import (
	"context"
	"errors"
	"fmt"
	"github.com/tencentyun/cos-go-sdk-v5"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

//参考文档：https://cloud.tencent.com/document/product/436/31215

type TXYun struct {
	endpoint        string
	accessKeyId     string
	accessKeySecret string
	bucketName      string
	cosobject       *cos.ObjectService
}

func NewTXYun(endpointOfBucket, bucketName, accessKeyID, accessKeySecret string) (error, *TXYun) {
	//bucket的命名规则为{name}-{appid} ，此处填写的存储桶名称必须为此格式
	urlTemp := fmt.Sprintf("https://%s.%s", bucketName, endpointOfBucket)
	u, _ := url.Parse(urlTemp)
	b := &cos.BaseURL{BucketURL: u}
	c := cos.NewClient(b, &http.Client{
		Timeout: 100 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  os.Getenv(accessKeyID),
			SecretKey: os.Getenv(accessKeySecret),
		},
	})
	if nil == c {
		return errors.New("创建腾讯云对象失败"), nil
	}

	txy := new(TXYun)
	txy.endpoint = endpointOfBucket
	txy.accessKeyId = accessKeyID
	txy.accessKeySecret = accessKeySecret
	txy.bucketName = bucketName
	txy.cosobject = c.Object
	return nil, txy
}

func (this *TXYun) IsExistFile(remoteFilePath string) (bool, error) {
	if nil == this.cosobject {
		return false, errors.New("IsExistFile,txyun client not exist,please check!")
	}
	return this.cosobject.IsExist(context.Background(), remoteFilePath)
}

// 上传文件
func (this *TXYun) UploadFile(localFilePath, remoteFilePath string) error {
	if nil == this.cosobject {
		return errors.New("UploadFile,txyun client not exist,please check!")
	}
	_, err := this.cosobject.PutFromFile(context.Background(), remoteFilePath, localFilePath, nil)
	return err
}

// 下载文件
func (this *TXYun) DownFile(remoteFilePath string) (err error, data []byte) {
	if nil == this.cosobject {
		return errors.New("DownFile,txyun client not exist,please check!"), nil
	}
	resp, err := this.cosobject.Get(context.Background(), remoteFilePath, nil)
	defer func() {
		if resp == nil {
			return
		}
		resp.Body.Close()
	}()
	if nil != err {
		return err, nil
	}
	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err, nil
	}
	return nil, data
}

func (this *TXYun) DownFile2Local(remoteFilePath, downloadedFileName string) error {
	if nil == this.cosobject {
		return errors.New("DownFile2Local,txyun client not exist,please check!")
	}
	_, err := this.cosobject.GetToFile(context.Background(), remoteFilePath, downloadedFileName, nil)
	return err
}

func (this *TXYun) DeleteFile(remoteFilePath string) error {
	if nil == this.cosobject {
		return errors.New("DeleteFile,txyun client not exist,please check!")
	}
	_, err := this.cosobject.Delete(context.Background(), remoteFilePath)
	return err
}

func (this *TXYun) CopyFile(resPath, targetPath string) error {
	if nil == this.cosobject {
		return errors.New("CopyFile,txyun client not exist,please check!")
	}
	sourcePath := fmt.Sprintf("%s.%s/%s", this.bucketName, this.endpoint, resPath)
	_, _, err := this.cosobject.Copy(context.Background(), targetPath, sourcePath, nil)
	return err
}

// 预热
func (this *TXYun) Refresh(cdnId string, resultFunc RefreshResult, remoteFilePathList []string) error {
	return errors.New("未实现预热功能")
}
