package cdn

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

//参考文档：https://help.aliyun.com/document_detail/32145.html

type Aws struct {
	endpoint        string
	accessKeyId     string
	accessKeySecret string
	bucketName      string
	bucket          *s3.S3
}

func NewAws(endpointOfBucket, bucketName, accessKeyID, accessKeySecret string) (error, *Aws) {
	//获取一个AWS session
	region := strings.Replace(endpointOfBucket, "s3.", "", -1)
	region = strings.Replace(region, ".amazonaws.com", "", -1)
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(accessKeyID, accessKeySecret, ""),
		Region:      aws.String(region),
		Endpoint:    aws.String(endpointOfBucket)})

	if err != nil {
		return err, nil
	}
	aws := new(Aws)
	aws.endpoint = endpointOfBucket
	aws.accessKeyId = accessKeyID
	aws.accessKeySecret = accessKeySecret
	aws.bucketName = bucketName
	aws.bucket = s3.New(sess)
	return nil, aws
}

func (this *Aws) IsExistFile(remoteFilePath string) (bool, error) {
	if nil == this.bucket {
		return false, errors.New("IsExistFile,Aws bucket not exist,please check!")
	}
	result, err := this.bucket.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(this.bucketName),
		Key:    aws.String(remoteFilePath),
	})
	if result.ContentLength == nil {
		return false, nil
	}
	return true, err
}

// 上传文件
func (this *Aws) UploadFile(localFilePath, remoteFilePath string) error {
	if nil == this.bucket {
		return errors.New("UploadFile,aliyun bucket not exist,please check!")
	}
	fp, err := os.Open(localFilePath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(1200)*time.Second) //一个文件不能超过20分钟
	_, err = this.bucket.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(this.bucketName),
		Key:    aws.String(remoteFilePath),
		Body:   fp,
		//ACL:    aws.String("public-read"),//用于控制每个文件访问权限
	})
	cancel()
	fp.Close()
	return err
}

// 下载文件
func (this *Aws) DownFile(remoteFilePath string) (err error, data []byte) {
	if nil == this.bucket {
		return errors.New("DownFile,Aws bucket not exist,please check!"), nil
	}
	sContext, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	out, err := this.bucket.GetObjectWithContext(sContext, &s3.GetObjectInput{
		Bucket: aws.String(this.bucketName),
		Key:    aws.String(remoteFilePath),
	})
	defer func() {
		cancel()
		if out == nil || out.Body == nil {
			return
		}
		out.Body.Close()
	}()
	if nil != err {
		return err, nil
	}
	data, err = ioutil.ReadAll(out.Body)
	if err != nil {
		return err, nil
	}
	return nil, data
}

func (this *Aws) DownFile2Local(remoteFilePath, downloadedFileName string) error {
	if nil == this.bucket {
		return errors.New("DownFile2Local,Aws bucket not exist,please check!")
	}

	sContext, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer func() {
		cancel()
	}()

	w, err := os.OpenFile(downloadedFileName, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	params := s3.GetObjectInput{
		Bucket: aws.String(this.bucketName),
		Key:    aws.String(remoteFilePath),
	}
	d := s3manager.NewDownloaderWithClient(this.bucket)
	_, err = d.DownloadWithContext(sContext, w, &params)
	return err
}

func (this *Aws) DeleteFile(remoteFilePath string) error {
	if nil == this.bucket {
		return errors.New("DeleteFile,Aws bucket not exist,please check!")
	}
	_, err := this.bucket.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(this.bucketName),
		Key:    aws.String(remoteFilePath),
	})
	return err
}

func (this *Aws) CopyFile(resPath, targetPath string) error {
	if nil == this.bucket {
		return errors.New("CopyFile,Aws bucket not exist,please check!")
	}
	sContext, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	_, err := this.bucket.CopyObjectWithContext(sContext, &s3.CopyObjectInput{
		Bucket:     aws.String(this.bucketName),
		CopySource: aws.String(this.bucketName + "/" + resPath),
		Key:        aws.String(targetPath),
	})
	cancel()
	return err
}

// 预热
func (this *Aws) Refresh(cdnId string, resultFunc RefreshResult, remoteFilePathList []string) error {
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(this.accessKeyId, this.accessKeySecret, ""),
	})
	if err != nil {
		return err
	}

	//转成aws的数组结构
	filePaths := make([]*string, 0)
	for _, v := range remoteFilePathList {
		filePaths = append(filePaths, aws.String(v))
	}

	cf := cloudfront.New(sess)
	input := cloudfront.CreateInvalidationInput{}
	input.DistributionId = &cdnId
	input.InvalidationBatch = &cloudfront.InvalidationBatch{}
	strTime := strconv.FormatInt(time.Now().Unix(), 10)
	input.InvalidationBatch.SetCallerReference(strTime)
	cfPath := cloudfront.Paths{}
	quantity := int64(len(filePaths))
	cfPath.Quantity = &quantity
	cfPath.SetItems(filePaths)
	input.InvalidationBatch.SetPaths(&cfPath)
	output, err := cf.CreateInvalidation(&input)
	if err != nil {
		return err
	}

	//检查是否完成
	process := *output.Invalidation.Status
	for process != "Completed" {
		resultFunc(process)
		time.Sleep(time.Duration(10) * time.Second)
	}
	return nil
}
