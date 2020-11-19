package main

import (
	"bytes"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

var (
	apiUrl = "http://39.108.180.201:8888/api/v1"
	//apiUrl     = "http://127.0.0.1/api/v1"
	httpClient *http.Client
	//remoteDir  = "/root"
	//remoteDir = "/home/stack"
	insideDir  = "/home/stack/inside"
	outsideDir = "/home/stack/outside"
	wg         sync.WaitGroup
)

type ClientConfig struct {
	Host       string
	Port       int64
	Username   string
	Password   string
	SshClient  *ssh.Client
	SftpClient *sftp.Client
	LastResult string
}

const (
	//HOST      = "39.108.180.201"
	//USERNAME  = "root"
	//PASSWORD  = "Hyj877286291"
	HOST      = "10.8.7.77"
	USERNAME  = "stack"
	PASSWORD  = "Picc123456"
	HttpProxy = "http://proxy.piccnet.com.cn:3128"
)

func init() {
	proxy := func(_ *http.Request) (*url.URL, error) {
		return url.Parse(HttpProxy)
	}

	httpTransport := &http.Transport{
		Proxy: proxy,
	}

	httpClient = &http.Client{
		Transport: httpTransport,
	}
}

func main() {
	wg.Add(2)
	downloadTickChan := time.Tick(time.Second * 3)
	uploadTickChan := time.Tick(time.Second * 5)
	currentTask := ""
	//下载文件到云桌面
	go download(downloadTickChan, currentTask)
	//上传文件到外网服务器
	go upload(uploadTickChan)
	wg.Wait()
}
func getContent(filename string) []byte {
	log.Println("开始获取服务器文件...")
	request, err := http.NewRequest(http.MethodGet, apiUrl+"/downloadFromServer?filename="+filename, nil)
	if err != nil {
		log.Println(err)
	}
	resp, err := httpClient.Do(request)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	response, _ := ioutil.ReadAll(resp.Body)

	return response
}
func deleteFile(filename string) {
	fmt.Println("开始删除文件：", filename)
	request, err := http.NewRequest(http.MethodGet, apiUrl+"/deleteFile?filename="+filename, nil)
	if err != nil {
		log.Println(err)
	}
	resp, err := httpClient.Do(request)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	log.Println(string(respBody))
}
func (cliConf *ClientConfig) connHost(host string, port int64, username, password string) {
	var (
		sshClient  *ssh.Client
		sftpClient *sftp.Client
		err        error
	)
	cliConf.Host = host
	cliConf.Port = port
	cliConf.Username = username
	cliConf.Password = password
	cliConf.Port = port

	config := ssh.ClientConfig{
		User: cliConf.Username,
		Auth: []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 0,
	}
	addr := fmt.Sprintf("%s:%d", cliConf.Host, cliConf.Port)

	//获取client
	if sshClient, err = ssh.Dial("tcp", addr, &config); err != nil {
		log.Println("error occurred:", err)
	}
	// create sftp client
	if sftpClient, err = sftp.NewClient(sshClient); err != nil {
		log.Println("error occurred:", err)
	}

	cliConf.SshClient = sshClient
	cliConf.SftpClient = sftpClient
}
func (cliConf *ClientConfig) RunShell(shell string) string {
	var (
		session *ssh.Session
		err     error
	)

	if session, err = cliConf.SshClient.NewSession(); err != nil {
		log.Println("error occurred:", err)
	}

	//执行shell
	if output, err := session.CombinedOutput(shell); err != nil {
		//log.Println("error occurred:", err)
		cliConf.LastResult = err.Error()
	} else {
		cliConf.LastResult = string(output)
	}
	return cliConf.LastResult
}
func sftpUpload(fileName string, srcFile []byte) {
	cliConf := new(ClientConfig)
	cliConf.connHost(HOST, 22, USERNAME, PASSWORD)
	dstFile, err := cliConf.SftpClient.Create(path.Join(insideDir, fileName))
	if err != nil {
		log.Println(err)
	}
	defer cliConf.SshClient.Close()
	defer cliConf.SftpClient.Close()
	defer dstFile.Close()
	total, _ := dstFile.Write(srcFile)
	log.Println(fileName, "文件上传完成，共", total/1024/1024, "M")
}
func sftpDownload(fileName string) *os.File {
	log.Println(fileName)
	cliConf := new(ClientConfig)
	cliConf.connHost(HOST, 22, USERNAME, PASSWORD)
	srcFile, err := cliConf.SftpClient.Open(path.Join(outsideDir, fileName))
	dstFile, err := os.Create(path.Join("./", fileName))
	if _, err = srcFile.WriteTo(dstFile); err != nil {
		log.Println(err)
	}
	cliConf.RunShell("rm -rf " + path.Join(outsideDir, fileName))
	defer srcFile.Close()
	fileReader, err := os.Open(path.Join("./", fileName))
	if err != nil {
		log.Println(err)
	}
	_ = os.Remove(path.Join("./", fileName))
	return fileReader
}
func download(downloadTickChan <-chan time.Time, currentTask string) {
	for {
		log.Println("监测服务器是否有新文件")
		request, err := http.NewRequest(http.MethodGet, apiUrl+"/singleFile", nil)
		if err != nil {
			log.Println(err)
		}
		resp, err := httpClient.Do(request)
		if err != nil {
			log.Println(err)
			continue
		}
		response, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		fileName := strings.ReplaceAll(string(response), "\"", "")
		if err != nil {
			log.Println(err)
		}

		if len(fileName) != 0 && fileName != "null" && currentTask != fileName {
			//获取数据
			content := getContent(fileName)
			currentTask = fileName
			//删除临时文件
			deleteFile(fileName)
			//传入10.8.7.77
			log.Println("文件已从服务器拉至本地，开始上传至内网服务器...")
			sftpUpload(fileName, content)
			currentTask = ""
		}
		<-downloadTickChan
	}
}

func upload(uploadTickChan <-chan time.Time) {
	for {
		log.Println("监测云桌面是否有新文件")
		cliConf := new(ClientConfig)
		cliConf.connHost(HOST, 22, USERNAME, PASSWORD)
		fileList := strings.Split(cliConf.RunShell("ls -l "+outsideDir+"| grep ^[^d] | awk '{print $9}'"), "\n")
		cliConf.SftpClient.Close()
		cliConf.SshClient.Close()
		for _, filename := range fileList[1:] {
			if filename == "" {
				continue
			}
			fileReader := sftpDownload(filename)
			bodyBuf := &bytes.Buffer{}
			bodyWriter := multipart.NewWriter(bodyBuf)
			fileWriter, err := bodyWriter.CreateFormFile("file", filename)
			if err != nil {
				log.Println(err)
			}
			_, err = io.Copy(fileWriter, fileReader)
			if err != nil {
				log.Println(err)
			}
			contentType := bodyWriter.FormDataContentType()
			_ = bodyWriter.Close()
			//上传文件
			request, err := http.NewRequest(http.MethodPost, apiUrl+"/uploadToServer", bodyBuf)
			if err != nil {
				log.Println(err)
			}
			request.Header.Set("Content-Type", contentType)
			resp, err := httpClient.Do(request)
			if err != nil {
				log.Println(err)
				continue
			}
			response, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			log.Println(string(response))
		}
		<-uploadTickChan
	}
}
