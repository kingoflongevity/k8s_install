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
	Etcd                 EtcdConfiguration    `json:"etcd,omitempty"`
	Addons               []AddonConfiguration `json:"addons,omitempty"`
}

// InitConfiguration 定义初始化配置
type InitConfiguration struct {
	LocalAPIEndpoint LocalAPIEndpoint `json:"localAPIEndpoint"`
	NodeRegistration NodeRegistration `json:"nodeRegistration"`
}

// LocalAPIEndpoint 定义本地API端点
type LocalAPIEndpoint struct {
	AdvertiseAddress string `json:"advertiseAddress"`
	BindPort         int    `json:"bindPort"`
}

// NodeRegistration 定义节点注册配置
type NodeRegistration struct {
	Name      string  `json:"name,omitempty"`
	CRISocket string  `json:"criSocket,omitempty"`
	Taints    []Taint `json:"taints,omitempty"`
}

// Taint 定义节点污点
type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect"`
}

// ClusterConfiguration 定义集群配置
type ClusterConfiguration struct {
	KubernetesVersion    string                 `json:"kubernetesVersion"`
	ControlPlaneEndpoint string                 `json:"controlPlaneEndpoint,omitempty"`
	Networking           Networking             `json:"networking"`
	API                  APIConfiguration       `json:"api"`
	ControllerManager    ComponentConfiguration `json:"controllerManager"`
	Scheduler            ComponentConfiguration `json:"scheduler"`
}

// APIConfiguration 定义API服务器配置
type APIConfiguration struct {
	TimeoutForControlPlane int `json:"timeoutForControlPlane,omitempty"`
}

// ComponentConfiguration 定义组件配置
type ComponentConfiguration struct {
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// Networking 定义网络配置
type Networking struct {
	PodSubnet            string `json:"podSubnet"`
	ServiceSubnet        string `json:"serviceSubnet"`
	DNSDomain            string `json:"dnsDomain"`
	ServiceNodePortRange string `json:"serviceNodePortRange,omitempty"`
}

// EtcdConfiguration 定义Etcd配置
type EtcdConfiguration struct {
	Local    LocalEtcdConfiguration    `json:"local,omitempty"`
	External ExternalEtcdConfiguration `json:"external,omitempty"`
}

// LocalEtcdConfiguration 定义本地Etcd配置
type LocalEtcdConfiguration struct {
	DataDir   string            `json:"dataDir"`
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// ExternalEtcdConfiguration 定义外部Etcd配置
type ExternalEtcdConfiguration struct {
	Endpoints  []string `json:"endpoints"`
	CACertFile string   `json:"caFile"`
	CertFile   string   `json:"certFile"`
	KeyFile    string   `json:"keyFile"`
}

// HarborConfig 定义Harbor仓库配置
type HarborConfig struct {
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Project  string `json:"project"`
	SkipTLS  bool   `json:"skipTls"`
}

// AddonConfiguration 定义附加组件配置
type AddonConfiguration struct {
	Name    string                 `json:"name"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config,omitempty"`
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
	// 确保配置有合理的默认值
	if config.APIVersion == "" {
		config.APIVersion = "kubeadm.k8s.io/v1beta3"
	}
	if config.Kind == "" {
		config.Kind = "InitConfiguration"
	}
	if config.ClusterConfiguration.KubernetesVersion == "" {
		config.ClusterConfiguration.KubernetesVersion = "stable-1"
	}
	if config.ClusterConfiguration.Networking.PodSubnet == "" {
		config.ClusterConfiguration.Networking.PodSubnet = "10.244.0.0/16"
	}
	if config.ClusterConfiguration.Networking.ServiceSubnet == "" {
		config.ClusterConfiguration.Networking.ServiceSubnet = "10.96.0.0/12"
	}
	if config.ClusterConfiguration.Networking.DNSDomain == "" {
		config.ClusterConfiguration.Networking.DNSDomain = "cluster.local"
	}
	if config.Etcd.Local.DataDir == "" {
		config.Etcd.Local.DataDir = "/var/lib/etcd"
	}
	if config.InitConfiguration.NodeRegistration.CRISocket == "" {
		config.InitConfiguration.NodeRegistration.CRISocket = "/run/containerd/containerd.sock"
	}

	// 生成kubeadm配置文件内容
	configContent := fmt.Sprintf(`apiVersion: %s
kind: %s
initConfiguration:
  localAPIEndpoint:
    advertiseAddress: %s
    bindPort: %d
  nodeRegistration:
    name: %s
    criSocket: %s
    taints: %s
clusterConfiguration:
  kubernetesVersion: %s
  controlPlaneEndpoint: %s
  networking:
    podSubnet: %s
    serviceSubnet: %s
    dnsDomain: %s
    serviceNodePortRange: %s
  api:
    timeoutForControlPlane: %d
  controllerManager:
    extraArgs: %s
  scheduler:
    extraArgs: %s
  etcd:
    local:
      dataDir: %s
      extraArgs: %s
`,
		config.APIVersion,
		config.Kind,
		config.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress,
		config.InitConfiguration.LocalAPIEndpoint.BindPort,
		config.InitConfiguration.NodeRegistration.Name,
		config.InitConfiguration.NodeRegistration.CRISocket,
		formatTaints(config.InitConfiguration.NodeRegistration.Taints),
		config.ClusterConfiguration.KubernetesVersion,
		config.ClusterConfiguration.ControlPlaneEndpoint,
		config.ClusterConfiguration.Networking.PodSubnet,
		config.ClusterConfiguration.Networking.ServiceSubnet,
		config.ClusterConfiguration.Networking.DNSDomain,
		config.ClusterConfiguration.Networking.ServiceNodePortRange,
		config.ClusterConfiguration.API.TimeoutForControlPlane,
		formatExtraArgs(config.ClusterConfiguration.ControllerManager.ExtraArgs),
		formatExtraArgs(config.ClusterConfiguration.Scheduler.ExtraArgs),
		config.Etcd.Local.DataDir,
		formatExtraArgs(config.Etcd.Local.ExtraArgs),
	)

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

	// 执行kubeadm init，添加生产环境推荐参数
	result, err := RunCommand("kubeadm", "init", "--config", tmpFile, "--upload-certs")
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("kubeadm命令未找到，请确保kubeadm已安装且在PATH环境变量中")
		}
		return "", err
	}

	return result, nil
}

// formatTaints 格式化污点配置
func formatTaints(taints []Taint) string {
	if len(taints) == 0 {
		return "[]"
	}

	result := "["
	for i, taint := range taints {
		result += fmt.Sprintf(`{\"effect\":\"%s\",\"key\":\"%s\",\"value\":\"%s\"}`, taint.Effect, taint.Key, taint.Value)
		if i < len(taints)-1 {
			result += ", "
		}
	}
	result += "]"
	return result
}

// formatExtraArgs 格式化额外参数配置
func formatExtraArgs(args map[string]string) string {
	if args == nil || len(args) == 0 {
		return "{}"
	}

	result := "{"
	first := true
	for key, value := range args {
		if !first {
			result += ", "
		}
		result += fmt.Sprintf(`\"%s\":\"%s\"`, key, value)
		first = false
	}
	result += "}"
	return result
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

// PullKubernetesImages 拉取Kubernetes所需的镜像到本地
func PullKubernetesImages(version string) (string, error) {
	// 执行kubeadm config images pull命令拉取所有必要的镜像
	result, err := RunCommand("kubeadm", "config", "images", "pull", "--kubernetes-version", version)
	if err != nil {
		return "", fmt.Errorf("拉取Kubernetes镜像失败: %v", err)
	}
	return result, nil
}

// PushImagesToHarbor 将本地Kubernetes镜像推送到Harbor仓库
func PushImagesToHarbor(harborConfig HarborConfig, kubernetesVersion string) (string, error) {
	if !harborConfig.Enabled {
		return "", fmt.Errorf("Harbor配置未启用")
	}

	// 1. 登录Harbor仓库
	loginCmd := fmt.Sprintf("docker login %s -u %s -p %s", harborConfig.URL, harborConfig.Username, harborConfig.Password)
	if harborConfig.SkipTLS {
		loginCmd += " --tlsverify=false"
	}
	_, err := RunCommand("bash", "-c", loginCmd)
	if err != nil {
		return "", fmt.Errorf("登录Harbor仓库失败: %v", err)
	}

	// 2. 获取Kubernetes所需的镜像列表
	imagesCmd := fmt.Sprintf("kubeadm config images list --kubernetes-version %s", kubernetesVersion)
	imagesList, err := RunCommand("bash", "-c", imagesCmd)
	if err != nil {
		return "", fmt.Errorf("获取Kubernetes镜像列表失败: %v", err)
	}

	// 3. 处理每个镜像，标签并推送
	var result strings.Builder
	images := strings.Split(imagesList, "\n")
	for _, image := range images {
		image = strings.TrimSpace(image)
		if image == "" {
			continue
		}

		// 获取镜像名称和标签
		imageName := image[strings.LastIndex(image, "/")+1:]
		if !strings.Contains(imageName, ":") {
			imageName += ":latest"
		}

		// 构建Harbor镜像标签
		harborImage := fmt.Sprintf("%s/%s/%s", harborConfig.URL, harborConfig.Project, imageName)

		// 标签镜像
		tagCmd := fmt.Sprintf("docker tag %s %s", image, harborImage)
		_, err := RunCommand("bash", "-c", tagCmd)
		if err != nil {
			result.WriteString(fmt.Sprintf("标签镜像 %s 失败: %v\n", image, err))
			continue
		}

		// 推送镜像到Harbor
		pushCmd := fmt.Sprintf("docker push %s", harborImage)
		if harborConfig.SkipTLS {
			pushCmd += " --tlsverify=false"
		}
		pushResult, err := RunCommand("bash", "-c", pushCmd)
		if err != nil {
			result.WriteString(fmt.Sprintf("推送镜像 %s 失败: %v\n", harborImage, err))
			continue
		}

		result.WriteString(fmt.Sprintf("成功推送镜像 %s 到 %s\n", image, harborConfig.URL))
		result.WriteString(pushResult + "\n")
	}

	return result.String(), nil
}

// DownloadKubeadmPackage 下载指定版本的kubeadm包及其相关包
func DownloadKubeadmPackage(version, arch, distro string, sourceURL string) (string, error) {
	// 如果没有指定源，使用默认源
	if sourceURL == "" {
		source := GetDefaultSource()
		sourceURL = source.URL
	}

	// 下载所有相关的Kubernetes包
	packages := []string{"kubeadm", "kubelet", "kubectl"}

	for _, packageName := range packages {
		// 检查包是否已存在
		if CheckPackageExists(packageName, version, arch, distro) {
			fmt.Printf("Package %s-%s-%s-%s already exists, skipping download\n", packageName, version, arch, distro)
			continue
		}

		// 构建下载URL
		downloadURL := fmt.Sprintf("%s/release/%s/bin/linux/%s/%s", sourceURL, version, arch, packageName)

		// 获取本地存储路径
		destPath := GetPackagePath(packageName, version, arch, distro)
		if destPath == "" {
			return "", fmt.Errorf("failed to get package path for %s", packageName)
		}

		// 下载文件
		fmt.Printf("Downloading %s from %s to %s\n", packageName, downloadURL, destPath)
		file, err := os.Create(destPath)
		if err != nil {
			return "", fmt.Errorf("failed to create file %s: %v", destPath, err)
		}

		// 发送HTTP请求
		resp, err := http.Get(downloadURL)
		if err != nil {
			file.Close()
			os.Remove(destPath) // 清理失败的下载
			return "", fmt.Errorf("failed to download %s: %v", downloadURL, err)
		}

		// 检查响应状态
		if resp.StatusCode != http.StatusOK {
			file.Close()
			resp.Body.Close()
			os.Remove(destPath) // 清理失败的下载
			return "", fmt.Errorf("failed to download %s: HTTP %d", downloadURL, resp.StatusCode)
		}

		// 获取文件总大小
		totalSize := resp.ContentLength
		var downloaded int64

		// 创建一个缓冲区
		buffer := make([]byte, 4096)

		// 循环读取数据并写入文件，同时显示进度
		for {
			n, err := resp.Body.Read(buffer)
			if err != nil && err != io.EOF {
				file.Close()
				resp.Body.Close()
				os.Remove(destPath) // 清理失败的下载
				return "", fmt.Errorf("failed to read response body: %v", err)
			}

			if n == 0 {
				break
			}

			// 写入文件
			if _, err := file.Write(buffer[:n]); err != nil {
				file.Close()
				resp.Body.Close()
				os.Remove(destPath) // 清理失败的下载
				return "", fmt.Errorf("failed to write package to file: %v", err)
			}

			// 更新已下载大小并显示进度
			downloaded += int64(n)
			if totalSize > 0 {
				percent := float64(downloaded) / float64(totalSize) * 100
				fmt.Printf("\rDownloading %s: %.1f%% (%d/%d bytes)", packageName, percent, downloaded, totalSize)
			}
		}
		fmt.Printf("\n") // 换行，避免进度信息影响后续输出

		// 关闭文件和响应
		file.Close()
		resp.Body.Close()

		// 设置文件可执行权限
		if err := os.Chmod(destPath, 0755); err != nil {
			fmt.Printf("Warning: Failed to set executable permission on %s: %v\n", destPath, err)
		}
	}

	// 返回kubeadm包的路径
	return GetPackagePath("kubeadm", version, arch, distro), nil
}

// DeployKubeadmPackage 部署kubeadm包到远程节点
func DeployKubeadmPackage(packagePath, nodeIP, username, password string, port int, privateKey string) error {
	// 从包路径中提取版本、架构和发行版信息
	filename := filepath.Base(packagePath)
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		return fmt.Errorf("invalid package path format: %s", packagePath)
	}
	version := parts[1]
	arch := parts[2]
	distro := parts[3]

	// 所有需要部署的Kubernetes包
	packages := []string{"kubeadm", "kubelet", "kubectl"}

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

	// 部署所有包
	for _, packageName := range packages {
		// 获取包的本地路径
		localPackagePath := GetPackagePath(packageName, version, arch, distro)
		if localPackagePath == "" {
			return fmt.Errorf("failed to get package path for %s", packageName)
		}

		// 检查包是否存在
		if _, err := os.Stat(localPackagePath); os.IsNotExist(err) {
			return fmt.Errorf("package %s-%s-%s-%s not found at %s", packageName, version, arch, distro, localPackagePath)
		}

		// 上传包到远程节点
		remotePath := fmt.Sprintf("/tmp/%s", filepath.Base(localPackagePath))
		fmt.Printf("Uploading %s to %s:%s\n", localPackagePath, nodeIP, remotePath)
		if err := client.UploadFile(localPackagePath, remotePath); err != nil {
			return fmt.Errorf("failed to upload %s to %s: %v", packageName, nodeIP, err)
		}

		// 在远程节点上安装包
		// 注意：这里我们直接复制二进制文件到/usr/bin目录，而不是使用包管理器
		installCmd := fmt.Sprintf("sudo mkdir -p /usr/bin && sudo cp %s /usr/bin/%s && sudo chmod +x /usr/bin/%s", remotePath, packageName, packageName)
		fmt.Printf("Installing %s on %s\n", packageName, nodeIP)
		_, err = client.RunCommand(installCmd)
		if err != nil {
			return fmt.Errorf("failed to install %s on %s: %v", packageName, nodeIP, err)
		}

		// 清理远程临时文件
		cleanupCmd := fmt.Sprintf("rm -f %s", remotePath)
		if _, err := client.RunCommand(cleanupCmd); err != nil {
			fmt.Printf("Warning: Failed to clean up remote file %s: %v\n", remotePath, err)
		}
	}

	// 验证安装
	versionCmd := "kubeadm version --short"
	versionOutput, err := client.RunCommand(versionCmd)
	if err != nil {
		return fmt.Errorf("failed to verify kubeadm installation on %s: %v", nodeIP, err)
	}

	fmt.Printf("Successfully installed Kubernetes packages on %s: %s\n", nodeIP, strings.TrimSpace(versionOutput))

	return nil
}
