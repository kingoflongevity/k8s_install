package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHClient SSH客户端
type SSHClient struct {
	client *ssh.Client
}

// NewSSHClient 创建新的SSH客户端
func NewSSHClient(config SSHConfig) (*SSHClient, error) {
	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境应该使用更安全的HostKeyCallback
		Timeout:         30 * time.Second,
	}

	// 配置认证方式
	if config.PrivateKey != "" {
		// 使用私钥认证
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
	} else if config.Password != "" {
		// 使用密码认证
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(config.Password))
	} else {
		return nil, fmt.Errorf("either password or privateKey must be provided")
	}

	// 连接到SSH服务器
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", addr, err)
	}

	return &SSHClient{client: client}, nil
}

// SSHConfig SSH连接配置
type SSHConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
}

// Close 关闭SSH连接
func (c *SSHClient) Close() error {
	return c.client.Close()
}

// RunCommand 执行SSH命令
func (c *SSHClient) RunCommand(cmd string) (string, error) {
	// 创建SSH会话
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	// 设置命令执行超时
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// 执行命令
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(cmd)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out: %s", cmd)
		}
		return "", fmt.Errorf("command failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	return stdout.String(), nil
}

// UploadFile 上传文件到远程服务器
func (c *SSHClient) UploadFile(localPath, remotePath string) error {
	// 创建SFTP客户端
	sftpClient, err := sftp.NewClient(c.client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}
	defer sftpClient.Close()

	// 读取本地文件
	localFile, err := ioutil.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %v", err)
	}

	// 写入远程文件
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close()

	_, err = remoteFile.Write(localFile)
	if err != nil {
		return fmt.Errorf("failed to write remote file: %v", err)
	}

	return nil
}

// DownloadFile 从远程服务器下载文件
func (c *SSHClient) DownloadFile(remotePath, localPath string) error {
	// 创建SFTP客户端
	sftpClient, err := sftp.NewClient(c.client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}
	defer sftpClient.Close()

	// 读取远程文件
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %v", err)
	}
	defer remoteFile.Close()

	// 读取文件内容
	content, err := ioutil.ReadAll(remoteFile)
	if err != nil {
		return fmt.Errorf("failed to read remote file: %v", err)
	}

	// 写入本地文件
	err = ioutil.WriteFile(localPath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write local file: %v", err)
	}

	return nil
}

// TestConnection 测试SSH连接
func TestConnection(config SSHConfig) (bool, error) {
	client, err := NewSSHClient(config)
	if err != nil {
		return false, err
	}
	defer client.Close()

	// 执行简单命令测试连接
	_, err = client.RunCommand("echo 'hello'")
	if err != nil {
		return false, err
	}

	return true, nil
}
