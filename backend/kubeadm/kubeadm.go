package kubeadm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"k8s-installer/ssh"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// KubeadmConfig 定义Kubeadm配置
type KubeadmConfig struct {
	APIVersion           string               `json:"apiVersion"`
	Kind                 string               `json:"kind"`
	InitConfiguration    InitConfiguration    `json:"initConfiguration"`
	ClusterConfiguration ClusterConfiguration `json:"clusterConfiguration"`
}

// InitConfiguration 定义初始化配置
type InitConfiguration struct {
	LocalAPIEndpoint LocalAPIEndpoint `json:"localAPIEndpoint"`
}

// LocalAPIEndpoint 定义本地API端点
type LocalAPIEndpoint struct {
	AdvertiseAddress string `json:"advertiseAddress"`
	BindPort         int    `json:"bindPort"`
}

// ClusterConfiguration 定义集群配置
type ClusterConfiguration struct {
	KubernetesVersion    string     `json:"kubernetesVersion"`
	ControlPlaneEndpoint string     `json:"controlPlaneEndpoint,omitempty"`
	Networking           Networking `json:"networking"`
}

// Networking 定义网络配置
type Networking struct {
	PodSubnet     string `json:"podSubnet"`
	ServiceSubnet string `json:"serviceSubnet"`
	DNSDomain     string `json:"dnsDomain"`
}

// RunCommand 执行命令并返回结果
func RunCommand(cmd string, args ...string) (string, error) {
	// 设置命令执行超时（默认300秒）
	timeout := 300 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 使用CommandContext创建带有上下文的命令
	c := exec.CommandContext(ctx, cmd, args...)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	if err != nil {
		// 检查是否超时
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %v: %s %v", timeout, cmd, args)
		}
		// 收集详细的错误信息
		return "", fmt.Errorf("command failed: %v\nCommand: %s %v\nStderr: %s\nStdout: %s",
			err, cmd, args, stderr.String(), stdout.String())
	}

	return stdout.String(), nil
}

// InitMaster 初始化K8s主节点
func InitMaster(config KubeadmConfig) (string, error) {
	// 生成kubeadm配置文件内容
	configContent := fmt.Sprintf(`apiVersion: %s
kind: %s
initConfiguration:
  localAPIEndpoint:
    advertiseAddress: %s
    bindPort: %d
clusterConfiguration:
  kubernetesVersion: %s
  networking:
    podSubnet: %s
    serviceSubnet: %s
    dnsDomain: %s
`, config.APIVersion, config.Kind, config.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress, config.InitConfiguration.LocalAPIEndpoint.BindPort, config.ClusterConfiguration.KubernetesVersion, config.ClusterConfiguration.Networking.PodSubnet, config.ClusterConfiguration.Networking.ServiceSubnet, config.ClusterConfiguration.Networking.DNSDomain)

	// 写入临时配置文件
	tmpFile := "/tmp/kubeadm-config.yaml"
	// 使用Go标准库创建文件，而不是依赖外部命令
	f, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temp config file: %v", err)
	}
	defer f.Close()

	_, err = f.WriteString(configContent)
	if err != nil {
		return "", fmt.Errorf("failed to write to temp config file: %v", err)
	}

	// 执行kubeadm init
	result, err := RunCommand("kubeadm", "init", "--config", tmpFile)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("kubeadm命令未找到，请确保kubeadm已安装且在PATH环境变量中")
		}
		return "", err
	}

	return result, nil
}

// JoinWorker 加入工作节点
func JoinWorker(token, caCertHash, controlPlaneEndpoint string) (string, error) {
	result, err := RunCommand("kubeadm", "join", controlPlaneEndpoint, "--token", token, "--discovery-token-ca-cert-hash", caCertHash)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("kubeadm命令未找到，请确保kubeadm已安装且在PATH环境变量中")
		}
		return "", err
	}

	return result, nil
}

// GetJoinCommand 获取加入命令
func GetJoinCommand() (string, error) {
	result, err := RunCommand("kubeadm", "token", "create", "--print-join-command")
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("kubeadm命令未找到，请确保kubeadm已安装且在PATH环境变量中")
		}
		return "", err
	}

	return strings.TrimSpace(result), nil
}

// ResetCluster 重置集群
func ResetCluster() (string, error) {
	result, err := RunCommand("kubeadm", "reset", "--force")
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("kubeadm命令未找到，请确保kubeadm已安装且在PATH环境变量中")
		}
		return "", err
	}

	return result, nil
}

// CheckKubeadmVersion 检查kubeadm版本
func CheckKubeadmVersion() (string, error) {
	result, err := RunCommand("kubeadm", "version", "--short")
	if err != nil {
		// 检查是否是kubeadm命令不存在的错误
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("kubeadm命令未找到，请确保kubeadm已安装且在PATH环境变量中")
		}
		return "", err
	}

	return strings.TrimSpace(result), nil
}

// DownloadKubeadmPackage 下载指定版本的kubeadm包
func DownloadKubeadmPackage(version, arch, distro string) (string, error) {
	// 构建下载URL
	// 这里假设使用kubernetes官方的包仓库，实际可能需要根据不同发行版调整
	baseURL := fmt.Sprintf("https://packages.cloud.google.com/apt/pool/main/k/kubeadm/")
	packageName := fmt.Sprintf("kubeadm_%s-00_amd64.deb", version)
	downloadURL := baseURL + packageName

	// 创建下载目录
	downloadDir := "/tmp/kubeadm-packages"
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create download directory: %v", err)
	}

	// 下载文件
	destPath := filepath.Join(downloadDir, packageName)
	file, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// 发送HTTP请求
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download package: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download package: HTTP %d", resp.StatusCode)
	}

	// 写入文件
	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write package to file: %v", err)
	}

	return destPath, nil
}

// DeployKubeadmPackage 部署kubeadm包到远程节点
func DeployKubeadmPackage(packagePath, nodeIP, username, password string, port int, privateKey string) error {
	// 创建SSH配置
	sshConfig := ssh.SSHConfig{
		Host:       nodeIP,
		Port:       port,
		Username:   username,
		Password:   password,
		PrivateKey: privateKey,
	}

	// 连接到远程节点
	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", nodeIP, err)
	}
	defer client.Close()

	// 上传kubeadm包到远程节点
	remotePath := "/tmp/" + filepath.Base(packagePath)
	if err := client.UploadFile(packagePath, remotePath); err != nil {
		return fmt.Errorf("failed to upload package to %s: %v", nodeIP, err)
	}

	// 安装kubeadm包
	var installCmd string
	if strings.HasSuffix(packagePath, ".deb") {
		// Debian/Ubuntu 系统
		installCmd = fmt.Sprintf("sudo dpkg -i %s", remotePath)
	} else if strings.HasSuffix(packagePath, ".rpm") {
		// RHEL/CentOS 系统
		installCmd = fmt.Sprintf("sudo rpm -ivh %s", remotePath)
	} else {
		return fmt.Errorf("unsupported package format: %s", packagePath)
	}

	// 执行安装命令
	_, err = client.RunCommand(installCmd)
	if err != nil {
		return fmt.Errorf("failed to install kubeadm package on %s: %v", nodeIP, err)
	}

	// 验证安装
	versionCmd := "kubeadm version --short"
	versionOutput, err := client.RunCommand(versionCmd)
	if err != nil {
		return fmt.Errorf("failed to verify kubeadm installation on %s: %v", nodeIP, err)
	}

	fmt.Printf("Successfully installed kubeadm on %s: %s\n", nodeIP, strings.TrimSpace(versionOutput))

	return nil
}
