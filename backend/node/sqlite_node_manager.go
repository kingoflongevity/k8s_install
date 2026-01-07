package node

import (
	"database/sql"
	"errors"
	"fmt"
	"k8s-installer/log"
	"strings"
	"sync"
	"time"

	"k8s-installer/ssh"

	_ "modernc.org/sqlite"
)

// SqliteNodeManager SQLite节点管理器
type SqliteNodeManager struct {
	db         *sql.DB
	mutex      sync.RWMutex
	logManager *log.SqliteLogManager
}

// NewSqliteNodeManager 创建新的SQLite节点管理器
func NewSqliteNodeManager(dbPath string) (*SqliteNodeManager, error) {
	// 打开数据库连接
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// 创建nodes表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		ip TEXT NOT NULL,
		port INTEGER NOT NULL DEFAULT 22,
		username TEXT NOT NULL,
		password TEXT,
		private_key TEXT,
		node_type TEXT NOT NULL DEFAULT 'worker',
		status TEXT NOT NULL DEFAULT 'offline',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %v", err)
	}

	// 初始化日志管理器
	logManager, err := log.NewSqliteLogManager(db)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize log manager: %v", err)
	}

	return &SqliteNodeManager{
		db:         db,
		logManager: logManager,
	}, nil
}

// GetNodes 获取所有节点
func (m *SqliteNodeManager) GetNodes() ([]Node, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	rows, err := m.db.Query("SELECT id, name, ip, port, username, password, private_key, node_type, status, created_at, updated_at FROM nodes")
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %v", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var node Node
		if err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.IP,
			&node.Port,
			&node.Username,
			&node.Password,
			&node.PrivateKey,
			&node.NodeType,
			&node.Status,
			&node.CreatedAt,
			&node.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan node: %v", err)
		}
		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %v", err)
	}

	return nodes, nil
}

// GetNode 根据ID获取节点
func (m *SqliteNodeManager) GetNode(id string) (*Node, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var node Node
	err := m.db.QueryRow(
		"SELECT id, name, ip, port, username, password, private_key, node_type, status, created_at, updated_at FROM nodes WHERE id = ?",
		id,
	).Scan(
		&node.ID,
		&node.Name,
		&node.IP,
		&node.Port,
		&node.Username,
		&node.Password,
		&node.PrivateKey,
		&node.NodeType,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("node not found")
		}
		return nil, fmt.Errorf("failed to get node: %v", err)
	}

	return &node, nil
}

// CreateNode 创建新节点
func (m *SqliteNodeManager) CreateNode(node Node) (*Node, error) {
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

	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}

	node.UpdatedAt = time.Now()

	// 插入数据
	_, err := m.db.Exec(
		"INSERT INTO nodes (id, name, ip, port, username, password, private_key, node_type, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		node.ID,
		node.Name,
		node.IP,
		node.Port,
		node.Username,
		node.Password,
		node.PrivateKey,
		node.NodeType,
		node.Status,
		node.CreatedAt,
		node.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to insert node: %v", err)
	}

	return &node, nil
}

// UpdateNode 更新节点信息
func (m *SqliteNodeManager) UpdateNode(id string, node Node) (*Node, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查节点是否存在
	exists, err := m.nodeExists(id)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errors.New("node not found")
	}

	// 更新节点信息
	node.ID = id
	node.UpdatedAt = time.Now()

	_, err = m.db.Exec(
		"UPDATE nodes SET name = ?, ip = ?, port = ?, username = ?, password = ?, private_key = ?, node_type = ?, status = ?, updated_at = ? WHERE id = ?",
		node.Name,
		node.IP,
		node.Port,
		node.Username,
		node.Password,
		node.PrivateKey,
		node.NodeType,
		node.Status,
		node.UpdatedAt,
		node.ID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update node: %v", err)
	}

	return &node, nil
}

// DeleteNode 删除节点
func (m *SqliteNodeManager) DeleteNode(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查节点是否存在
	exists, err := m.nodeExists(id)
	if err != nil {
		return err
	}

	if !exists {
		return errors.New("node not found")
	}

	// 删除节点
	_, err = m.db.Exec("DELETE FROM nodes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete node: %v", err)
	}

	return nil
}

// TestConnection 测试节点连接
func (m *SqliteNodeManager) TestConnection(id string) (bool, error) {
	m.mutex.RLock()
	node, err := m.GetNode(id)
	m.mutex.RUnlock()

	if err != nil {
		return false, err
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
		// 更新节点状态为离线
		m.mutex.Lock()
		node.Status = NodeStatusOffline
		node.UpdatedAt = time.Now()
		m.updateNodeStatus(id, node.Status, node.UpdatedAt)
		m.mutex.Unlock()

		// 记录日志
		logEntry := log.LogEntry{
			ID:        time.Now().Format("20060102150405"),
			NodeID:    node.ID,
			NodeName:  node.Name,
			Operation: "TestConnection",
			Command:   fmt.Sprintf("Test SSH connection to %s:%d", node.IP, node.Port),
			Output:    fmt.Sprintf("Error: %v", err),
			Status:    "failed",
			CreatedAt: time.Now(),
		}
		m.CreateLog(logEntry)

		return false, err
	}
	defer client.Close()

	// 执行简单命令测试连接
	testCmd := "echo 'hello'"
	output, err := client.RunCommand(testCmd)
	if err != nil {
		// 更新节点状态为离线
		m.mutex.Lock()
		node.Status = NodeStatusOffline
		node.UpdatedAt = time.Now()
		m.updateNodeStatus(id, node.Status, node.UpdatedAt)
		m.mutex.Unlock()

		// 记录日志
		logEntry := log.LogEntry{
			ID:        time.Now().Format("20060102150405"),
			NodeID:    node.ID,
			NodeName:  node.Name,
			Operation: "TestConnection",
			Command:   fmt.Sprintf("Test SSH connection with command: %s", testCmd),
			Output:    fmt.Sprintf("Error: %v\nOutput: %s", err, output),
			Status:    "failed",
			CreatedAt: time.Now(),
		}
		m.CreateLog(logEntry)

		return false, err
	}

	// 更新节点状态为在线
	m.mutex.Lock()
	node.Status = NodeStatusOnline
	node.UpdatedAt = time.Now()
	m.updateNodeStatus(id, node.Status, node.UpdatedAt)
	m.mutex.Unlock()

	// 记录日志
	logEntry := log.LogEntry{
		ID:        time.Now().Format("20060102150405"),
		NodeID:    node.ID,
		NodeName:  node.Name,
		Operation: "TestConnection",
		Command:   fmt.Sprintf("Test SSH connection to %s:%d", node.IP, node.Port),
		Output:    output,
		Status:    "success",
		CreatedAt: time.Now(),
	}
	m.CreateLog(logEntry)

	return true, nil
}

// ConfigureSSHSettings 配置节点SSH设置
func (m *SqliteNodeManager) ConfigureSSHSettings(id string) error {
	m.mutex.RLock()
	node, err := m.GetNode(id)
	m.mutex.RUnlock()

	if err != nil {
		return err
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
func (m *SqliteNodeManager) ConfigureSSHPasswdless() error {
	m.mutex.RLock()
	allNodes, err := m.GetNodes()
	m.mutex.RUnlock()

	if err != nil {
		return err
	}

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

// InstallDocker 安装Docker
func (m *SqliteNodeManager) InstallDocker(id string, version string) error {
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

	// 2. 安装Docker
	output, err := m.installContainerRuntime(client, distro, "docker", version)

	// 记录日志
	logEntry := log.LogEntry{
		ID:        time.Now().Format("20060102150405"),
		NodeID:    node.ID,
		NodeName:  node.Name,
		Operation: "InstallDocker",
		Command:   fmt.Sprintf("Install Docker on %s", distro),
		Output:    output,
		Status:    "success",
		CreatedAt: time.Now(),
	}

	if err != nil {
		logEntry.Status = "failed"
		logEntry.Output = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
		m.CreateLog(logEntry)
		return err
	}

	m.CreateLog(logEntry)
	return nil
}

// ConfigureDocker 配置Docker
func (m *SqliteNodeManager) ConfigureDocker(id string, config DockerConfig) error {
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

	// 执行配置逻辑
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

	// 构建daemon.json配置
	configCmd := fmt.Sprintf(`
mkdir -p /etc/docker
cat <<EOF | tee /etc/docker/daemon.json
{
	"exec-opts": ["native.cgroupdriver=%s"],
	"log-driver": "%s",
	"log-opts": {
		"max-size": "%s",
		"max-file": "%d"
	},
	"storage-driver": "%s",
	"registry-mirrors": %s,
	"data-root": "%s"
}
EOF
systemctl daemon-reload
systemctl restart docker
	`,
		config.CgroupDriver,
		config.LogDriver,
		config.LogMaxSize,
		config.LogMaxFile,
		config.StorageDriver,
		sqliteFormatRegMirrors(config.RegistryMirrors),
		config.DataRoot,
	)

	output, err := client.RunCommand(configCmd)

	// 记录日志
	logEntry := log.LogEntry{
		ID:        time.Now().Format("20060102150405"),
		NodeID:    node.ID,
		NodeName:  node.Name,
		Operation: "ConfigureDocker",
		Command:   "Configure Docker daemon.json",
		Output:    output,
		Status:    "success",
		CreatedAt: time.Now(),
	}

	if err != nil {
		logEntry.Status = "failed"
		logEntry.Output = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
		m.CreateLog(logEntry)
		return err
	}

	m.CreateLog(logEntry)
	return nil
}

// StartDocker 启动Docker服务
func (m *SqliteNodeManager) StartDocker(id string) error {
	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行启动逻辑
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

	startCmd := `
systemctl start docker
systemctl enable docker
`

	output, err := client.RunCommand(startCmd)

	// 记录日志
	logEntry := log.LogEntry{
		ID:        time.Now().Format("20060102150405"),
		NodeID:    node.ID,
		NodeName:  node.Name,
		Operation: "StartDocker",
		Command:   "Start Docker service",
		Output:    output,
		Status:    "success",
		CreatedAt: time.Now(),
	}

	if err != nil {
		logEntry.Status = "failed"
		logEntry.Output = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
		m.CreateLog(logEntry)
		return err
	}

	m.CreateLog(logEntry)
	return nil
}

// StopDocker 停止Docker服务
func (m *SqliteNodeManager) StopDocker(id string) error {
	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行停止逻辑
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

	stopCmd := `
systemctl stop docker
systemctl disable docker
`

	output, err := client.RunCommand(stopCmd)

	// 记录日志
	logEntry := log.LogEntry{
		ID:        time.Now().Format("20060102150405"),
		NodeID:    node.ID,
		NodeName:  node.Name,
		Operation: "StopDocker",
		Command:   "Stop Docker service",
		Output:    output,
		Status:    "success",
		CreatedAt: time.Now(),
	}

	if err != nil {
		logEntry.Status = "failed"
		logEntry.Output = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
		m.CreateLog(logEntry)
		return err
	}

	m.CreateLog(logEntry)
	return nil
}

// CheckDockerStatus 检查Docker服务状态
func (m *SqliteNodeManager) CheckDockerStatus(id string) (string, error) {
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

	statusCmd := `
if systemctl is-active --quiet docker; then
    echo "running"
elif systemctl is-enabled --quiet docker; then
    echo "enabled but not running"
else
    echo "stopped"
fi
`

	statusOutput, err := client.RunCommand(statusCmd)
	if err != nil {
		return "", err
	}

	status := strings.TrimSpace(statusOutput)

	// 记录日志
	logEntry := log.LogEntry{
		ID:        time.Now().Format("20060102150405"),
		NodeID:    node.ID,
		NodeName:  node.Name,
		Operation: "CheckDockerStatus",
		Command:   "Check Docker service status",
		Output:    status,
		Status:    "success",
		CreatedAt: time.Now(),
	}

	m.CreateLog(logEntry)

	return status, nil
}

// BatchInstallDocker 批量安装Docker容器运行时
func (m *SqliteNodeManager) BatchInstallDocker(nodeIds []string, version string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.InstallDocker(id, version); err != nil {
			results.WriteString(fmt.Sprintf("安装失败: %v\n\n", err))
		} else {
			results.WriteString("安装成功\n\n")
		}
	}
	return results.String(), nil
}

// BatchConfigureDocker 批量配置Docker
func (m *SqliteNodeManager) BatchConfigureDocker(nodeIds []string, config DockerConfig) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.ConfigureDocker(id, config); err != nil {
			results.WriteString(fmt.Sprintf("配置失败: %v\n\n", err))
		} else {
			results.WriteString("配置成功\n\n")
		}
	}
	return results.String(), nil
}

// BatchStartDocker 批量启动Docker服务
func (m *SqliteNodeManager) BatchStartDocker(nodeIds []string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.StartDocker(id); err != nil {
			results.WriteString(fmt.Sprintf("启动失败: %v\n\n", err))
		} else {
			results.WriteString("启动成功\n\n")
		}
	}
	return results.String(), nil
}

// BatchStopDocker 批量停止Docker服务
func (m *SqliteNodeManager) BatchStopDocker(nodeIds []string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.StopDocker(id); err != nil {
			results.WriteString(fmt.Sprintf("停止失败: %v\n\n", err))
		} else {
			results.WriteString("停止成功\n\n")
		}
	}
	return results.String(), nil
}

// RemoveDocker 删除Docker服务
func (m *SqliteNodeManager) RemoveDocker(id string) error {
	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行删除逻辑
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

	removeCmd := `
# 停止Docker服务
systemctl stop docker
# 禁用Docker服务
systemctl disable docker
# 删除Docker包
if command -v apt-get &> /dev/null; then
    apt-get remove -y docker-ce docker-ce-cli containerd.io
    apt-get autoremove -y
    apt-get purge -y docker-ce docker-ce-cli containerd.io
    rm -rf /var/lib/docker
    rm -rf /var/lib/containerd
    rm -rf /etc/docker
elif command -v yum &> /dev/null; then
    yum remove -y docker-ce docker-ce-cli containerd.io
    yum autoremove -y
    rm -rf /var/lib/docker
    rm -rf /var/lib/containerd
    rm -rf /etc/docker
fi
`

	output, err := client.RunCommand(removeCmd)

	// 记录日志
	logEntry := log.LogEntry{
		ID:        time.Now().Format("20060102150405"),
		NodeID:    node.ID,
		NodeName:  node.Name,
		Operation: "RemoveDocker",
		Command:   "Remove Docker service",
		Output:    output,
		Status:    "success",
		CreatedAt: time.Now(),
	}

	if err != nil {
		logEntry.Status = "failed"
		logEntry.Output = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
		m.CreateLog(logEntry)
		return err
	}

	m.CreateLog(logEntry)
	return nil
}

// EnableDocker 启用Docker开机自启
func (m *SqliteNodeManager) EnableDocker(id string) error {
	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行启用逻辑
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

	enableCmd := `
# 启用Docker开机自启
systemctl enable docker
`

	output, err := client.RunCommand(enableCmd)

	// 记录日志
	logEntry := log.LogEntry{
		ID:        time.Now().Format("20060102150405"),
		NodeID:    node.ID,
		NodeName:  node.Name,
		Operation: "EnableDocker",
		Command:   "Enable Docker auto-start",
		Output:    output,
		Status:    "success",
		CreatedAt: time.Now(),
	}

	if err != nil {
		logEntry.Status = "failed"
		logEntry.Output = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
		m.CreateLog(logEntry)
		return err
	}

	m.CreateLog(logEntry)
	return nil
}

// DisableDocker 禁用Docker开机自启
func (m *SqliteNodeManager) DisableDocker(id string) error {
	// 获取节点
	node, err := m.GetNode(id)
	if err != nil {
		return err
	}

	// 执行禁用逻辑
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

	disableCmd := `
# 禁用Docker开机自启
systemctl disable docker
`

	output, err := client.RunCommand(disableCmd)

	// 记录日志
	logEntry := log.LogEntry{
		ID:        time.Now().Format("20060102150405"),
		NodeID:    node.ID,
		NodeName:  node.Name,
		Operation: "DisableDocker",
		Command:   "Disable Docker auto-start",
		Output:    output,
		Status:    "success",
		CreatedAt: time.Now(),
	}

	if err != nil {
		logEntry.Status = "failed"
		logEntry.Output = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
		m.CreateLog(logEntry)
		return err
	}

	m.CreateLog(logEntry)
	return nil
}

// BatchRemoveDocker 批量删除Docker服务
func (m *SqliteNodeManager) BatchRemoveDocker(nodeIds []string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.RemoveDocker(id); err != nil {
			results.WriteString(fmt.Sprintf("删除失败: %v\n\n", err))
		} else {
			results.WriteString("删除成功\n\n")
		}
	}
	return results.String(), nil
}

// BatchEnableDocker 批量启用Docker开机自启
func (m *SqliteNodeManager) BatchEnableDocker(nodeIds []string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.EnableDocker(id); err != nil {
			results.WriteString(fmt.Sprintf("启用自启失败: %v\n\n", err))
		} else {
			results.WriteString("启用自启成功\n\n")
		}
	}
	return results.String(), nil
}

// BatchDisableDocker 批量禁用Docker开机自启
func (m *SqliteNodeManager) BatchDisableDocker(nodeIds []string) (string, error) {
	var results strings.Builder
	for _, id := range nodeIds {
		results.WriteString(fmt.Sprintf("=== 节点 %s ===\n", id))
		if err := m.DisableDocker(id); err != nil {
			results.WriteString(fmt.Sprintf("禁用自启失败: %v\n\n", err))
		} else {
			results.WriteString("禁用自启成功\n\n")
		}
	}
	return results.String(), nil
}

// BatchCheckDockerStatus 批量检查Docker服务状态
func (m *SqliteNodeManager) BatchCheckDockerStatus(nodeIds []string) (map[string]string, error) {
	statusMap := make(map[string]string)
	for _, id := range nodeIds {
		status, err := m.CheckDockerStatus(id)
		if err != nil {
			statusMap[id] = fmt.Sprintf("获取状态失败: %v", err)
		} else {
			statusMap[id] = status
		}
	}
	return statusMap, nil
}

// 辅助方法：更新节点状态
func (m *SqliteNodeManager) updateNodeStatus(id, status string, updatedAt time.Time) error {
	_, err := m.db.Exec(
		"UPDATE nodes SET status = ?, updated_at = ? WHERE id = ?",
		status,
		updatedAt,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update node status: %v", err)
	}
	return nil
}

// 辅助方法：检查节点是否存在
func (m *SqliteNodeManager) nodeExists(id string) (bool, error) {
	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM nodes WHERE id = ?", id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check node existence: %v", err)
	}
	return count > 0, nil
}

// sqliteFormatRegMirrors 格式化镜像加速地址为JSON数组
func sqliteFormatRegMirrors(mirrors []string) string {
	if len(mirrors) == 0 {
		return "[]"
	}

	result := "["
	for i, mirror := range mirrors {
		result += fmt.Sprintf(`\"%s\"`, mirror)
		if i < len(mirrors)-1 {
			result += ","
		}
	}
	result += "]"
	return result
}

// installContainerRuntime 安装容器运行时
func (m *SqliteNodeManager) installContainerRuntime(client *ssh.SSHClient, distro, runtime, version string) (string, error) {
	var cmd string
	switch distro {
	case "ubuntu", "debian":
		if runtime == "containerd" {
			cmd = `
			apt-get update && apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release
			mkdir -p /etc/apt/keyrings
			curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
			echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
			apt-get update
			apt-get install -y containerd.io
			mkdir -p /etc/containerd
			containerd config default | tee /etc/containerd/config.toml
			sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
			systemctl restart containerd
			systemctl enable containerd
			`
		} else if runtime == "docker" {
			// 构建Docker安装命令，支持指定版本
			installCmd := "apt-get install -y docker-ce docker-ce-cli containerd.io"
			if version != "" {
				installCmd = fmt.Sprintf("apt-get install -y docker-ce=%s docker-ce-cli=%s containerd.io", version, version)
			}
			cmd = fmt.Sprintf(`
			apt-get update && apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release
			mkdir -p /etc/apt/keyrings
			curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
			echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
			apt-get update
			%s
			systemctl restart docker
			systemctl enable docker
			`, installCmd)
		}
	case "centos", "rhel", "rocky", "almalinux":
		if runtime == "containerd" {
			cmd = `
			yum install -y yum-utils
			yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
			yum install -y containerd.io
			mkdir -p /etc/containerd
			containerd config default | tee /etc/containerd/config.toml
			sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
			systemctl restart containerd
			systemctl enable containerd
			`
		} else if runtime == "docker" {
			// 构建Docker安装命令，支持指定版本
			installCmd := "yum install -y docker-ce docker-ce-cli containerd.io"
			if version != "" {
				installCmd = fmt.Sprintf("yum install -y docker-ce-%s docker-ce-cli-%s containerd.io", version, version)
			}
			cmd = fmt.Sprintf(`
			yum install -y yum-utils
			yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
			%s
			systemctl restart docker
			systemctl enable docker
			`, installCmd)
		}
	default:
		return "", fmt.Errorf("unsupported distribution: %s", distro)
	}

	output, err := client.RunCommand(cmd)
	return output, err
}

// GetLogs 获取所有日志
func (m *SqliteNodeManager) GetLogs() ([]log.LogEntry, error) {
	return m.logManager.GetLogs()
}

// GetLogsByNode 获取指定节点的日志
func (m *SqliteNodeManager) GetLogsByNode(nodeID string) ([]log.LogEntry, error) {
	return m.logManager.GetLogsByNode(nodeID)
}

// ClearLogs 清除所有日志
func (m *SqliteNodeManager) ClearLogs() error {
	return m.logManager.ClearLogs()
}

// CreateLog 创建新日志
func (m *SqliteNodeManager) CreateLog(log log.LogEntry) error {
	return m.logManager.CreateLog(log)
}
