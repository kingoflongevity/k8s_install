package node

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s-installer/log"
	"k8s-installer/ssh"

	// 使用纯Go实现的SQLite驱动，不需要CGO
	_ "modernc.org/sqlite"
)

// SqliteNodeManager SQLite节点管理器
type SqliteNodeManager struct {
	db            *sql.DB
	mutex         sync.RWMutex
	scriptManager interface{}    // 脚本管理器接口
	logManager    log.LogManager // 日志管理器
}

// GetDB 获取数据库连接
func (m *SqliteNodeManager) GetDB() interface{} {
	return m.db
}

// NewSqliteNodeManager 创建新的SQLite节点管理器
func NewSqliteNodeManager(dbPath string) (*SqliteNodeManager, error) {
	// 打开数据库连接，使用modernc.org/sqlite驱动，驱动名称为"sqlite"
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
		return nil, fmt.Errorf("failed to create nodes table: %v", err)
	}

	// 创建scripts表，用于存储部署流程脚本
	createScriptsTableSQL := `
	CREATE TABLE IF NOT EXISTS scripts (
		name TEXT PRIMARY KEY,
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`

	_, err = db.Exec(createScriptsTableSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to create scripts table: %v", err)
	}

	// 创建日志管理器
	logManager, err := log.NewSqliteLogManager(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create log manager: %v", err)
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

// SetScriptManager 设置脚本管理器
func (m *SqliteNodeManager) SetScriptManager(scriptManager interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.scriptManager = scriptManager
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

	fmt.Printf("=== 测试节点 %s 的SSH连接 ===\n", node.Name)
	fmt.Printf("连接到: %s@%s:%d\n", node.Username, node.IP, node.Port)

	client, err := ssh.NewSSHClient(sshConfig)
	if err != nil {
		fmt.Printf("✗ SSH客户端创建失败: %v\n", err)
		// 更新节点状态为离线
		m.mutex.Lock()
		node.Status = NodeStatusOffline
		node.UpdatedAt = time.Now()
		m.updateNodeStatus(id, node.Status, node.UpdatedAt)
		m.mutex.Unlock()
		return false, err
	}
	defer client.Close()

	// 执行简单命令测试连接
	fmt.Println("执行测试命令: echo 'hello'")
	testOutput, err := client.RunCommandWithOutput("echo 'hello'", func(line string) {
		fmt.Printf("输出: %s\n", line)
	})
	if err != nil {
		fmt.Printf("✗ 命令执行失败: %v\n", err)
		// 更新节点状态为离线
		m.mutex.Lock()
		node.Status = NodeStatusOffline
		node.UpdatedAt = time.Now()
		m.updateNodeStatus(id, node.Status, node.UpdatedAt)
		m.mutex.Unlock()
		return false, err
	}

	fmt.Printf("✓ 命令执行成功，输出: %s\n", strings.TrimSpace(testOutput))
	// 更新节点状态为在线
	m.mutex.Lock()
	node.Status = NodeStatusOnline
	node.UpdatedAt = time.Now()
	m.updateNodeStatus(id, node.Status, node.UpdatedAt)
	m.mutex.Unlock()

	fmt.Printf("✓ 节点 %s 连接测试成功，状态更新为在线\n", node.Name)
	return true, nil
}

// DeployNode 部署节点
func (m *SqliteNodeManager) DeployNode(id string) error {
	m.mutex.Lock()
	// 更新节点状态为部署中
	node, err := m.GetNode(id)
	if err != nil {
		m.mutex.Unlock()
		return err
	}

	node.Status = NodeStatusDeploying
	node.UpdatedAt = time.Now()
	m.updateNodeStatus(id, node.Status, node.UpdatedAt)
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
		m.updateNodeStatus(id, node.Status, node.UpdatedAt)
		m.mutex.Unlock()
		return err
	}

	// 更新节点状态为就绪
	m.mutex.Lock()
	node.Status = NodeStatusReady
	node.UpdatedAt = time.Now()
	m.updateNodeStatus(id, node.Status, node.UpdatedAt)
	m.mutex.Unlock()

	return nil
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

	// 定义输出回调函数
	outputCallback := func(line string) {
		fmt.Println(line) // 实时打印到控制台
	}

	// 1. 生成SSH密钥对
	fmt.Printf("=== 配置节点 %s 的SSH设置 ===\n", node.Name)
	fmt.Println("1. 生成SSH密钥对...")
	_, err = client.RunCommandWithOutput("mkdir -p ~/.ssh && chmod 700 ~/.ssh", outputCallback)
	if err != nil {
		return fmt.Errorf("failed to create .ssh directory: %v", err)
	}

	// 生成密钥对，不使用密码
	_, err = client.RunCommandWithOutput("ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N '' -q", outputCallback)
	if err != nil {
		return fmt.Errorf("failed to generate SSH key: %v", err)
	}

	// 设置公钥文件权限
	_, err = client.RunCommandWithOutput("chmod 644 ~/.ssh/id_rsa.pub", outputCallback)
	if err != nil {
		return fmt.Errorf("failed to set public key permissions: %v", err)
	}

	// 设置私钥文件权限
	_, err = client.RunCommandWithOutput("chmod 600 ~/.ssh/id_rsa", outputCallback)
	if err != nil {
		return fmt.Errorf("failed to set private key permissions: %v", err)
	}

	// 2. 配置SSH服务，允许公钥认证
	fmt.Println("2. 配置SSH服务，允许公钥认证...")
	_, err = client.RunCommandWithOutput("sudo sed -i 's/^#PubkeyAuthentication yes/PubkeyAuthentication yes/' /etc/ssh/sshd_config", outputCallback)
	if err != nil {
		return fmt.Errorf("failed to configure SSHD: %v", err)
	}

	// 重启SSH服务
	fmt.Println("3. 重启SSH服务...")
	_, err = client.RunCommandWithOutput("sudo systemctl restart sshd", outputCallback)
	if err != nil {
		// 尝试使用service命令（兼容不同Linux发行版）
		fmt.Println("尝试使用service命令重启SSH服务...")
		_, err = client.RunCommandWithOutput("sudo service ssh restart", outputCallback)
		if err != nil {
			return fmt.Errorf("failed to restart SSH service: %v", err)
		}
	}

	fmt.Printf("=== 节点 %s 的SSH设置配置完成 ===\n", node.Name)
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

	// 定义输出回调函数
	outputCallback := func(line string) {
		fmt.Println(line) // 实时打印到控制台
	}

	// 1. 收集所有节点的公钥
	fmt.Println("=== 收集所有节点的公钥 ===")
	nodePublicKeys := make(map[string]string)

	for _, node := range allNodes {
		fmt.Printf("处理节点: %s (%s)\n", node.Name, node.IP)
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
		fmt.Printf("  获取节点 %s 的公钥...\n", node.Name)
		publicKey, err := client.RunCommandWithOutput("cat ~/.ssh/id_rsa.pub", outputCallback)
		if err != nil {
			// 如果公钥不存在，先配置SSH设置
			client.Close()
			fmt.Printf("  节点 %s 公钥不存在，正在配置SSH设置...\n", node.Name)
			if err := m.ConfigureSSHSettings(node.ID); err != nil {
				return fmt.Errorf("failed to configure SSH settings for node %s: %v", node.Name, err)
			}

			// 重新创建客户端并获取公钥
			client, err = ssh.NewSSHClient(sshConfig)
			if err != nil {
				return fmt.Errorf("failed to re-create SSH client for node %s: %v", node.Name, err)
			}

			publicKey, err = client.RunCommandWithOutput("cat ~/.ssh/id_rsa.pub", outputCallback)
			if err != nil {
				client.Close()
				return fmt.Errorf("failed to get public key for node %s: %v", node.Name, err)
			}
		}

		nodePublicKeys[node.Name] = strings.TrimSpace(publicKey)
		fmt.Printf("  成功获取节点 %s 的公钥\n", node.Name)
		client.Close()
	}

	// 2. 将所有公钥分发到每个节点的authorized_keys文件中
	fmt.Println("\n=== 将所有公钥分发到每个节点的authorized_keys文件中 ===")
	for _, targetNode := range allNodes {
		fmt.Printf("配置节点: %s (%s)\n", targetNode.Name, targetNode.IP)
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
		fmt.Printf("  清空 %s 的authorized_keys文件...\n", targetNode.Name)
		_, err = client.RunCommandWithOutput("> ~/.ssh/authorized_keys", outputCallback)
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to clear authorized_keys for node %s: %v", targetNode.Name, err)
		}

		// 添加所有节点的公钥到authorized_keys文件
		for nodeName, publicKey := range nodePublicKeys {
			// 添加公钥到authorized_keys文件，包括自己的
			fmt.Printf("  添加节点 %s 的公钥到 %s...\n", nodeName, targetNode.Name)
			cmd := fmt.Sprintf("echo '%s' >> ~/.ssh/authorized_keys", publicKey)
			_, err = client.RunCommandWithOutput(cmd, outputCallback)
			if err != nil {
				client.Close()
				return fmt.Errorf("failed to add public key for node %s to %s: %v", nodeName, targetNode.Name, err)
			}
		}

		// 设置authorized_keys文件权限
		fmt.Printf("  设置 %s 的authorized_keys文件权限...\n", targetNode.Name)
		_, err = client.RunCommandWithOutput("chmod 600 ~/.ssh/authorized_keys", outputCallback)
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to set authorized_keys permissions for node %s: %v", targetNode.Name, err)
		}

		fmt.Printf("  成功配置节点 %s 的SSH免密访问\n", targetNode.Name)
		client.Close()
	}

	// 3. 测试节点之间的免密连接
	fmt.Println("\n=== 测试节点之间的免密连接 ===")
	for i, sourceNode := range allNodes {
		for j, targetNode := range allNodes {
			// 跳过自己
			if i == j {
				continue
			}

			fmt.Printf("测试从 %s 到 %s 的免密连接...\n", sourceNode.Name, targetNode.Name)
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
			_, err = client.RunCommandWithOutput(testCmd, outputCallback)
			client.Close()

			if err != nil {
				return fmt.Errorf("SSH passwdless test failed from %s to %s: %v", sourceNode.Name, targetNode.Name, err)
			} else {
				fmt.Printf("✓ 从 %s 到 %s 的免密连接测试成功\n", sourceNode.Name, targetNode.Name)
			}
		}
	}

	fmt.Println("\n=== 所有节点之间的SSH免密互通配置完成 ===")
	return nil
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

// formatRegistryMirrors 格式化镜像加速地址为JSON数组
func formatRegMirrors(mirrors []string) string {
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

// deployMasterNode 部署主节点
func (m *SqliteNodeManager) deployMasterNode(client *ssh.SSHClient) error {
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

	// 2. 从脚本管理器获取系统准备脚本
	var systemPrepCmd string
	var systemPrepFound bool
	if m.scriptManager != nil {
		if scriptGetter, ok := m.scriptManager.(interface {
			GetScript(name string) (string, bool)
		}); ok {
			// 尝试获取特定发行版的系统准备脚本，使用与前端一致的命名格式
			stepName := strings.ReplaceAll(strings.ToLower("系统准备"), " ", "_")
			systemPrepScriptName := fmt.Sprintf("%s_%s", distro, stepName)
			if script, scriptFound := scriptGetter.GetScript(systemPrepScriptName); scriptFound {
				systemPrepCmd = script
				systemPrepFound = true
				fmt.Printf("Using custom system prep script for %s\n", distro)
			} else {
				// 尝试获取通用系统准备脚本
				if script, scriptFound := scriptGetter.GetScript("system_prep"); scriptFound {
					systemPrepCmd = script
					systemPrepFound = true
					fmt.Printf("Using custom system prep script\n")
				}
			}
		}
	}

	// 执行系统准备脚本（无论是否是自定义脚本）
	fmt.Println("=== 执行系统准备脚本 ===")
	if systemPrepFound {
		systemPrepOutput, err := client.RunCommandWithOutput(systemPrepCmd, func(line string) {
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			fmt.Printf("系统准备脚本执行出现错误: %v\n输出: %s\n", err, systemPrepOutput)
			fmt.Println("警告: 系统准备脚本执行失败，但将继续尝试IP转发配置...")
			// 不返回错误，继续执行IP转发配置
		} else {
			fmt.Println("系统准备脚本执行成功")
		}
	} else {
		// 3. 禁用swap
		fmt.Println("\n=== 执行禁用swap操作 ===")
		swapCmd := `
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
`
		swapOutput, err := client.RunCommandWithOutput(swapCmd, func(line string) {
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			fmt.Printf("禁用swap操作失败: %v\n输出: %s\n", err, swapOutput)
			fmt.Println("警告: 禁用swap操作失败，但将继续执行...")
			// 不返回错误，继续执行
		} else {
			fmt.Println("禁用swap操作成功")
		}

		// 4. 设置内核参数（生产环境推荐配置）
		fmt.Println("\n=== 执行内核参数配置 ===")
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
sudo sysctl --system
sudo modprobe br_netfilter
sudo modprobe overlay
`
		kernelOutput, err := client.RunCommandWithOutput(kernelCmd, func(line string) {
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			fmt.Printf("内核参数配置失败: %v\n输出: %s\n", err, kernelOutput)
			fmt.Println("警告: 内核参数配置失败，但将继续执行...")
			// 不返回错误，继续执行
		} else {
			fmt.Println("内核参数配置成功")
		}
	}

	// 添加延迟，确保系统准备完全执行
	fmt.Println("\n=== 等待5秒，确保系统准备完全执行 ===")
	if _, err := client.RunCommand("sleep 5"); err != nil {
		fmt.Printf("等待命令执行失败: %v\n", err)
	}

	// 确保IP转发配置被正确设置，即使系统准备脚本中已有配置，再单独执行一次确保生效
	fmt.Println("\n=== 执行IP转发配置脚本 ===")
	ensureIpForwardCmd := `
# 1. 确保/etc/sysctl.d目录存在
echo "=== 确保配置目录存在 ==="
sudo mkdir -p /etc/sysctl.d

# 2. 写入IP转发配置文件，使用bash -c确保权限
echo "1. 正在配置IP转发..."
sudo bash -c 'cat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf
net.ipv4.ip_forward = 1
EOF'

# 3. 验证IP转发配置文件是否生成，失败则重试
echo "2. 验证IP转发配置文件是否生成..."
for i in {1..3}; do
    if [ -f /etc/sysctl.d/99-kubernetes-ipforward.conf ]; then
        echo "✓ 配置文件已生成，内容为:"
        sudo cat /etc/sysctl.d/99-kubernetes-ipforward.conf
        break
    else
        echo "✗ 配置文件未生成，正在重试 ($i/3)..."
        sudo bash -c 'cat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf
net.ipv4.ip_forward = 1
EOF'
        sleep 1
    fi
done

# 4. 写入其他Kubernetes所需内核参数配置文件
echo "3. 正在配置其他Kubernetes内核参数..."
sudo bash -c 'cat <<EOF > /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF'

# 5. 验证其他内核参数配置文件是否生成，失败则重试
echo "4. 验证其他内核参数配置文件是否生成..."
for i in {1..3}; do
    if [ -f /etc/sysctl.d/k8s.conf ]; then
        echo "✓ 配置文件已生成，内容为:"
        sudo cat /etc/sysctl.d/k8s.conf
        break
    else
        echo "✗ 配置文件未生成，正在重试 ($i/3)..."
        sudo bash -c 'cat <<EOF > /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF'
        sleep 1
    fi
done

# 6. 设置配置文件权限，确保系统可以读取
echo "5. 设置配置文件权限..."
sudo chmod 644 /etc/sysctl.d/99-kubernetes-ipforward.conf
sudo chmod 644 /etc/sysctl.d/k8s.conf

# 7. 加载必要的内核模块
echo "6. 正在加载内核模块..."
sudo modprobe overlay || echo "overlay模块已加载或加载失败"
sudo modprobe br_netfilter || echo "br_netfilter模块已加载或加载失败"

# 8. 应用所有内核参数
echo "7. 正在应用内核参数..."
sudo sysctl --system

# 9. 立即设置IP转发值，确保即时生效
echo "8. 确保IP转发即时生效..."
sudo sysctl -w net.ipv4.ip_forward=1
sudo sysctl -w net.bridge.bridge-nf-call-iptables=1
sudo sysctl -w net.bridge.bridge-nf-call-ip6tables=1

# 10. 直接写入/proc/sys/net/ipv4/ip_forward文件确保立即生效
echo "9. 直接写入/proc/sys/net/ipv4/ip_forward文件确保立即生效..."
sudo bash -c 'echo 1 > /proc/sys/net/ipv4/ip_forward'
echo "直接写入文件后，内容为: $(cat /proc/sys/net/ipv4/ip_forward)"

# 11. 等待1秒，确保设置生效
sleep 1

# 12. 验证内核参数设置
echo "10. 最终验证内核参数..."
sudo sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward

# 13. 检查/proc/sys/net/ipv4/ip_forward文件内容
echo "11. 检查/proc/sys/net/ipv4/ip_forward文件内容..."
cat /proc/sys/net/ipv4/ip_forward

# 14. 验证文件权限
echo "12. 验证配置文件权限..."
sudo ls -la /etc/sysctl.d/99-kubernetes-ipforward.conf /etc/sysctl.d/k8s.conf 2>/dev/null || echo "配置文件可能未生成"

# 15. 列出/etc/sysctl.d目录下的所有配置文件，确认文件已生成
echo "13. 列出/etc/sysctl.d目录下的所有配置文件..."
sudo ls -la /etc/sysctl.d/

# 16. 立即验证IP转发是否生效
echo "7. 验证IP转发状态..."
current_ip_forward=$(sudo sysctl -n net.ipv4.ip_forward)
echo "当前IP转发值: $current_ip_forward"

# 如果IP转发未启用，直接通过sysctl命令设置
echo "8. 确保IP转发即时生效..."
sudo sysctl -w net.ipv4.ip_forward=1
sudo sysctl -w net.bridge.bridge-nf-call-iptables=1
sudo sysctl -w net.bridge.bridge-nf-call-ip6tables=1

# 再次验证最终状态
echo "9. 最终验证IP转发状态..."
final_ip_forward=$(sudo sysctl -n net.ipv4.ip_forward)
echo "最终IP转发值: $final_ip_forward"

# 检查/proc/sys/net/ipv4/ip_forward文件内容
echo "10. 检查/proc/sys/net/ipv4/ip_forward文件内容..."
cat /proc/sys/net/ipv4/ip_forward
`
	ensureIpForwardOutput, err := client.RunCommandWithOutput(ensureIpForwardCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("IP转发配置脚本执行出现错误: %v\n输出: %s\n", err, ensureIpForwardOutput)
		fmt.Println("警告: IP转发配置脚本执行失败，但将继续执行...")
		// 不返回错误，继续执行
	} else {
		fmt.Println("IP转发配置脚本执行成功")
	}

	// 添加延迟，确保IP转发配置完全生效
	fmt.Println("\n=== 等待3秒，确保IP转发配置完全生效 ===")
	if _, err := client.RunCommand("sleep 3"); err != nil {
		fmt.Printf("等待命令执行失败: %v\n", err)
	}

	// 最终验证IP转发状态
	fmt.Println("\n=== 最终验证IP转发状态 ===")
	finalCheckCmd := `
# 最终验证IP转发状态
final_ip_forward=$(sudo sysctl -n net.ipv4.ip_forward)
echo "最终IP转发值: $final_ip_forward"

# 检查/proc/sys/net/ipv4/ip_forward文件内容
echo "=== 检查/proc/sys/net/ipv4/ip_forward文件内容 ==="
cat /proc/sys/net/ipv4/ip_forward
`
	finalCheckOutput, err := client.RunCommandWithOutput(finalCheckCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("最终IP转发验证失败: %v\n输出: %s\n", err, finalCheckOutput)
		// 不返回错误，继续执行
	} else {
		fmt.Println("最终IP转发验证完成")
	}

	// 5. 设置容器运行时（默认使用containerd，生产环境推荐）
	containerRuntime := "containerd"
	if err := m.installContainerRuntime(client, distro, containerRuntime); err != nil {
		return err
	}

	// 6. 安装kubeadm, kubelet和kubectl
	if err := m.installKubernetesComponents(client, distro); err != nil {
		return err
	}

	return nil
}

// deployWorkerNode 部署工作节点
func (m *SqliteNodeManager) deployWorkerNode(client *ssh.SSHClient) error {
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

	// 2. 从脚本管理器获取系统准备脚本
	var systemPrepCmd string
	var systemPrepFound bool
	if m.scriptManager != nil {
		if scriptGetter, ok := m.scriptManager.(interface {
			GetScript(name string) (string, bool)
		}); ok {
			// 尝试获取特定发行版的系统准备脚本，使用与前端一致的命名格式
			stepName := strings.ReplaceAll(strings.ToLower("系统准备"), " ", "_")
			systemPrepScriptName := fmt.Sprintf("%s_%s", distro, stepName)
			if script, scriptFound := scriptGetter.GetScript(systemPrepScriptName); scriptFound {
				systemPrepCmd = script
				systemPrepFound = true
				fmt.Printf("Using custom system prep script for %s\n", distro)
			} else {
				// 尝试获取通用系统准备脚本
				if script, scriptFound := scriptGetter.GetScript("system_prep"); scriptFound {
					systemPrepCmd = script
					systemPrepFound = true
					fmt.Printf("Using custom system prep script\n")
				}
			}
		}
	}

	// 执行系统准备脚本（无论是否是自定义脚本）
	fmt.Println("=== 执行系统准备脚本 ===")
	if systemPrepFound {
		systemPrepOutput, err := client.RunCommandWithOutput(systemPrepCmd, func(line string) {
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			fmt.Printf("系统准备脚本执行出现错误: %v\n输出: %s\n", err, systemPrepOutput)
			fmt.Println("警告: 系统准备脚本执行失败，但将继续尝试IP转发配置...")
			// 不返回错误，继续执行IP转发配置
		} else {
			fmt.Println("系统准备脚本执行成功")
		}
	} else {
		// 3. 禁用swap
		fmt.Println("\n=== 执行禁用swap操作 ===")
		swapCmd := `
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
`
		swapOutput, err := client.RunCommandWithOutput(swapCmd, func(line string) {
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			fmt.Printf("禁用swap操作失败: %v\n输出: %s\n", err, swapOutput)
			fmt.Println("警告: 禁用swap操作失败，但将继续执行...")
			// 不返回错误，继续执行
		} else {
			fmt.Println("禁用swap操作成功")
		}

		// 4. 设置内核参数
		fmt.Println("\n=== 执行内核参数配置 ===")
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
sudo sysctl --system
sudo modprobe br_netfilter
sudo modprobe overlay
`
		kernelOutput, err := client.RunCommandWithOutput(kernelCmd, func(line string) {
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			fmt.Printf("内核参数配置失败: %v\n输出: %s\n", err, kernelOutput)
			fmt.Println("警告: 内核参数配置失败，但将继续执行...")
			// 不返回错误，继续执行
		} else {
			fmt.Println("内核参数配置成功")
		}
	}

	// 添加延迟，确保系统准备完全执行
	fmt.Println("\n=== 等待5秒，确保系统准备完全执行 ===")
	if _, err := client.RunCommand("sleep 5"); err != nil {
		fmt.Printf("等待命令执行失败: %v\n", err)
	}

	// 确保IP转发配置被正确设置，即使系统准备脚本中已有配置，再单独执行一次确保生效
	fmt.Println("\n=== 执行IP转发配置脚本 ===")
	ensureIpForwardCmd := `
# 确保IP转发配置文件存在并包含正确的配置，使用sudo确保权限
 echo "1. 正在配置IP转发..."
sudo cat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf
net.ipv4.ip_forward = 1
EOF

# 验证配置文件内容
echo "2. 验证IP转发配置文件..."
sudo cat /etc/sysctl.d/99-kubernetes-ipforward.conf

# 确保其他Kubernetes所需内核参数配置正确，使用sudo确保权限
echo "3. 正在配置其他Kubernetes内核参数..."
sudo cat <<EOF > /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

# 验证其他内核参数配置文件内容
echo "4. 验证其他内核参数配置文件..."
sudo cat /etc/sysctl.d/k8s.conf

# 加载必要的内核模块，使用sudo确保权限
echo "5. 正在加载内核模块..."
sudo modprobe br_netfilter || echo "br_netfilter模块加载失败或已加载"
sudo modprobe overlay || echo "overlay模块加载失败或已加载"

# 应用所有内核参数，使用sudo确保权限
echo "6. 正在应用内核参数..."
sudo sysctl --system

# 立即验证IP转发是否生效
echo "7. 验证IP转发状态..."
current_ip_forward=$(sudo sysctl -n net.ipv4.ip_forward)
echo "当前IP转发值: $current_ip_forward"

# 如果IP转发未启用，直接通过sysctl命令设置
echo "8. 确保IP转发即时生效..."
sudo sysctl -w net.ipv4.ip_forward=1
sudo sysctl -w net.bridge.bridge-nf-call-iptables=1
sudo sysctl -w net.bridge.bridge-nf-call-ip6tables=1

# 再次验证最终状态
echo "9. 最终验证IP转发状态..."
final_ip_forward=$(sudo sysctl -n net.ipv4.ip_forward)
echo "最终IP转发值: $final_ip_forward"

# 检查/proc/sys/net/ipv4/ip_forward文件内容
echo "10. 检查/proc/sys/net/ipv4/ip_forward文件内容..."
cat /proc/sys/net/ipv4/ip_forward
`
	ensureIpForwardOutput, err := client.RunCommandWithOutput(ensureIpForwardCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("IP转发配置脚本执行出现错误: %v\n输出: %s\n", err, ensureIpForwardOutput)
		fmt.Println("警告: IP转发配置脚本执行失败，但将继续执行...")
		// 不返回错误，继续执行
	} else {
		fmt.Println("IP转发配置脚本执行成功")
	}

	// 添加延迟，确保IP转发配置完全生效
	fmt.Println("\n=== 等待3秒，确保IP转发配置完全生效 ===")
	if _, err := client.RunCommand("sleep 3"); err != nil {
		fmt.Printf("等待命令执行失败: %v\n", err)
	}

	// 最终验证IP转发状态
	fmt.Println("\n=== 最终验证IP转发状态 ===")
	finalCheckCmd := `
# 最终验证IP转发状态
final_ip_forward=$(sudo sysctl -n net.ipv4.ip_forward)
echo "最终IP转发值: $final_ip_forward"

# 检查/proc/sys/net/ipv4/ip_forward文件内容
echo "=== 检查/proc/sys/net/ipv4/ip_forward文件内容 ==="
cat /proc/sys/net/ipv4/ip_forward
`
	finalCheckOutput, err := client.RunCommandWithOutput(finalCheckCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("最终IP转发验证失败: %v\n输出: %s\n", err, finalCheckOutput)
		// 不返回错误，继续执行
	} else {
		fmt.Println("最终IP转发验证完成")
	}

	// 5. 设置容器运行时
	containerRuntime := "containerd"
	if err := m.installContainerRuntime(client, distro, containerRuntime); err != nil {
		return err
	}

	// 6. 安装kubeadm和kubelet
	if err := m.installKubernetesComponents(client, distro); err != nil {
		return err
	}

	return nil
}

// installContainerRuntime 安装容器运行时
func (m *SqliteNodeManager) installContainerRuntime(client *ssh.SSHClient, distro, runtime string) error {
	var cmd string
	var found bool

	// 从脚本管理器获取容器运行时安装脚本，使用与前端一致的命名格式
	if m.scriptManager != nil {
		if scriptGetter, ok := m.scriptManager.(interface {
			GetScript(name string) (string, bool)
		}); ok {
			// 使用与前端完全一致的脚本命名格式: ${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}
			// 步骤名称："安装容器运行时"
			stepName := strings.ReplaceAll(strings.ToLower("安装容器运行时"), " ", "_")
			runtimeScriptName := fmt.Sprintf("%s_%s", distro, stepName)
			if script, scriptFound := scriptGetter.GetScript(runtimeScriptName); scriptFound {
				cmd = script
				found = true
				fmt.Printf("Using custom script for container runtime installation on %s: %s\n", distro, runtimeScriptName)
			} else {
				// 尝试旧格式的脚本名作为备选
				runtimeScriptNameOld := fmt.Sprintf("%s_%s", runtime, distro)
				if script, scriptFound := scriptGetter.GetScript(runtimeScriptNameOld); scriptFound {
					cmd = script
					found = true
					fmt.Printf("Using old format custom script for container runtime installation: %s\n", runtimeScriptNameOld)
				} else {
					// 尝试获取特定运行时的通用脚本
					if script, scriptFound := scriptGetter.GetScript(runtime); scriptFound {
						cmd = script
						found = true
						fmt.Printf("Using custom script for container runtime installation\n", runtime)
					}
				}
			}
		}
	}

	// 如果没有找到自定义脚本，使用默认命令
	if !found {
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
				cmd = `
				apt-get update && apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release
				mkdir -p /etc/apt/keyrings
				curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
				echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
				apt-get update
				apt-get install -y docker-ce docker-ce-cli containerd.io
				systemctl restart docker
				systemctl enable docker
				`
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
				cmd = `
				yum install -y yum-utils
				yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
				yum install -y docker-ce docker-ce-cli containerd.io
				systemctl restart docker
				systemctl enable docker
				`
			}
		default:
			return fmt.Errorf("unsupported distribution: %s", distro)
		}
	}

	if _, err := client.RunCommand(cmd); err != nil {
		return err
	}

	return nil
}

// InstallKubernetesComponents 安装Kubernetes组件（公开方法，实现NodeManager接口）
func (m *SqliteNodeManager) InstallKubernetesComponents(id string, kubeadmVersion string) error {
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
func (m *SqliteNodeManager) installKubernetesComponents(client *ssh.SSHClient, distro string) error {
	var addRepoCmd string
	var installComponentsCmd string
	var found bool

	// 从脚本管理器获取脚本
	if m.scriptManager != nil {
		if scriptGetter, ok := m.scriptManager.(interface {
			GetScript(name string) (string, bool)
		}); ok {
			// 1. 先添加Kubernetes仓库
			// 使用与前端完全一致的脚本命名格式: ${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}
			// 步骤名称："添加Kubernetes仓库"
			addRepoStepName := "添加kubernetes仓库"
			// 将步骤名称转换为小写并替换所有空格为下划线（与前端保持一致）
			formattedAddRepoStepName := strings.ReplaceAll(strings.ToLower(addRepoStepName), " ", "_")
			addRepoScriptName := fmt.Sprintf("%s_%s", distro, formattedAddRepoStepName)
			if script, scriptFound := scriptGetter.GetScript(addRepoScriptName); scriptFound {
				addRepoCmd = script
				fmt.Printf("Using custom script for adding Kubernetes repository on %s: %s\n", distro, addRepoScriptName)
			} else {
				// 尝试旧格式的脚本名作为备选
				oldAddRepoScriptName := fmt.Sprintf("%s_添加kubernetes仓库", distro)
				if script, scriptFound := scriptGetter.GetScript(oldAddRepoScriptName); scriptFound {
					addRepoCmd = script
					fmt.Printf("Using old format custom script for adding Kubernetes repository: %s\n", oldAddRepoScriptName)
				} else {
					fmt.Printf("No custom script found for adding Kubernetes repository on %s\n", distro)
				}
			}

			// 2. 然后安装Kubernetes组件
			// 使用与前端完全一致的脚本命名格式: ${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}
			// 步骤名称："安装Kubernetes组件"
			installComponentsStepName := "安装kubernetes组件"
			// 将步骤名称转换为小写并替换所有空格为下划线（与前端保持一致）
			formattedInstallComponentsStepName := strings.ReplaceAll(strings.ToLower(installComponentsStepName), " ", "_")
			installComponentsScriptName := fmt.Sprintf("%s_%s", distro, formattedInstallComponentsStepName)
			if script, scriptFound := scriptGetter.GetScript(installComponentsScriptName); scriptFound {
				installComponentsCmd = script
				found = true
				fmt.Printf("Using custom script for Kubernetes components installation on %s: %s\n", distro, installComponentsScriptName)
			} else {
				// 尝试旧格式的脚本名作为备选
				oldInstallComponentsScriptName := fmt.Sprintf("%s_安装kubernetes组件", distro)
				if script, scriptFound := scriptGetter.GetScript(oldInstallComponentsScriptName); scriptFound {
					installComponentsCmd = script
					found = true
					fmt.Printf("Using old format custom script for Kubernetes components installation: %s\n", oldInstallComponentsScriptName)
				} else {
					fmt.Printf("No custom script found for Kubernetes components installation on %s\n", distro)
				}
			}
		}
	}

	// 合并命令
	fullCmd := ""
	if addRepoCmd != "" {
		fullCmd += addRepoCmd + "\n"
	}

	// 如果没有找到自定义安装组件脚本，使用默认命令
	if !found {
		switch distro {
		case "ubuntu", "debian":
			if addRepoCmd == "" {
				// 没有自定义添加仓库脚本，使用默认添加仓库命令
				fullCmd += `
				apt-get update
				apt-get install -y apt-transport-https ca-certificates curl gpg
				
				// 创建keyring目录
				mkdir -p -m 755 /etc/apt/keyrings
				
				// 下载并安装GPG密钥
				curl -fsSL -L https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
				
				// 添加Kubernetes仓库
				echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list
				
				// 更新仓库缓存
				apt-get update
				`
			}
			// 使用默认安装组件命令
			fullCmd += `
			apt-get install -y kubelet kubeadm kubectl
			systemctl enable --now kubelet
			`
		case "centos", "rhel", "rocky", "almalinux":
			if addRepoCmd == "" {
				// 没有自定义添加仓库脚本，使用默认添加仓库命令
				fullCmd += `
				// 清理旧的Kubernetes仓库配置
			rm -f /etc/yum.repos.d/kubernetes.repo
		rm -f /etc/yum.repos.d/packages.cloud.google.com_yum_repos_kubernetes-el7-x86_64.repo
				
				// 添加Kubernetes仓库
				cat <<EOF > /etc/yum.repos.d/kubernetes.repo
				[kubernetes]
				name=Kubernetes
				baseurl=https://pkgs.k8s.io/core:/stable:/v1.30/rpm/
				enabled=1
				gpgcheck=1
				gpgkey=https://pkgs.k8s.io/core:/stable:/v1.30/rpm/repodata/repomd.xml.key
				exclude=kubelet kubeadm kubectl
				EOF
				
				// 更新仓库缓存
				if command -v dnf &> /dev/null; then
					dnf makecache -y
				else
					yum makecache -y
				fi
				`
			}
			// 使用默认安装组件命令
			fullCmd += `
			// 安装Kubernetes组件
			if command -v dnf &> /dev/null; then
				dnf install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
			else
				yum install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
			fi
			
			// 启动kubelet
			systemctl enable --now kubelet
			`
		default:
			return fmt.Errorf("unsupported distribution: %s", distro)
		}
	} else {
		// 使用自定义安装组件脚本
		fullCmd += installComponentsCmd
	}

	// 执行完整的Kubernetes组件安装命令并实时输出
	_, err := client.RunCommandWithOutput(fullCmd, func(line string) {
		// 实时打印到控制台，便于调试和监控
		fmt.Println(line)
	})

	if err != nil {
		return err
	}

	return nil
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
func (m *SqliteNodeManager) CreateLog(logEntry log.LogEntry) error {
	return m.logManager.CreateLog(logEntry)
}
