package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"k8s-installer/ssh"
)

// FileNodeManager 文件节点管理器
type FileNodeManager struct {
	nodes         map[string]Node
	mutex         sync.RWMutex
	filePath      string
	scriptManager interface{} // 脚本管理器接口
}

// NewFileNodeManager 创建新的文件节点管理器
func NewFileNodeManager(filePath string) (*FileNodeManager, error) {
	manager := &FileNodeManager{
		nodes:    make(map[string]Node),
		filePath: filePath,
	}

	// 尝试加载现有数据
	if err := manager.loadNodes(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load nodes: %v", err)
	}

	return manager, nil
}

// loadNodes 从文件加载节点数据
func (m *FileNodeManager) loadNodes() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	file, err := os.Open(m.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var nodes []Node
	if err := json.NewDecoder(file).Decode(&nodes); err != nil {
		return err
	}

	// 将节点转换为map
	for _, node := range nodes {
		m.nodes[node.ID] = node
	}

	return nil
}

// saveNodes 将节点数据保存到文件
func (m *FileNodeManager) saveNodes() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 将map转换为切片
	var nodes []Node
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}

	file, err := os.Create(m.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(nodes)
}

// GetNodes 获取所有节点
func (m *FileNodeManager) GetNodes() ([]Node, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var nodes []Node
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetNode 根据ID获取节点
func (m *FileNodeManager) GetNode(id string) (*Node, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	node, exists := m.nodes[id]
	if !exists {
		return nil, errors.New("node not found")
	}

	return &node, nil
}

// CreateNode 创建新节点
func (m *FileNodeManager) CreateNode(node Node) (*Node, error) {
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

	// 保存到文件
	if err := m.saveNodes(); err != nil {
		return nil, fmt.Errorf("failed to save node: %v", err)
	}

	return &node, nil
}

// UpdateNode 更新节点信息
func (m *FileNodeManager) UpdateNode(id string, node Node) (*Node, error) {
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

	// 保存到文件
	if err := m.saveNodes(); err != nil {
		return nil, fmt.Errorf("failed to update node: %v", err)
	}

	return &node, nil
}

// DeleteNode 删除节点
func (m *FileNodeManager) DeleteNode(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.nodes[id]; !exists {
		return errors.New("node not found")
	}

	delete(m.nodes, id)
	return m.saveNodes()
}

// SetScriptManager 设置脚本管理器
func (m *FileNodeManager) SetScriptManager(scriptManager interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.scriptManager = scriptManager
	return nil
}

// TestConnection 测试节点连接
func (m *FileNodeManager) TestConnection(id string) (bool, error) {
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

		// 保存到文件
		if saveErr := m.saveNodes(); saveErr != nil {
			return false, fmt.Errorf("failed to update node status: %v", saveErr)
		}

		return false, err
	}
	defer client.Close()

	// 更新节点状态为在线
	m.mutex.Lock()
	node.Status = NodeStatusOnline
	node.UpdatedAt = time.Now()
	m.nodes[id] = node
	m.mutex.Unlock()

	// 保存到文件
	if saveErr := m.saveNodes(); saveErr != nil {
		return false, fmt.Errorf("failed to update node status: %v", saveErr)
	}

	return true, nil
}

// ConfigureSSHSettings 配置节点SSH设置
func (m *FileNodeManager) ConfigureSSHSettings(id string) error {
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
func (m *FileNodeManager) ConfigureSSHPasswdless() error {
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

// deployMasterNode 部署主节点
func (m *FileNodeManager) deployMasterNode(client *ssh.SSHClient) error {
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

	// 3. 安装kubeadm, kubelet和kubectl
	if err := m.installKubernetesComponents(client, distro); err != nil {
		return err
	}

	// 4. 禁用swap
	swapCmd := `
swapoff -a
sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
`
	if _, err := client.RunCommand(swapCmd); err != nil {
		return err
	}

	// 5. 设置内核参数（生产环境推荐配置）
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

// deployWorkerNode 部署工作节点
func (m *FileNodeManager) deployWorkerNode(client *ssh.SSHClient) error {
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

	// 3. 安装kubeadm和kubelet
	if err := m.installKubernetesComponents(client, distro); err != nil {
		return err
	}

	// 4. 禁用swap
	swapCmd := `
swapoff -a
sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
`
	if _, err := client.RunCommand(swapCmd); err != nil {
		return err
	}

	// 5. 设置内核参数
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
func (m *FileNodeManager) installContainerRuntime(client *ssh.SSHClient, distro, runtime, version string) error {
	var cmd string
	// 只支持containerd
	switch distro {
	case "ubuntu", "debian":
		cmd = `
		apt-get update && apt-get install -y containerd
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
		cmd = `
	if command -v dnf &> /dev/null; then
		dnf install -y containerd
	else
		yum install -y containerd
	fi
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

// InstallKubernetesComponents 安装Kubernetes组件（公开方法，实现NodeManager接口）
func (m *FileNodeManager) InstallKubernetesComponents(id string, kubeadmVersion string) error {
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

	// 调用私有的安装方法
	return m.installKubernetesComponents(client, distro)
}

// installKubernetesComponents 安装Kubernetes组件（私有辅助方法）
func (m *FileNodeManager) installKubernetesComponents(client *ssh.SSHClient, distro string) error {
	var cmd string
	var found bool

	// 从脚本管理器获取Kubernetes组件安装脚本
	if m.scriptManager != nil {
		if scriptGetter, ok := m.scriptManager.(interface {
			GetScript(name string) (string, bool)
		}); ok {
			// 尝试获取特定发行版的脚本，使用与前端一致的命名格式: ${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}
			// 前端步骤名为 "安装Kubernetes组件"
			componentScriptName := fmt.Sprintf("%s_安装kubernetes组件", distro)
			if script, scriptFound := scriptGetter.GetScript(componentScriptName); scriptFound {
				cmd = script
				found = true
				fmt.Printf("Using custom script for Kubernetes components installation on %s\n", distro)
			} else {
				// 尝试获取旧格式的脚本，保持向后兼容
				oldComponentScriptName := fmt.Sprintf("k8s_components_%s", distro)
				if script, scriptFound := scriptGetter.GetScript(oldComponentScriptName); scriptFound {
					cmd = script
					found = true
					fmt.Printf("Using old format custom script for Kubernetes components installation on %s\n", distro)
				} else {
					// 尝试获取通用脚本
					if script, scriptFound := scriptGetter.GetScript("k8s_components"); scriptFound {
						cmd = script
						found = true
						fmt.Printf("Using custom script for Kubernetes components installation\n")
					}
				}
			}
		}
	}

	// 如果没有找到自定义脚本，使用默认命令
	if !found {
		switch distro {
		case "ubuntu", "debian":
			cmd = `
apt-get update
apt-get install -y apt-transport-https ca-certificates curl
curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg https://packages.cloud.google.com/apt/doc/apt-key.gpg
echo "deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | tee /etc/apt/sources.list.d/kubernetes.list
apt-get update
# 锁定版本到最新稳定版，生产环境推荐
K8S_VERSION=$(apt-cache madison kubeadm | grep -E "^kubeadm\s+\|\s+\d+\.\d+\.\d+" | head -1 | awk '{print $3}' | sed 's/-00//')
apt-get install -y kubeadm=$K8S_VERSION kubelet=$K8S_VERSION kubectl=$K8S_VERSION
apt-mark hold kubelet kubeadm kubectl

# 生产环境优化：配置kubelet使用systemd cgroup驱动
cat <<EOF | tee /etc/default/kubelet
KUBELET_EXTRA_ARGS="--cgroup-driver=systemd --runtime-cgroups=/system.slice/containerd.service --kubelet-cgroups=/system.slice/kubelet.service"
EOF

systemctl daemon-reload
systemctl restart kubelet
systemctl enable kubelet
		`
		case "centos", "rhel", "rocky", "alma":
			cmd = `
yum install -y yum-utils
yum-config-manager --add-repo https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
rpm --import https://packages.cloud.google.com/yum/doc/yum-key.gpg
rpm --import https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
# 禁用SELINUX（生产环境推荐）
setenforce 0
sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config
# 锁定版本到最新稳定版
K8S_VERSION=$(yum list --showduplicates kubeadm --disableexcludes=kubernetes | sort -r | grep -E "^kubeadm\.x86_64" | head -1 | awk '{print $2}')
yum install -y kubelet-$K8S_VERSION kubeadm-$K8S_VERSION kubectl-$K8S_VERSION
yum versionlock add kubelet kubeadm kubectl

# 生产环境优化：配置kubelet使用systemd cgroup驱动
cat <<EOF | tee /etc/sysconfig/kubelet
KUBELET_EXTRA_ARGS="--cgroup-driver=systemd --runtime-cgroups=/system.slice/containerd.service --kubelet-cgroups=/system.slice/kubelet.service"
EOF

systemctl daemon-reload
systemctl restart kubelet
systemctl enable kubelet
		`
		default:
			return fmt.Errorf("unsupported distribution: %s", distro)
		}
	}

	_, err := client.RunCommand(cmd)
	return err
}
