package main

import (
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"log"
	"net"
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

const (
	HOST     = "39.108.180.201"
	USERNAME = "root"
	PASSWORD = "Hyj877286291"
)

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
func sftpDownload() {
	cliConf := new(ClientConfig)
	cliConf.connHost(HOST, 22, USERNAME, PASSWORD)
	defer cliConf.SshClient.Close()
	defer cliConf.SftpClient.Close()
	fileList := strings.Split(cliConf.RunShell("ls -l "+"/root"+"| grep ^[^d] | awk '{print $9}'"), "\n")
	for _, file := range fileList[1:] {
		//上传文件
		fmt.Println(file)
	}
}
func main() {
	sftpDownload()
}
