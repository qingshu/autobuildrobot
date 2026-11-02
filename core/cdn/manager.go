package cdn

import (
	"errors"
	"strconv"
	"sync"
)

var cdnList map[string]IcdnClient
var cdnLock sync.RWMutex

func init() {
	cdnList = make(map[string]IcdnClient)
}

// 获取cdn对象
func GetCdnClient(cdnType, urlOfBucket, bucketName, accessKeyID, accessKeySecret string) (err error, cdnClient IcdnClient) {
	cdnKey := cdnType + bucketName

	//从缓存中获取数据
	cdnLock.RLock()
	if icdnClient, ok := cdnList[cdnKey]; ok {
		cdnLock.RUnlock()
		return nil, icdnClient
	}
	cdnLock.RUnlock()

	//缓存中没有则新建实例
	cdnLock.Lock()
	defer cdnLock.Unlock()
	switch cdnType {
	case strconv.Itoa(CdnType_ALiYun):
		{
			err, cdnClient = NewALiYun(urlOfBucket, bucketName, accessKeyID, accessKeySecret)
		}
	case strconv.Itoa(CdnType_HuaWeiYun):
		{
			err, cdnClient = NewHuaWeiYun(urlOfBucket, bucketName, accessKeyID, accessKeySecret)
		}
	case strconv.Itoa(CdnType_Aws):
		{
			err, cdnClient = NewAws(urlOfBucket, bucketName, accessKeyID, accessKeySecret)
		}
	case strconv.Itoa(CdnType_TXYun):
		{
			err, cdnClient = NewTXYun(urlOfBucket, bucketName, accessKeyID, accessKeySecret)
		}
	case strconv.Itoa(CdnType_UCloud):
		{
			err, cdnClient = NewUCloud(urlOfBucket, bucketName, accessKeyID, accessKeySecret)
		}
	default:
		return errors.New(cdnType + "cdn不存在"), nil
	}
	if nil != err {
		return err, nil
	}
	cdnList[cdnKey] = cdnClient
	return nil, cdnClient
}
