package tool

import (
	"autobuildrobot/internal/processrun"
	log "autobuildrobot/mylog"
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/astaxie/beego/cache"
	"github.com/axgle/mahonia"
	"golang.org/x/crypto/ssh"
)

type ExecCommandFunc func(result string)

// 执行简单指令,等待执行完毕并返回标准输出
func ExecSimpleCmd(cmdName, s string) (string, error) {
	//函数返回一个*Cmd，用于使用给出的参数执行name指定的程序
	log.Info("开始执行：" + s)
	cmd := exec.Command(cmdName, "-c", s)

	//读取io.Writer类型的cmd.Stdout，再通过bytes.Buffer(缓冲byte类型的缓冲器)将byte类型转化为string类型(out.String():这是bytes类型提供的接口)
	var out bytes.Buffer
	cmd.Stdout = &out

	//Run执行c包含的命令，并阻塞直到完成。  这里stdout被取出，cmd.Wait()无法正确获取stdin,stdout,stderr，则阻塞在那了
	err := cmd.Run()
	var enc mahonia.Decoder
	if runtime.GOOS == "windows" {
		enc = mahonia.NewDecoder("gbk")
	} else {
		enc = mahonia.NewDecoder("utf-8")
	}

	return enc.ConvertString(out.String()), err
}

// 执行相对负责的指令,实时输出结果
func ExecComplexCmdWithContext(cmdName, command string, ctx context.Context, output ExecCommandFunc) error {
	return ExecCommandArgs(ctx, "", cmdName, []string{"-c", command}, output)
}

func ExecComplexCmd(cmdName, command string, output ExecCommandFunc) error {
	return ExecComplexCmdWithContext(cmdName, command, context.Background(), output)
}

// ExecCommandArgs avoids shell interpolation of user-controlled arguments.
func ExecCommandArgs(ctx context.Context, dir, executable string, args []string, output ExecCommandFunc) error {
	decoder := mahonia.NewDecoder("utf-8")
	if runtime.GOOS == "windows" {
		decoder = mahonia.NewDecoder("gbk")
	}
	return processrun.Run(ctx, dir, executable, args, func(line string) {
		if output != nil {
			output(decoder.ConvertString(line))
		}
	})
}

// 执行远端shell
func RemoteShell(cmd, account, pwd, addr string, execCommandFunc ExecCommandFunc) error {
	//beego.Run()
	session, err := SSHConnect(account, pwd, addr)
	if err != nil {
		return err
	}

	defer session.Close()
	if nil == execCommandFunc {
		session.Run(cmd)
		return nil
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	go func() {
		session.Run(cmd)
	}()

	//创建一个流来读取管道内内容，这里逻辑是通过一行一行的读取的
	reader := bufio.NewReader(stdout)
	var enc mahonia.Decoder
	if runtime.GOOS == "windows" {
		enc = mahonia.NewDecoder("gbk")
	} else {
		enc = mahonia.NewDecoder("utf-8")
	}

	//实时循环读取输出流中的一行内容
	for {
		line, err2 := reader.ReadString('\n')
		if err2 != nil || io.EOF == err2 {
			log.Error("err2 != nil || io.EOF == err2")
			break
		}
		if line == "\r\n" {
			continue
		}
		temp := enc.ConvertString(line)
		execCommandFunc(temp)
	}
	return nil
}

// 建立一个ssh链接
func SSHConnect(user, password, addr string) (*ssh.Session, error) {
	var (
		auth         []ssh.AuthMethod
		clientConfig *ssh.ClientConfig
		client       *ssh.Client
		session      *ssh.Session
		err          error
	)

	// get auth method
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(password))
	hostKeyCallbk := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return nil
	}

	clientConfig = &ssh.ClientConfig{
		User: user,
		Auth: auth,
		// Timeout:             30 * time.Second,
		HostKeyCallback: hostKeyCallbk,
	}

	// connet to ssh
	if client, err = ssh.Dial("tcp", addr, clientConfig); err != nil {
		return nil, err
	}

	// create session
	if session, err = client.NewSession(); err != nil {
		return nil, err
	}
	return session, nil
}

// 发送http请求
func Http(requestType, url, content string) (error, []byte) {
	//创建一个请求
	result := ""
	req, err := http.NewRequest(requestType, url, strings.NewReader(content))
	if err != nil {
		result = "发送http请求异常：" + err.Error()
		log.Error(result)
		return errors.New(result), nil
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		result = "发送http请求失败：" + err.Error()
		log.Error(result)
		return errors.New(result), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP status %d", resp.StatusCode), nil
	}
	body, err := ioutil.ReadAll(resp.Body)
	return err, body
}

// 序列化json
func MarshalJson(jsonData interface{}) string {
	data, _ := json.Marshal(jsonData)
	return string(data)
}

// 反序列化json
func UnmarshJson(jsonData []byte, data interface{}) error {
	return json.Unmarshal([]byte(jsonData), data)
}

var gobDataFilePath = filepath.Join(filepath.Dir(GetFileRootPath()), "gobData") //gob文件夹名字

// 读取gob文件
func ReadGobFile(fileName string, data interface{}) {
	var dataFile = path.Join(gobDataFilePath, fileName)
	_, err := os.Stat(dataFile)
	if err == nil {
		content, err := ioutil.ReadFile(dataFile)
		if err != nil {
			log.Error("读取用户数据配置文件失败：" + err.Error())
			return
		}
		buf := bytes.NewBuffer(content)
		dec := gob.NewDecoder(buf)
		dec.Decode(data)
	} else {
		_, existPath := os.Stat(gobDataFilePath)
		if nil != existPath {
			os.MkdirAll(gobDataFilePath, os.ModePerm)
		}
	}
}

// 保存gob数据
func SaveGobFile(fileName string, _data interface{}) (result string) {
	//编码并存储
	data, errEncodeUser := cache.GobEncode(_data)
	if nil != errEncodeUser {
		result = "编码用户数据失败：" + errEncodeUser.Error()
		log.Error(result)
		return
	}
	fileObj, err := os.Create(path.Join(gobDataFilePath, fileName))
	if err != nil {
		result = "获取用户文件失败：" + err.Error()
		return
	}
	writer := bufio.NewWriter(fileObj)
	defer writer.Flush()
	writer.Write(data)
	return
}

func GetFileRootPath() string {
	//暂时以可执行文件所在目录为文件根目录
	exePath, err := os.Executable()
	if err != nil {
		panic("获取可执行文件目录异常！" + err.Error())
	}
	return exePath
}

// 判断文件是否存在
func CheckFileIsExist(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
}

// 拷贝文件
func CopyFile(srcName, dstName string) (written int64, err error) {
	if CheckFileIsExist(dstName) {
		os.Remove(dstName)
	}
	src, err := os.Open(srcName)
	if err != nil {
		return
	}
	defer src.Close()
	dst, err := os.OpenFile(dstName, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer dst.Close()
	return io.Copy(dst, src)
}

// 判断文件夹是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	//isnotexist来判断，是不是不存在的错误
	if os.IsNotExist(err) { //如果返回的错误类型使用os.isNotExist()判断为true，说明文件或者文件夹不存在
		return false, nil
	}
	return false, err //如果有错误了，但是不是不存在的错误，所以把这个错误原封不动的返回
}

// 创建文件夹
func CreateDir(_path string) error {
	if _, err := os.Stat(_path); os.IsNotExist(err) {
		// 必须分成两步
		// 先创建文件夹
		mkErr := os.MkdirAll(_path, os.ModePerm)
		if nil != mkErr {
			return mkErr
		}

		// 再修改权限
		return os.Chmod(_path, os.ModePerm)
	} else {
		return err
	}
}

// 将项目名转换成md5串
func CalcMd5(source string) string {
	hash := md5.New()
	hash.Write([]byte(source))
	cipherText2 := hash.Sum(nil)
	md5Sign := make([]byte, 32)
	hex.Encode(md5Sign, cipherText2)
	return string(md5Sign)
}

// 计算文件md5值
func CalcFileMd5(filePath string) string {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return ""
	}
	md5 := md5.New()
	md5.Write(data)
	return hex.EncodeToString(md5.Sum(nil))
}

// 异或加密数据
func XOREncryptionData(data []byte, key string) []byte {
	if key == "" {
		return data
	}
	length := len(data)
	keyLength := len(key)
	if keyLength <= 0 {
		return data
	}
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		index := i % keyLength
		result[i] = data[i] ^ key[index]
	}
	return result
}

// 混淆名字
func DealZZZZZName(name, zzzTimeStamp string) string {
	//处理混淆
	if zzzTimeStamp == "" {
		return name
	}
	return CalcMd5(name + zzzTimeStamp)
}

// 返回原名和混淆名字隐射表
func GetOriginalAndZZZName(mappingFilePath, zzzTtimeStamp string) (map[string]string, error) {
	mapping := make(map[string]string)
	if zzzTtimeStamp == "" {
		return mapping, nil
	}
	mappingFile, err := os.Open(mappingFilePath)
	if err != nil {
		fmt.Println("打开项目本地映射文件失败 os.Open:", err)
		return mapping, errors.New("打开项目本地映射文件失败 os.Open:" + err.Error())
	}
	mappingFileByts, err := ioutil.ReadAll(mappingFile)
	mappingFile.Close()
	if err != nil {
		return mapping, errors.New("读取项目本地映射文件失败 os.Open:" + err.Error())
	}
	mappingDatas := string(mappingFileByts)
	if mappingDatas == "" {
		return mapping, errors.New("隐射文件为空！！")
	}
	list := strings.Split(mappingDatas, "\r\n")
	if len(list) <= 1 {
		return mapping, errors.New("解析隐射文件失败！！")
	}

	//随便取第一个看是否跟文件隐射一致
	isMatchFiles := false
	for _, v := range list {
		if v == "" {
			continue
		}
		firstData := strings.Split(v, ":")
		if len(firstData) < 2 {
			log.Error("隐射文件不是隐射内容？：" + v)
			continue
		}
		isMatchFiles = DealZZZZZName(firstData[0], zzzTtimeStamp) == firstData[1]
		break
	}

	//缓存隐射表
	if isMatchFiles {
		//如果files混淆名一致则直接取当前映射表内容
		for _, v := range list {
			if v == "" {
				continue
			}
			oneLines := strings.Split(v, ":")
			if len(oneLines) < 2 {
				log.Error("隐射文件不是隐射内容？：" + v)
				continue
			}
			mapping[oneLines[1]] = oneLines[0]
		}
	} else {
		//否则重新计算隐射内容
		for _, v := range list {
			oneLines := strings.Split(v, ":")
			zzzName := DealZZZZZName(oneLines[0], zzzTtimeStamp)
			mapping[zzzName] = oneLines[0]
		}
	}
	return mapping, nil
}
