package ssh

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"k8s-installer/log"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHClient SSH客户端
type SSHClient struct {
	client     *ssh.Client
	logManager interface {
		CreateLog(logEntry interface{}) error
	}
	nodeID   string
	nodeName string
}

// OutputCallback 实时输出回调函数
type OutputCallback func(line string)

// SetLogManager 设置日志管理器
func (c *SSHClient) SetLogManager(logManager interface {
	CreateLog(logEntry interface{}) error
}) {
	c.logManager = logManager
}

// SetNodeInfo 设置节点信息
func (c *SSHClient) SetNodeInfo(nodeID, nodeName string) {
	c.nodeID = nodeID
	c.nodeName = nodeName
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
		return nil, fmt.Errorf("either password or privateKey must be provided for SSH connection to %s:%d", config.Host, config.Port)
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

// RunCommand 执行SSH命令，并记录完整的执行日志到日志管理系统
func (c *SSHClient) RunCommand(cmd string) (string, error) {
	// 创建SSH会话
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	// 设置命令执行超时（60分钟），适应Kubernetes组件安装的耗时过程
	ctx, cancel := context.WithTimeout(context.Background(), 3600*time.Second)
	defer cancel()

	// 执行命令
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// 记录命令开始执行的时间
	executionStartTime := time.Now()

	// 拆分命令，记录每个步骤
	cmdLines := strings.Split(cmd, "\n")
	var filteredCmdLines []string
	for _, line := range cmdLines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			filteredCmdLines = append(filteredCmdLines, line)
		}
	}

	// 构建命令执行开始的日志
	startLogEntry := log.LogEntry{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		NodeID:    c.nodeID,
		NodeName:  c.nodeName,
		Operation: "SSHCommandExecution",
		Command:   cmd,
		Output:    fmt.Sprintf("开始执行命令，共 %d 个步骤\n命令: %s\n", len(filteredCmdLines), cmd),
		Status:    "running",
		CreatedAt: executionStartTime,
		UpdatedAt: executionStartTime,
	}

	// 将开始日志写入日志管理系统
	if c.logManager != nil {
		c.logManager.CreateLog(startLogEntry)
	}

	// 记录每个步骤的执行
	for i, stepCmd := range filteredCmdLines {
		stepLogEntry := log.LogEntry{
			ID:        fmt.Sprintf("%d-%d", time.Now().UnixNano(), i),
			NodeID:    c.nodeID,
			NodeName:  c.nodeName,
			Operation: "StepExecution",
			Command:   stepCmd,
			Output:    fmt.Sprintf("执行第 %d/%d 步: %s\n正在执行: 开始执行命令...", i+1, len(filteredCmdLines), stepCmd),
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if c.logManager != nil {
			c.logManager.CreateLog(stepLogEntry)
		}
	}

	err = session.Run(cmd)

	// 记录命令执行结束的时间和耗时
	executionEndTime := time.Now()
	executionDuration := executionEndTime.Sub(executionStartTime)

	// 构建完整的日志记录
	logOutput := fmt.Sprintf("=== SSH命令执行日志 ===\n")
	logOutput += fmt.Sprintf("命令: %s\n", cmd)
	logOutput += fmt.Sprintf("开始时间: %s\n", executionStartTime.Format("2006-01-02 15:04:05"))
	logOutput += fmt.Sprintf("结束时间: %s\n", executionEndTime.Format("2006-01-02 15:04:05"))
	logOutput += fmt.Sprintf("执行耗时: %v\n", executionDuration)
	logOutput += fmt.Sprintf("\n=== 标准输出 ===\n%s\n", stdout.String())
	logOutput += fmt.Sprintf("=== 标准错误 ===\n%s\n", stderr.String())

	// 记录每个步骤的执行结果
	for i, stepCmd := range filteredCmdLines {
		stepResultLogEntry := log.LogEntry{
			ID:        fmt.Sprintf("%d-%d", executionStartTime.UnixNano(), i),
			NodeID:    c.nodeID,
			NodeName:  c.nodeName,
			Operation: "StepExecution",
			Command:   stepCmd,
			Output:    fmt.Sprintf("执行第 %d/%d 步: %s\n执行成功\n", i+1, len(filteredCmdLines), stepCmd),
			Status:    "success",
			CreatedAt: executionStartTime,
			UpdatedAt: executionEndTime,
		}

		if c.logManager != nil {
			c.logManager.CreateLog(stepResultLogEntry)
		}
	}

	// 打印完整日志到控制台
	fmt.Println(logOutput)

	// 构建命令执行结束的日志
	status := "success"
	if err != nil {
		status = "failed"
	}

	endLogEntry := log.LogEntry{
		ID:        startLogEntry.ID,
		NodeID:    c.nodeID,
		NodeName:  c.nodeName,
		Operation: "SSHCommandExecution",
		Command:   cmd,
		Output:    logOutput,
		Status:    status,
		CreatedAt: executionStartTime,
		UpdatedAt: executionEndTime,
	}

	// 将结束日志写入日志管理系统
	if c.logManager != nil {
		c.logManager.CreateLog(endLogEntry)
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after 60 minutes: %s\nStdout: %s\nStderr: %s", cmd, stdout.String(), stderr.String())
		}
		// 区分不同类型的错误
		if exitErr, ok := err.(*ssh.ExitError); ok {
			// 检查是否是信号中断
			if exitErr.Signal() == "TERM" {
				return "", fmt.Errorf("command was terminated by signal SIGTERM after 60 minutes: %s\nStdout: %s\nStderr: %s", cmd, stdout.String(), stderr.String())
			}
			return "", fmt.Errorf("command failed with exit code %d: %s\nStdout: %s\nStderr: %s", exitErr.ExitStatus(), cmd, stdout.String(), stderr.String())
		}
		return "", fmt.Errorf("command failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	return stdout.String(), nil
}

// RunCommandWithOutput 执行SSH命令并实时输出结果
func (c *SSHClient) RunCommandWithOutput(cmd string, callback OutputCallback) (string, error) {
	// 创建SSH会话
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	// 设置命令执行超时（60分钟）
	ctx, cancel := context.WithTimeout(context.Background(), 3600*time.Second)
	defer cancel()

	// 获取会话的标准输出和标准错误
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe: %v", err)
	}

	// 记录命令开始执行的时间
	executionStartTime := time.Now()

	// 构建命令执行开始的日志
	startLogEntry := log.LogEntry{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		NodeID:    c.nodeID,
		NodeName:  c.nodeName,
		Operation: "SSHCommandExecution",
		Command:   cmd,
		Output:    "命令开始执行...",
		Status:    "running",
		CreatedAt: executionStartTime,
		UpdatedAt: executionStartTime,
	}

	// 将开始日志写入日志管理系统
	if c.logManager != nil {
		c.logManager.CreateLog(startLogEntry)
	}

	// 启动命令执行
	err = session.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start command: %v", err)
	}

	// 实时读取标准输出
	var stdoutBuf strings.Builder
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutBuf.WriteString(line + "\n")
			callback(line)
		}
	}()

	// 实时读取标准错误
	var stderrBuf strings.Builder
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")
			callback(line)
		}
	}()

	// 等待命令执行完成
	err = session.Wait()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 记录命令执行结束的时间和耗时
	executionEndTime := time.Now()
	executionDuration := executionEndTime.Sub(executionStartTime)

	// 构建完整的日志记录
	logOutput := fmt.Sprintf("=== SSH命令执行日志 ===\n")
	logOutput += fmt.Sprintf("命令: %s\n", cmd)
	logOutput += fmt.Sprintf("开始时间: %s\n", executionStartTime.Format("2006-01-02 15:04:05"))
	logOutput += fmt.Sprintf("结束时间: %s\n", executionEndTime.Format("2006-01-02 15:04:05"))
	logOutput += fmt.Sprintf("执行耗时: %v\n", executionDuration)
	logOutput += fmt.Sprintf("\n=== 标准输出 ===\n%s\n", stdout)
	logOutput += fmt.Sprintf("=== 标准错误 ===\n%s\n", stderr)

	// 打印完整日志到控制台
	fmt.Println(logOutput)

	// 构建命令执行结束的日志
	status := "success"
	if err != nil {
		status = "failed"
	}

	endLogEntry := log.LogEntry{
		ID:        startLogEntry.ID,
		NodeID:    c.nodeID,
		NodeName:  c.nodeName,
		Operation: "SSHCommandExecution",
		Command:   cmd,
		Output:    logOutput,
		Status:    status,
		CreatedAt: executionStartTime,
		UpdatedAt: executionEndTime,
	}

	// 将结束日志写入日志管理系统
	if c.logManager != nil {
		c.logManager.CreateLog(endLogEntry)
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout, fmt.Errorf("command timed out after 60 minutes: %s\nStdout: %s\nStderr: %s", cmd, stdout, stderr)
		}
		// 区分不同类型的错误
		if exitErr, ok := err.(*ssh.ExitError); ok {
			// 检查是否是信号中断
			if exitErr.Signal() == "TERM" {
				return stdout, fmt.Errorf("command was terminated by signal SIGTERM after 60 minutes: %s\nStdout: %s\nStderr: %s", cmd, stdout, stderr)
			}
			return stdout, fmt.Errorf("command failed with exit code %d: %s\nStdout: %s\nStderr: %s", exitErr.ExitStatus(), cmd, stdout, stderr)
		}
		return stdout, fmt.Errorf("command failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	return stdout, nil
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
