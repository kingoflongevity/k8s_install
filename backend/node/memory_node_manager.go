package node

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s-installer/log"
	"k8s-installer/ssh"
)

// MemoryNodeManager 内存节点管理器
type MemoryNodeManager struct {
	nodes         map[string]Node
	mutex         sync.RWMutex
	scriptManager interface{} // 脚本管理器接口
}

// NewMemoryNodeManager 创建新的内存节点管理器
func NewMemoryNodeManager() *MemoryNodeManager {
	return &MemoryNodeManager{
		nodes: make(map[string]Node),
	}
}

// GetNodes 获取所有节点
func (m *MemoryNodeManager) GetNodes() ([]Node, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var nodes []Node
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetNode 根据ID获取节点
func (m *MemoryNodeManager) GetNode(id string) (*Node, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	node, exists := m.nodes[id]
	if !exists {
		return nil, errors.New("node not found")
	}

	return &node, nil
}

// CreateNode 创建新节点
func (m *MemoryNodeManager) CreateNode(node Node) (*Node, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 生成ID
	if node.ID == "" {
		node.ID = time.Now().Format("20060102150405")
	}

	// 设置默认值
	if node.Port == 0 {
		node.Port = 22
	}

	if node.NodeType == "" {
		node.NodeType = NodeTypeWorker
	}

	if node.Status == "" {
		node.Status = NodeStatusOffline
	}

	node.CreatedAt = time.Now()
	node.UpdatedAt = time.Now()

	// 保存节点
	m.nodes[node.ID] = node

	return &node, nil
}

// UpdateNode 更新节点信息
func (m *MemoryNodeManager) UpdateNode(id string, node Node) (*Node, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查节点是否存在
	_, exists := m.nodes[id]
	if !exists {
		return nil, errors.New("node not found")
	}

	// 更新节点信息
	node.ID = id
	node.UpdatedAt = time.Now()
	m.nodes[id] = node

	return &node, nil
}

// DeleteNode 删除节点
func (m *MemoryNodeManager) DeleteNode(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查节点是否存在
	_, exists := m.nodes[id]
	if !exists {
		return errors.New("node not found")
	}

	// 删除节点
	delete(m.nodes, id)

	return nil
}

// SetScriptManager 设置脚本管理器
func (m *MemoryNodeManager) SetScriptManager(scriptManager interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.scriptManager = scriptManager
	return nil
}

// TestConnection 测试节点连接
func (m *MemoryNodeManager) TestConnection(id string) (bool, error) {
	m.mutex.RLock()
	node, exists := m.nodes[id]
	m.mutex.RUnlock()

	if !exists {
		return false, errors.New("node not found")
	}

	// 测试SSH连接
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		// 更新节点状态为离线
		m.mutex.Lock()
		node.Status = NodeStatusOffline
		node.UpdatedAt = time.Now()
		m.nodes[id] = node
		m.mutex.Unlock()

		return false, err
	}
	defer client.Close()

	// 检测操作系统类型
	distroCmd := `
if [ -f /etc/os-release ]; then
	. /etc/os-release
	echo "$ID"
else
	# 尝试获取其他发行版信息
	if [ -f /etc/centos-release ]; then
		echo "centos"
	elif [ -f /etc/redhat-release ]; then
		echo "rhel"
	else
		echo "unknown"
	fi
fi
`
	distroOutput, err := client.RunCommand(distroCmd)
	osType := "unknown"
	if err == nil {
		osType = strings.TrimSpace(distroOutput)
	}

	// 更新节点状态为在线并保存操作系统类型
	m.mutex.Lock()
	node.Status = NodeStatusOnline
	node.OS = osType
	node.UpdatedAt = time.Now()
	m.nodes[id] = node
	m.mutex.Unlock()

	return true, nil
}

// InstallKubernetesComponents 安装Kubernetes组件
func (m *MemoryNodeManager) InstallKubernetesComponents(id string, kubeadmVersion string) error {
	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 测试连接
	connected, err := m.TestConnection(id)
	if !connected {
		return err
	}

	// 执行安装逻辑
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	// 1. 检测操作系统类型和版本
	distroCmd := `
if [ -f /etc/os-release ]; then
	. /etc/os-release
	echo "$ID $VERSION_ID"
else
	# 尝试获取其他发行版信息
	if [ -f /etc/centos-release ]; then
		echo "centos $(grep -o '[0-9]\+\.[0-9]\+' /etc/centos-release)"
	elif [ -f /etc/redhat-release ]; then
		echo "rhel $(grep -o '[0-9]\+\.[0-9]\+' /etc/redhat-release)"
	else
		echo "unknown 0.0"
	fi
fi
`
	distroOutput, err := client.RunCommand(distroCmd)
	if err != nil {
		return err
	}
	distroInfo := strings.TrimSpace(distroOutput)
	distroParts := strings.Split(distroInfo, " ")
	distro := distroParts[0]

	// 2. 安装Kubernetes组件
	var cmd string
	switch distro {
	case "ubuntu", "debian":
		cmd = `
	apt-get update && apt-get install -y apt-transport-https ca-certificates curl gpg
	
	# 创建keyring目录
	mkdir -p -m 755 /etc/apt/keyrings
	
	# 下载并安装GPG密钥
	curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
	chmod 644 /etc/apt/keyrings/kubernetes-apt-keyring.gpg
	
	# 添加Kubernetes repo
	echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list
	chmod 644 /etc/apt/sources.list.d/kubernetes.list
	
	apt-get update
	apt-get install -y kubelet kubeadm kubectl
	apt-mark hold kubelet kubeadm kubectl
	
	# 生产环境优化：配置kubelet使用systemd cgroup驱动
	cat <<EOF | tee /etc/default/kubelet
	KUBELET_EXTRA_ARGS="--cgroup-driver=systemd --runtime-cgroups=/system.slice/containerd.service --kubelet-cgroups=/system.slice/kubelet.service"
	EOF
	
	systemctl daemon-reload
	systemctl restart kubelet
	systemctl enable kubelet
	`
	case "centos", "rhel", "rocky", "almalinux":
		cmd = `
	# 检查包管理器类型
	if command -v dnf &> /dev/null; then
		PKG_MGR="dnf"
		PKG_MGR_UTILS="dnf-utils"
	else
		PKG_MGR="yum"
		PKG_MGR_UTILS="yum-utils"
	fi
	
	# 更彻底地移除旧的Kubernetes repo配置
	rm -f /etc/yum.repos.d/kubernetes.repo
	rm -f /etc/yum.repos.d/packages.cloud.google.com_yum_repos_kubernetes*.repo
	
	# 移除所有可能的旧repo
	for repo in $(sudo $PKG_MGR config-manager --list-repos 2>/dev/null | grep -i kubernetes | awk '{print $1}'); do
		sudo $PKG_MGR config-manager --remove-repo "$repo" 2>/dev/null || true
	done
	
	# 清理所有包含google或kubernetes的repo文件
	rm -f /etc/yum.repos.d/*google*.repo
	rm -f /etc/yum.repos.d/*kubernetes*.repo
	
	# 使用正确的Kubernetes repo URL，使用固定版本v1.30确保兼容性
	cat <<'EOF' | tee /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/v1.30/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/v1.30/rpm/repodata/repomd.xml.key
EOF
	
	# 清理并更新repo缓存
	sudo $PKG_MGR clean all
	sudo $PKG_MGR makecache
	
	# 安装必要的工具
	sudo $PKG_MGR install -y $PKG_MGR_UTILS
	
	# 安装Kubernetes组件
	sudo $PKG_MGR install -y kubelet kubeadm kubectl
	
	# 只在kubeadm安装成功后继续
	if ! command -v kubeadm &> /dev/null; then
		echo "ERROR: kubeadm command not found, installation failed!"
		exit 1
	fi
	
	# 安装dnf versionlock插件（如果使用dnf）并锁定版本
	if [ "$PKG_MGR" = "dnf" ]; then
		sudo $PKG_MGR install -y dnf-plugin-versionlock 2>/dev/null || true
		sudo dnf versionlock add kubelet kubeadm kubectl 2>/dev/null || true
	else
		sudo yum versionlock add kubelet kubeadm kubectl 2>/dev/null || true
	fi
	
	# 禁用SELINUX（生产环境推荐）
	sudo setenforce 0 2>/dev/null || true
	sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config 2>/dev/null || true
	
	# 生产环境优化：配置kubelet使用systemd cgroup驱动
	sudo mkdir -p /etc/sysconfig
	cat <<EOF | sudo tee /etc/sysconfig/kubelet
KUBELET_EXTRA_ARGS="--cgroup-driver=systemd --runtime-cgroups=/system.slice/containerd.service --kubelet-cgroups=/system.slice/kubelet.service"
EOF
	
	# 检查并启动kubelet服务
	sudo systemctl daemon-reload
	if systemctl list-unit-files | grep -q kubelet.service; then
		sudo systemctl restart kubelet
		sudo systemctl enable kubelet
		echo "kubelet.service enabled and started successfully"
	else
		# 尝试手动创建kubelet服务文件或使用其他方式启动
		echo "Warning: kubelet.service unit file not found, trying to start kubelet directly..."
		sudo kubelet --version
	fi
	
	# 最后验证kubeadm安装是否成功
	if ! command -v kubeadm &> /dev/null; then
		echo "ERROR: kubeadm command not found, installation failed!"
		exit 1
	fi
	
	echo "Kubernetes components installed successfully"
	kubeadm version
	`
	default:
		return fmt.Errorf("unsupported distribution: %s", distro)
	}

	// 执行安装命令
	_, err = client.RunCommand(cmd)
	return err
}

// deployMasterNode 部署主节点
func (m *MemoryNodeManager) deployMasterNode(client *ssh.SSHClient) error {
	// 1. 检测操作系统类型
	distroCmd := `
if [ -f /etc/os-release ]; then
	. /etc/os-release
	echo $ID
fi
`
	distroOutput, err := client.RunCommand(distroCmd)
	if err != nil {
		return err
	}
	distro := strings.TrimSpace(distroOutput)

	// 2. 设置容器运行时（默认使用containerd，生产环境推荐）
	containerRuntime := "containerd"
	if err := m.installContainerRuntime(client, distro, containerRuntime, ""); err != nil {
		return err
	}

	// 3. 禁用swap
	swapCmd := `
swapoff -a
sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
`
	if _, err := client.RunCommand(swapCmd); err != nil {
		return err
	}

	// 4. 设置内核参数（生产环境推荐配置）
	kernelCmd := `
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
vm.swappiness                       = 0
vm.overcommit_memory                = 1
kernel.panic                        = 10
kernel.panic_on_oops                = 1
EOF
sysctl --system
modprobe br_netfilter
modprobe overlay
`
	if _, err := client.RunCommand(kernelCmd); err != nil {
		return err
	}

	return nil
}

// installContainerRuntime 安装容器运行时
func (m *MemoryNodeManager) installContainerRuntime(client *ssh.SSHClient, distro, runtime, version string) error {
	var cmd string
	switch distro {
	case "ubuntu", "debian":
		// 只支持containerd
		cmd = `
		apt-get update && apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release
		mkdir -p /etc/apt/keyrings
		curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
		echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
		apt-get update && apt-get install -y containerd.io
		mkdir -p /etc/containerd
		containerd config default | tee /etc/containerd/config.toml
		# 生产环境优化：设置SystemdCgroup、 sandbox_image、默认运行时等
		sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
		sed -i 's/sandbox_image = "registry.k8s.io\/pause:3.6"/sandbox_image = "registry.k8s.io\/pause:3.9"/g' /etc/containerd/config.toml
		sed -i '/\[plugins."io.containerd.grpc.v1.cri\.containerd.runtimes.runc.options"\]/a\          SystemdCgroup = true' /etc/containerd/config.toml
		# 配置镜像加速
		sed -i '/\[plugins."io.containerd.grpc.v1.cri"\]/a\  sandbox_image = "registry.k8s.io\/pause:3.9"' /etc/containerd/config.toml
		sed -i '/\[plugins."io.containerd.grpc.v1.cri"\]/a\  systemd_cgroup = true' /etc/containerd/config.toml
		systemctl restart containerd
		systemctl enable containerd
		`
	case "centos", "rhel", "rocky", "alma":
		// 只支持containerd
		cmd = `
		yum install -y yum-utils
		yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
		yum install -y containerd.io
		mkdir -p /etc/containerd
		containerd config default | tee /etc/containerd/config.toml
		# 生产环境优化：设置SystemdCgroup、 sandbox_image、默认运行时等
		sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
		sed -i 's/sandbox_image = "registry.k8s.io\/pause:3.6"/sandbox_image = "registry.k8s.io\/pause:3.9"/g' /etc/containerd/config.toml
		sed -i '/\[plugins."io.containerd.grpc.v1.cri\.containerd.runtimes.runc.options"\]/a\          SystemdCgroup = true' /etc/containerd/config.toml
		# 配置镜像加速
		sed -i '/\[plugins."io.containerd.grpc.v1.cri"\]/a\  sandbox_image = "registry.k8s.io\/pause:3.9"' /etc/containerd/config.toml
		sed -i '/\[plugins."io.containerd.grpc.v1.cri"\]/a\  systemd_cgroup = true' /etc/containerd/config.toml
		systemctl restart containerd
		systemctl enable containerd
		`
	default:
		return fmt.Errorf("unsupported distribution: %s", distro)
	}

	_, err := client.RunCommand(cmd)
	return err
}

// 移除重复的installKubernetesComponents函数，使用已经定义的InstallKubernetesComponents函数

// ConfigureSSHSettings 配置节点SSH设置
func (m *MemoryNodeManager) ConfigureSSHSettings(id string) error {
	m.mutex.RLock()
	node, exists := m.nodes[id]
	m.mutex.RUnlock()

	if !exists {
		return errors.New("node not found")
	}

	// 创建SSH客户端
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	// 1. 生成SSH密钥对
	_, err = client.RunCommand("mkdir -p ~/.ssh && chmod 700 ~/.ssh")
	if err != nil {
		return fmt.Errorf("failed to create .ssh directory: %v", err)
	}

	// 生成密钥对，不使用密码
	_, err = client.RunCommand("ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N '' -q")
	if err != nil {
		return fmt.Errorf("failed to generate SSH key: %v", err)
	}

	// 设置公钥文件权限
	_, err = client.RunCommand("chmod 644 ~/.ssh/id_rsa.pub")
	if err != nil {
		return fmt.Errorf("failed to set public key permissions: %v", err)
	}

	// 设置私钥文件权限
	_, err = client.RunCommand("chmod 600 ~/.ssh/id_rsa")
	if err != nil {
		return fmt.Errorf("failed to set private key permissions: %v", err)
	}

	// 2. 配置SSH服务，允许公钥认证
	_, err = client.RunCommand("sudo sed -i 's/^#PubkeyAuthentication yes/PubkeyAuthentication yes/' /etc/ssh/sshd_config")
	if err != nil {
		return fmt.Errorf("failed to configure SSHD: %v", err)
	}

	// 重启SSH服务
	_, err = client.RunCommand("sudo systemctl restart sshd")
	if err != nil {
		// 尝试使用service命令（兼容不同Linux发行版）
		_, err = client.RunCommand("sudo service ssh restart")
		if err != nil {
			return fmt.Errorf("failed to restart SSH service: %v", err)
		}
	}

	return nil
}

// ConfigureSSHPasswdless 配置所有节点之间的SSH免密互通
func (m *MemoryNodeManager) ConfigureSSHPasswdless() error {
	m.mutex.RLock()
	allNodes := make([]Node, 0, len(m.nodes))
	for _, node := range m.nodes {
		allNodes = append(allNodes, node)
	}
	m.mutex.RUnlock()

	if len(allNodes) < 2 {
		return fmt.Errorf("at least 2 nodes are required for SSH passwdless configuration")
	}

	// 1. 收集所有节点的公钥
	nodePublicKeys := make(map[string]string)

	for _, node := range allNodes {
		// 创建SSH客户端
		sshConfig := ssh.SSHConfig{
			Host:       node.IP,
			Port:       node.Port,
			Username:   node.Username,
			Password:   node.Password,
			PrivateKey: node.PrivateKey,
		}

		client, err := ssh.NewSSHClient(sshConfig)
		if err != nil {
			return fmt.Errorf("failed to create SSH client for node %s: %v", node.Name, err)
		}

		// 获取公钥
		publicKey, err := client.RunCommand("cat ~/.ssh/id_rsa.pub")
		if err != nil {
			// 如果公钥不存在，先配置SSH设置
			client.Close()
			if err := m.ConfigureSSHSettings(node.ID); err != nil {
				return fmt.Errorf("failed to configure SSH settings for node %s: %v", node.Name, err)
			}

			// 重新创建客户端并获取公钥
			client, err = ssh.NewSSHClient(sshConfig)
			if err != nil {
				return fmt.Errorf("failed to re-create SSH client for node %s: %v", node.Name, err)
			}

			publicKey, err = client.RunCommand("cat ~/.ssh/id_rsa.pub")
			if err != nil {
				client.Close()
				return fmt.Errorf("failed to get public key for node %s: %v", node.Name, err)
			}
		}

		nodePublicKeys[node.Name] = strings.TrimSpace(publicKey)
		client.Close()
	}

	// 2. 将所有公钥分发到每个节点的authorized_keys文件中
	for _, targetNode := range allNodes {
		// 创建SSH客户端
		sshConfig := ssh.SSHConfig{
			Host:       targetNode.IP,
			Port:       targetNode.Port,
			Username:   targetNode.Username,
			Password:   targetNode.Password,
			PrivateKey: targetNode.PrivateKey,
		}

		client, err := ssh.NewSSHClient(sshConfig)
		if err != nil {
			return fmt.Errorf("failed to create SSH client for node %s: %v", targetNode.Name, err)
		}

		// 清空authorized_keys文件
		_, err = client.RunCommand("> ~/.ssh/authorized_keys")
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to clear authorized_keys for node %s: %v", targetNode.Name, err)
		}

		// 添加所有节点的公钥到authorized_keys文件
		for nodeName, publicKey := range nodePublicKeys {
			// 添加公钥到authorized_keys文件，包括自己的
			cmd := fmt.Sprintf("echo '%s' >> ~/.ssh/authorized_keys", publicKey)
			_, err = client.RunCommand(cmd)
			if err != nil {
				client.Close()
				return fmt.Errorf("failed to add public key for node %s to %s: %v", nodeName, targetNode.Name, err)
			}
		}

		// 设置authorized_keys文件权限
		_, err = client.RunCommand("chmod 600 ~/.ssh/authorized_keys")
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to set authorized_keys permissions for node %s: %v", targetNode.Name, err)
		}

		client.Close()
	}

	// 3. 测试节点之间的免密连接
	for i, sourceNode := range allNodes {
		for j, targetNode := range allNodes {
			// 跳过自己
			if i == j {
				continue
			}

			// 创建SSH客户端
			sshConfig := ssh.SSHConfig{
				Host:       sourceNode.IP,
				Port:       sourceNode.Port,
				Username:   sourceNode.Username,
				Password:   sourceNode.Password,
				PrivateKey: sourceNode.PrivateKey,
			}

			client, err := ssh.NewSSHClient(sshConfig)
			if err != nil {
				return fmt.Errorf("failed to create SSH client for node %s: %v", sourceNode.Name, err)
			}

			// 测试免密连接
			testCmd := fmt.Sprintf("ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 %s@%s 'echo success'", targetNode.Username, targetNode.IP)
			_, err = client.RunCommand(testCmd)
			client.Close()

			if err != nil {
				return fmt.Errorf("SSH passwdless test failed from %s to %s: %v", sourceNode.Name, targetNode.Name, err)
			}
		}
	}

	return nil
}

// deployWorkerNode 部署工作节点
func (m *MemoryNodeManager) deployWorkerNode(client *ssh.SSHClient) error {
	// 1. 检测操作系统类型
	distroCmd := `
if [ -f /etc/os-release ]; then
	. /etc/os-release
	echo $ID
fi
`
	distroOutput, err := client.RunCommand(distroCmd)
	if err != nil {
		return err
	}
	distro := strings.TrimSpace(distroOutput)

	// 2. 设置容器运行时
	containerRuntime := "containerd"
	if err := m.installContainerRuntime(client, distro, containerRuntime, ""); err != nil {
		return err
	}

	// 3. 禁用swap
	swapCmd := `
swapoff -a
sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
`
	if _, err := client.RunCommand(swapCmd); err != nil {
		return err
	}

	// 4. 设置内核参数
	kernelCmd := `
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
vm.swappiness                       = 0
vm.overcommit_memory                = 1
kernel.panic                        = 10
kernel.panic_on_oops                = 1
EOF
sysctl --system
modprobe br_netfilter
modprobe overlay
`
	if _, err := client.RunCommand(kernelCmd); err != nil {
		return err
	}

	return nil
}

// DeployNode 部署节点
func (m *MemoryNodeManager) DeployNode(id string) error {
	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 测试连接
	connected, err := m.TestConnection(id)
	if !connected {
		return err
	}

	// 执行部署逻辑
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	if node.NodeType == NodeTypeMaster {
		// 部署主节点
		return m.deployMasterNode(client)
	} else {
		// 部署工作节点
		return m.deployWorkerNode(client)
	}
}

// 批量安装容器运行时
func (m *MemoryNodeManager) BatchInstallContainerRuntime(nodeIds []string, runtimeType string, version string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.InstallContainerRuntime(id, runtimeType, version); err != nil {
			results.WriteString(fmt.Sprintf("安装失败: %v\n\n", err))
		} else {
			results.WriteString("安装成功\n\n")
		}
	}
	return results.String(), nil
}

// 批量配置容器运行时
func (m *MemoryNodeManager) BatchConfigureContainerRuntime(nodeIds []string, config ContainerRuntimeConfig) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.ConfigureContainerRuntime(id, config); err != nil {
			results.WriteString(fmt.Sprintf("配置失败: %v\n\n", err))
		} else {
			results.WriteString("配置成功\n\n")
		}
	}
	return results.String(), nil
}

// 批量启动容器运行时
func (m *MemoryNodeManager) BatchStartContainerRuntime(nodeIds []string, runtimeType string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.StartContainerRuntime(id, runtimeType); err != nil {
			results.WriteString(fmt.Sprintf("启动失败: %v\n\n", err))
		} else {
			results.WriteString("启动成功\n\n")
		}
	}
	return results.String(), nil
}

// 批量停止容器运行时
func (m *MemoryNodeManager) BatchStopContainerRuntime(nodeIds []string, runtimeType string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.StopContainerRuntime(id, runtimeType); err != nil {
			results.WriteString(fmt.Sprintf("停止失败: %v\n\n", err))
		} else {
			results.WriteString("停止成功\n\n")
		}
	}
	return results.String(), nil
}

// 批量移除容器运行时
func (m *MemoryNodeManager) BatchRemoveContainerRuntime(nodeIds []string, runtimeType string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.RemoveContainerRuntime(id, runtimeType); err != nil {
			results.WriteString(fmt.Sprintf("移除失败: %v\n\n", err))
		} else {
			results.WriteString("移除成功\n\n")
		}
	}
	return results.String(), nil
}

// 批量启用容器运行时开机自启
func (m *MemoryNodeManager) BatchEnableContainerRuntime(nodeIds []string, runtimeType string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.EnableContainerRuntime(id, runtimeType); err != nil {
			results.WriteString(fmt.Sprintf("启用失败: %v\n\n", err))
		} else {
			results.WriteString("启用成功\n\n")
		}
	}
	return results.String(), nil
}

// 批量禁用容器运行时开机自启
func (m *MemoryNodeManager) BatchDisableContainerRuntime(nodeIds []string, runtimeType string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.DisableContainerRuntime(id, runtimeType); err != nil {
			results.WriteString(fmt.Sprintf("禁用失败: %v\n\n", err))
		} else {
			results.WriteString("禁用成功\n\n")
		}
	}
	return results.String(), nil
}

// 批量检查容器运行时状态
func (m *MemoryNodeManager) BatchCheckContainerRuntimeStatus(nodeIds []string, runtimeType string) (map[string]string, error) {
	statusMap := make(map[string]string)
	for _, id := range nodeIds {
		status, err := m.CheckContainerRuntimeStatus(id, runtimeType)
		if err != nil {
			statusMap[id] = fmt.Sprintf("获取状态失败: %v", err)
		} else {
			statusMap[id] = status
		}
	}
	return statusMap, nil
}

// GetLogs 获取所有日志
func (m *MemoryNodeManager) GetLogs() ([]log.LogEntry, error) {
	// 内存实现不支持日志持久化，返回空数组
	return []log.LogEntry{}, nil
}

// GetLogsByNode 获取指定节点的日志
func (m *MemoryNodeManager) GetLogsByNode(nodeID string) ([]log.LogEntry, error) {
	// 内存实现不支持日志持久化，返回空数组
	return []log.LogEntry{}, nil
}

// ClearLogs 清除所有日志
func (m *MemoryNodeManager) ClearLogs() error {
	// 内存实现不支持日志持久化，直接返回
	return nil
}

// CreateLog 创建新日志
func (m *MemoryNodeManager) CreateLog(log log.LogEntry) error {
	// 内存实现不支持日志持久化，直接返回
	return nil
}

// InstallContainerRuntime 安装容器运行时
func (m *MemoryNodeManager) InstallContainerRuntime(id string, runtimeType string, version string) error {
	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 测试连接
	connected, err := m.TestConnection(id)
	if !connected {
		return err
	}

	// 执行安装逻辑
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	// 1. 检测操作系统类型
	distroCmd := `
	if [ -f /etc/os-release ]; then
		. /etc/os-release
		echo $ID
	fi
	`
	distroOutput, err := client.RunCommand(distroCmd)
	if err != nil {
		return err
	}
	distro := strings.TrimSpace(distroOutput)

	// 2. 安装容器运行时
	return m.installContainerRuntime(client, distro, runtimeType, version)
}

// ConfigureContainerRuntime 配置容器运行时
func (m *MemoryNodeManager) ConfigureContainerRuntime(id string, config ContainerRuntimeConfig) error {
	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 根据不同的容器运行时类型实现配置逻辑
	switch config.RuntimeType {
	case "containerd":
		// 配置containerd
		sshConfig := ssh.SSHConfig{
			Host:       node.IP,
			Port:       node.Port,
			Username:   node.Username,
			Password:   node.Password,
			PrivateKey: node.PrivateKey,
		}
		client, err := ssh.NewSSHClient(sshConfig)
		if err != nil {
			return err
		}
		defer client.Close()

		// 生成默认配置
		cmd := `sudo containerd config default | sudo tee /etc/containerd/config.toml`
		_, err = client.RunCommand(cmd)
		if err != nil {
			return err
		}

		// 配置systemd cgroup驱动
		cmd = `sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml`
		_, err = client.RunCommand(cmd)
		if err != nil {
			return err
		}

		// 重启containerd
		cmd = `sudo systemctl restart containerd`
		_, err = client.RunCommand(cmd)
		if err != nil {
			return err
		}

		return nil
	case "cri-o":
		// 配置cri-o
		// TODO: 实现cri-o配置
		return fmt.Errorf("ConfigureContainerRuntime for cri-o not implemented yet")
	default:
		return fmt.Errorf("ConfigureContainerRuntime not implemented for %s", config.RuntimeType)
	}
}

// StartContainerRuntime 启动容器运行时
func (m *MemoryNodeManager) StartContainerRuntime(id string, runtimeType string) error {
	// 根据运行时类型执行不同的启动命令
	var serviceName string
	switch runtimeType {
	case "containerd":
		serviceName = "containerd"
	case "cri-o":
		serviceName = "crio"
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}

	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行启动命令
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := fmt.Sprintf("sudo systemctl restart %s", serviceName)
	_, err = client.RunCommand(cmd)
	return err
}

// StopContainerRuntime 停止容器运行时
func (m *MemoryNodeManager) StopContainerRuntime(id string, runtimeType string) error {
	// 根据运行时类型执行不同的停止命令
	var serviceName string
	switch runtimeType {
	case "containerd":
		serviceName = "containerd"
	case "cri-o":
		serviceName = "crio"
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}

	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行停止命令
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	// 执行停止命令，忽略错误（服务可能未运行）
	cmd := fmt.Sprintf("sudo systemctl stop %s || true", serviceName)
	_, err = client.RunCommand(cmd)
	return err
}

// RemoveContainerRuntime 移除容器运行时
func (m *MemoryNodeManager) RemoveContainerRuntime(id string, runtimeType string) error {
	// 根据运行时类型执行不同的移除命令
	var serviceName, packageName string
	switch runtimeType {
	case "containerd":
		serviceName = "containerd"
		packageName = "containerd"
	case "cri-o":
		serviceName = "crio"
		packageName = "cri-o"
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}

	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行移除命令
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	// 停止服务
	cmd := fmt.Sprintf("sudo systemctl stop %s", serviceName)
	_, err = client.RunCommand(cmd)
	if err != nil {
		// 忽略停止服务的错误，继续移除
	}

	// 禁用服务
	cmd = fmt.Sprintf("sudo systemctl disable %s", serviceName)
	_, err = client.RunCommand(cmd)
	if err != nil {
		// 忽略禁用服务的错误，继续移除
	}

	// 移除软件包
	cmd = fmt.Sprintf(`
	if command -v apt-get &> /dev/null; then
		sudo apt-get remove -y %s
	elif command -v dnf &> /dev/null; then
		sudo dnf remove -y %s
	elif command -v yum &> /dev/null; then
		sudo yum remove -y %s
	fi
	`, packageName, packageName, packageName)
	_, err = client.RunCommand(cmd)
	if err != nil {
		return err
	}

	return nil
}

// EnableContainerRuntime 启用容器运行时开机自启
func (m *MemoryNodeManager) EnableContainerRuntime(id string, runtimeType string) error {
	// 根据运行时类型执行不同的启用命令
	var serviceName string
	switch runtimeType {
	case "containerd":
		serviceName = "containerd"
	case "cri-o":
		serviceName = "crio"
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}

	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行启用命令
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := fmt.Sprintf("sudo systemctl enable %s", serviceName)
	_, err = client.RunCommand(cmd)
	return err
}

// DisableContainerRuntime 禁用容器运行时开机自启
func (m *MemoryNodeManager) DisableContainerRuntime(id string, runtimeType string) error {
	// 根据运行时类型执行不同的禁用命令
	var serviceName string
	switch runtimeType {
	case "containerd":
		serviceName = "containerd"
	case "cri-o":
		serviceName = "crio"
	default:
		return fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}

	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行禁用命令
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := fmt.Sprintf("sudo systemctl disable %s", serviceName)
	_, err = client.RunCommand(cmd)
	return err
}

// CheckContainerRuntimeStatus 检查容器运行时状态
func (m *MemoryNodeManager) CheckContainerRuntimeStatus(id string, runtimeType string) (string, error) {
	// 根据运行时类型执行不同的状态检查命令
	var serviceName string
	switch runtimeType {
	case "containerd":
		serviceName = "containerd"
	case "cri-o":
		serviceName = "crio"
	default:
		return "", fmt.Errorf("unsupported runtime type: %s", runtimeType)
	}

	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return "", err
	}

	// 执行状态检查逻辑
	sshConfig := ssh.SSHConfig{
		Host:       node.IP,
		Port:       node.Port,
		Username:   node.Username,
		Password:   node.Password,
		PrivateKey: node.PrivateKey,
	}

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		return "", err
	}
	defer client.Close()

	cmd := fmt.Sprintf("sudo systemctl is-active %s", serviceName)
	output, err := client.RunCommand(cmd)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}
