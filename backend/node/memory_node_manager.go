package node

import (
	"errors"
	"sync"
	"time"

	"k8s-installer/ssh"
)

// MemoryNodeManager 内存节点管理器
type MemoryNodeManager struct {
	nodes map[string]Node
	mutex sync.RWMutex
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

	// 生成ID（这里简化处理，实际应该使用UUID）
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

	// 更新节点状态为在线
	m.mutex.Lock()
	node.Status = NodeStatusOnline
	node.UpdatedAt = time.Now()
	m.nodes[id] = node
	m.mutex.Unlock()

	return true, nil
}

// DeployNode 部署节点
func (m *MemoryNodeManager) DeployNode(id string) error {
	m.mutex.Lock()
	// 更新节点状态为部署中
	node, exists := m.nodes[id]
	if !exists {
		m.mutex.Unlock()
		return errors.New("node not found")
	}

	node.Status = NodeStatusDeploying
	node.UpdatedAt = time.Now()
	m.nodes[id] = node
	m.mutex.Unlock()

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

	// 根据节点类型执行不同的部署命令
	if node.NodeType == NodeTypeMaster {
		// 执行主节点部署命令
		err = m.deployMasterNode(client)
	} else {
		// 执行工作节点部署命令
		err = m.deployWorkerNode(client)
	}

	if err != nil {
		// 更新节点状态为错误
		m.mutex.Lock()
		node.Status = NodeStatusError
		node.UpdatedAt = time.Now()
		m.nodes[id] = node
		m.mutex.Unlock()
		return err
	}

	// 更新节点状态为就绪
	m.mutex.Lock()
	node.Status = NodeStatusReady
	node.UpdatedAt = time.Now()
	m.nodes[id] = node
	m.mutex.Unlock()

	return nil
}

// deployMasterNode 部署主节点
func (m *MemoryNodeManager) deployMasterNode(client *ssh.SSHClient) error {
	// 1. 安装Docker
	dockerCmd := `
	apt-get update && apt-get install -y docker.io
	systemctl enable docker && systemctl start docker
	`
	if _, err := client.RunCommand(dockerCmd); err != nil {
		return err
	}

	// 2. 安装kubeadm, kubelet和kubectl
	kubeadmCmd := `
	apt-get update && apt-get install -y apt-transport-https ca-certificates curl
	curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg https://packages.cloud.google.com/apt/doc/apt-key.gpg
	echo "deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | tee /etc/apt/sources.list.d/kubernetes.list
	apt-get update
	apt-get install -y kubelet kubeadm kubectl
	systemctl enable --now kubelet
	`
	if _, err := client.RunCommand(kubeadmCmd); err != nil {
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
	echo "net.bridge.bridge-nf-call-iptables=1" >> /etc/sysctl.conf
	echo "net.bridge.bridge-nf-call-ip6tables=1" >> /etc/sysctl.conf
	echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
	sysctl -p
	modprobe br_netfilter
	`
	if _, err := client.RunCommand(kernelCmd); err != nil {
		return err
	}

	return nil
}

// deployWorkerNode 部署工作节点
func (m *MemoryNodeManager) deployWorkerNode(client *ssh.SSHClient) error {
	// 1. 安装Docker
	dockerCmd := `
	apt-get update && apt-get install -y docker.io
	systemctl enable docker && systemctl start docker
	`
	if _, err := client.RunCommand(dockerCmd); err != nil {
		return err
	}

	// 2. 安装kubeadm和kubelet
	kubeadmCmd := `
	apt-get update && apt-get install -y apt-transport-https ca-certificates curl
	curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg https://packages.cloud.google.com/apt/doc/apt-key.gpg
	echo "deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | tee /etc/apt/sources.list.d/kubernetes.list
	apt-get update
	apt-get install -y kubelet kubeadm
	systemctl enable --now kubelet
	`
	if _, err := client.RunCommand(kubeadmCmd); err != nil {
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
	echo "net.bridge.bridge-nf-call-iptables=1" >> /etc/sysctl.conf
	echo "net.bridge.bridge-nf-call-ip6tables=1" >> /etc/sysctl.conf
	echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
	sysctl -p
	modprobe br_netfilter
	`
	if _, err := client.RunCommand(kernelCmd); err != nil {
		return err
	}

	return nil
}
