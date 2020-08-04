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
	"os"
	"path"
	"strings"
	"time"
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

var (
	apiUrl = "http://39.108.180.201:8888/api/v1"
	//apiUrl     = "http://127.0.0.1/api/v1"
	httpClient *http.Client
	//remoteDir  = "/root"
	remoteDir = "/home/stack"
	insideDir = "/home/stack/inside"
	outDir    = "/home/stack/outside"
)

const (
	HOST     = "39.108.180.201"
	USERNAME = "root"
	PASSWORD = "Hyj877286291"
)

func init() {
	httpClient = &http.Client{}
}
func (cliConf *ClientConfig) RunShell(shell string) string {
	var (
		session *ssh.Session
		err     error
	)

	if session, err = cliConf.SshClient.NewSession(); err != nil {
		log.Fatalln("error occurred:", err)
	}

	//执行shell
	if output, err := session.CombinedOutput(shell); err != nil {
		//log.Fatalln("error occurred:", err)
		cliConf.LastResult = err.Error()
	} else {
		cliConf.LastResult = string(output)
	}
	return cliConf.LastResult
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
		Timeout: 10 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", cliConf.Host, cliConf.Port)

	//获取client
	if sshClient, err = ssh.Dial("tcp", addr, &config); err != nil {
		log.Fatalln("error occurred:", err)
	}
	// create sftp client
	if sftpClient, err = sftp.NewClient(sshClient); err != nil {
		log.Fatalln("error occurred:", err)
	}

	cliConf.SshClient = sshClient
	cliConf.SftpClient = sftpClient
}
func sftpDownload(fileName string) *os.File {
	cliConf := new(ClientConfig)
	cliConf.connHost(HOST, 22, USERNAME, PASSWORD)
	srcFile, err := cliConf.SftpClient.Open(fileName)
	dstFile, err := os.Create(path.Join("./", fileName))
	if _, err = srcFile.WriteTo(dstFile); err != nil {
		log.Println(err)
	}
	defer srcFile.Close()
	fileReader, err := os.Open(fileName)
	if err != nil {
		log.Println(err)
	}
	_ = os.Remove(fileName)
	return fileReader
}
func main() {
	log.Println("监测云桌面是否有新文件")
	cliConf := new(ClientConfig)
	cliConf.connHost(HOST, 22, USERNAME, PASSWORD)
	fileList := strings.Split(cliConf.RunShell("ls -l "+"/root"+"| grep ^[^d] | awk '{print $9}'"), "\n")
	cliConf.SftpClient.Close()
	cliConf.SshClient.Close()
	for _, filename := range fileList[1:] {
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
		request, err := http.NewRequest(http.MethodPost, apiUrl+"/upload", bodyBuf)
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
}
