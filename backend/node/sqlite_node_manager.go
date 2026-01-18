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

// GetLogManager 获取日志管理器
func (m *SqliteNodeManager) GetLogManager() log.LogManager {
	return m.logManager
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
		os TEXT NOT NULL DEFAULT 'unknown',
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

	rows, err := m.db.Query("SELECT id, name, ip, port, username, password, private_key, node_type, status, os, created_at, updated_at FROM nodes")
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
			&node.OS,
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
		"SELECT id, name, ip, port, username, password, private_key, node_type, status, os, created_at, updated_at FROM nodes WHERE id = ?",
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
		&node.OS,
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

	// 设置默认操作系统类型
	if node.OS == "" {
		node.OS = "unknown"
	}

	// 插入数据
	_, err := m.db.Exec(
		"INSERT INTO nodes (id, name, ip, port, username, password, private_key, node_type, status, os, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		node.ID,
		node.Name,
		node.IP,
		node.Port,
		node.Username,
		node.Password,
		node.PrivateKey,
		node.NodeType,
		node.Status,
		node.OS,
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

	// 设置默认操作系统类型
	if node.OS == "" {
		node.OS = "unknown"
	}

	_, err = m.db.Exec(
		"UPDATE nodes SET name = ?, ip = ?, port = ?, username = ?, password = ?, private_key = ?, node_type = ?, status = ?, os = ?, updated_at = ? WHERE id = ?",
		node.Name,
		node.IP,
		node.Port,
		node.Username,
		node.Password,
		node.PrivateKey,
		node.NodeType,
		node.Status,
		node.OS,
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

	// 检测操作系统类型
	fmt.Println("检测操作系统类型...")
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
	fmt.Printf("✓ 操作系统检测成功: %s\n", osType)

	// 更新节点状态为在线并保存操作系统类型
	m.mutex.Lock()
	node.Status = NodeStatusOnline
	node.OS = osType
	node.UpdatedAt = time.Now()
	m.updateNodeStatus(id, node.Status, node.UpdatedAt)
	// 更新节点OS字段到数据库
	_, err = m.db.Exec("UPDATE nodes SET os = ?, updated_at = ? WHERE id = ?", osType, node.UpdatedAt, id)
	if err != nil {
		fmt.Printf("✗ 更新节点OS信息到数据库失败: %v\n", err)
	}
	m.mutex.Unlock()

	fmt.Printf("✓ 节点 %s 连接测试成功，状态更新为在线，操作系统: %s\n", node.Name, osType)
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
		err = m.deployMasterNode(client, node.ID, node.Name)
	} else {
		// 执行工作节点部署命令
		err = m.deployWorkerNode(client, node.ID, node.Name)
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

	// 检查并删除已存在的密钥文件，避免覆盖确认提示
	fmt.Println("  检查并删除已存在的密钥文件...")
	_, err = client.RunCommandWithOutput("rm -f ~/.ssh/id_rsa ~/.ssh/id_rsa.pub", outputCallback)
	if err != nil {
		return fmt.Errorf("failed to remove existing SSH keys: %v", err)
	}

	// 生成密钥对，不使用密码
	fmt.Println("  生成新的SSH密钥对...")
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

	fmt.Println("=== 开始配置所有节点之间的SSH免密互通 ===")

	// 1. 确保所有节点都已配置SSH密钥
	fmt.Println("\n=== 1. 确保所有节点都已配置SSH密钥 ===")
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

		// 检查公钥是否存在，不存在则配置SSH设置
		_, err = client.RunCommand("test -f ~/.ssh/id_rsa.pub")
		if err != nil {
			client.Close()
			fmt.Printf("  节点 %s 公钥不存在，正在配置SSH设置...\n", node.Name)
			if err := m.ConfigureSSHSettings(node.ID); err != nil {
				return fmt.Errorf("failed to configure SSH settings for node %s: %v", node.Name, err)
			}
			fmt.Printf("  节点 %s SSH设置配置完成\n", node.Name)
		} else {
			fmt.Printf("  节点 %s 已存在SSH密钥\n", node.Name)
		}
		client.Close()
	}

	// 2. 收集所有节点的公钥
	fmt.Println("\n=== 2. 收集所有节点的公钥 ===")
	nodePublicKeys := make(map[string]string)
	nodeIPMap := make(map[string]string) // 节点名称到IP的映射

	for _, node := range allNodes {
		fmt.Printf("获取节点 %s (%s) 的公钥...\n", node.Name, node.IP)
		nodeIPMap[node.Name] = node.IP

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
		client.Close()
		if err != nil {
			return fmt.Errorf("failed to get public key for node %s: %v", node.Name, err)
		}

		nodePublicKeys[node.Name] = strings.TrimSpace(publicKey)
		fmt.Printf("  成功获取节点 %s 的公钥\n", node.Name)
	}

	// 3. 配置每个节点的authorized_keys文件
	fmt.Println("\n=== 3. 配置每个节点的authorized_keys文件 ===")
	for _, targetNode := range allNodes {
		fmt.Printf("\n配置节点: %s (%s)\n", targetNode.Name, targetNode.IP)
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

		// 确保.ssh目录存在并设置正确权限
		fmt.Printf("  1. 设置.ssh目录权限...\n")
		permCmd := "mkdir -p ~/.ssh && chmod 755 ~ && chmod 700 ~/.ssh"
		_, err = client.RunCommandWithOutput(permCmd, outputCallback)
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to set .ssh directory permissions for node %s: %v", targetNode.Name, err)
		}

		// 清空并重新创建authorized_keys文件
		fmt.Printf("  2. 重新创建authorized_keys文件...\n")
		_, err = client.RunCommandWithOutput("rm -f ~/.ssh/authorized_keys && touch ~/.ssh/authorized_keys", outputCallback)
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to recreate authorized_keys for node %s: %v", targetNode.Name, err)
		}

		// 添加所有节点的公钥到authorized_keys文件
		fmt.Printf("  3. 添加所有节点的公钥...\n")
		for nodeName, publicKey := range nodePublicKeys {
			fmt.Printf("    添加节点 %s 的公钥...\n", nodeName)
			// 使用echo命令添加公钥，确保格式正确
			cmd := fmt.Sprintf("echo '%s' >> ~/.ssh/authorized_keys", publicKey)
			_, err = client.RunCommand(cmd)
			if err != nil {
				client.Close()
				return fmt.Errorf("failed to add public key for node %s to %s: %v", nodeName, targetNode.Name, err)
			}
		}

		// 设置authorized_keys文件权限
		fmt.Printf("  4. 设置authorized_keys文件权限...\n")
		_, err = client.RunCommandWithOutput("chmod 600 ~/.ssh/authorized_keys", outputCallback)
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to set authorized_keys permissions for node %s: %v", targetNode.Name, err)
		}

		// 验证authorized_keys文件内容
		fmt.Printf("  5. 验证authorized_keys文件...\n")
		verifyCmd := "echo '=== authorized_keys内容 ===' && wc -l ~/.ssh/authorized_keys && echo '=== 内容结束 ==='"
		_, err = client.RunCommandWithOutput(verifyCmd, outputCallback)
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to verify authorized_keys content for node %s: %v", targetNode.Name, err)
		}

		fmt.Printf("  ✓ 节点 %s 配置完成\n", targetNode.Name)
		client.Close()
	}

	// 4. 测试节点之间的免密连接
	fmt.Println("\n=== 4. 测试节点之间的免密连接 ===")
	testSuccessCount := 0
	testTotalCount := 0

	for i, sourceNode := range allNodes {
		for j, targetNode := range allNodes {
			// 跳过自己
			if i == j {
				continue
			}

			testTotalCount++
			fmt.Printf("\n测试从 %s 到 %s 的免密连接...\n", sourceNode.Name, targetNode.Name)

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
				fmt.Printf("  ✗ 创建SSH客户端失败: %v\n", err)
				continue
			}

			// 测试免密连接，使用简单的测试命令
			testCmd := fmt.Sprintf(
				"ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 %s@%s 'echo success'",
				targetNode.Username, targetNode.IP,
			)

			output, err := client.RunCommand(testCmd)
			client.Close()

			if err != nil {
				fmt.Printf("  ✗ 免密连接测试失败\n")
				fmt.Printf("    错误: %v\n", err)
			} else {
				if strings.TrimSpace(output) == "success" {
					fmt.Printf("  ✓ 免密连接测试成功\n")
					testSuccessCount++
				} else {
					fmt.Printf("  ✗ 免密连接测试失败，输出不符合预期: %s\n", output)
				}
			}
		}
	}

	// 5. 输出测试结果
	fmt.Println("\n=== 5. SSH免密互通配置结果 ===")
	fmt.Printf("测试总数: %d\n", testTotalCount)
	fmt.Printf("成功数量: %d\n", testSuccessCount)
	fmt.Printf("失败数量: %d\n", testTotalCount-testSuccessCount)

	if testSuccessCount == testTotalCount {
		fmt.Println("\n✓ 所有节点之间的SSH免密互通配置成功！")
	} else {
		fmt.Printf("\n⚠️  部分节点之间的免密连接测试失败，成功率: %.2f%%\n", float64(testSuccessCount)/float64(testTotalCount)*100)
		fmt.Println("建议检查失败节点的网络连接、SSH配置和公钥配置")
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
func (m *SqliteNodeManager) deployMasterNode(client *ssh.SSHClient, nodeID, nodeName string) error {
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
func (m *SqliteNodeManager) deployWorkerNode(client *ssh.SSHClient, nodeID, nodeName string) error {
	// 部署流程：
	// 1. 环境检查 → 2. 操作系统检测 → 3. 系统准备 → 4. IP转发配置 → 5. 容器运行时安装 → 6. Kubernetes组件安装 → 7. 部署完成验证

	// 记录部署开始日志
	if m.logManager != nil {
		startLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "部署开始",
			Output:    "开始执行Kubernetes工作节点部署流程",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(startLog)
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("=== Kubernetes工作节点部署流程开始 ===")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	// 1. 部署前环境检查
	fmt.Println("=== 步骤1: 部署前环境检查 ===")
	if m.logManager != nil {
		stepLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "环境检查",
			Output:    "执行部署前环境检查",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(stepLog)
	}
	envCheckCmd := `
	# 检查操作系统版本
	echo "1. 检查操作系统版本..."
	if [ -f /etc/os-release ]; then
		. /etc/os-release
		echo "操作系统: $PRETTY_NAME"
	elif [ -f /etc/centos-release ]; then
		echo "操作系统: $(cat /etc/centos-release)"
	elif [ -f /etc/redhat-release ]; then
		echo "操作系统: $(cat /etc/redhat-release)"
	else
		echo "操作系统: 未知"
	fi

	# 检查内核版本
	echo -e "\n2. 检查内核版本..."
	kernel_version=$(uname -r)
	echo "内核版本: $kernel_version"

	# 检查CPU核心数
	echo -e "\n3. 检查CPU核心数..."
	cpu_cores=$(nproc)
	echo "CPU核心数: $cpu_cores"
	if [ "$cpu_cores" -lt 2 ]; then
		echo "警告: CPU核心数少于2，可能影响Kubernetes性能"
	fi

	# 检查内存大小
	echo -e "\n4. 检查内存大小..."
	mem_total=$(grep MemTotal /proc/meminfo | awk '{print $2}')
	mem_gb=$(echo "scale=2; $mem_total / 1024 / 1024" | bc)
	echo "内存大小: ${mem_gb}GB"
	if (( $(echo "$mem_gb < 2.0" | bc -l) )); then
		echo "警告: 内存大小少于2GB，可能影响Kubernetes性能"
	fi

	# 检查网络连接
	echo -e "\n5. 检查网络连接..."
	if ping -c 1 8.8.8.8 > /dev/null 2>&1; then
		echo "网络连接: 正常"
	else
		echo "警告: 网络连接异常，可能影响软件安装"
	fi
	`
	envCheckOutput, err := client.RunCommandWithOutput(envCheckCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("环境检查执行出现错误: %v\n输出: %s\n", err, envCheckOutput)
		fmt.Println("警告: 环境检查执行失败，但将继续执行部署...")
	} else {
		fmt.Println("环境检查完成")
	}

	// 2. 检测操作系统类型
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("=== 步骤2: 检测操作系统类型 ===")
	if m.logManager != nil {
		stepLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "操作系统检测",
			Output:    "检测操作系统类型",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(stepLog)
	}
	distroCmd := `
	if [ -f /etc/os-release ]; then
		. /etc/os-release
		echo $ID
	elif [ -f /etc/centos-release ]; then
		echo "centos"
	elif [ -f /etc/redhat-release ]; then
		echo "rhel"
	elif [ -f /etc/rocky-release ]; then
		echo "rocky"
	elif [ -f /etc/almalinux-release ]; then
		echo "almalinux"
	elif [ -f /etc/debian_version ]; then
		echo "debian"
	else
		echo "unknown"
	fi
	`
	distroOutput, err := client.RunCommand(distroCmd)
	if err != nil {
		return fmt.Errorf("检测操作系统类型失败: %v", err)
	}
	distro := strings.TrimSpace(distroOutput)

	if distro == "unknown" {
		return fmt.Errorf("无法识别的操作系统类型，不支持部署Kubernetes工作节点")
	}

	fmt.Printf("操作系统类型: %s\n", distro)

	// 3. 系统准备
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("=== 步骤3: 系统准备 ===")
	if m.logManager != nil {
		stepLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "系统准备",
			Output:    "执行系统准备操作",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(stepLog)
	}

	// 从脚本管理器获取系统准备脚本
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
				fmt.Printf("使用自定义系统准备脚本: %s\n", systemPrepScriptName)
			} else {
				// 尝试获取通用系统准备脚本
				if script, scriptFound := scriptGetter.GetScript("system_prep"); scriptFound {
					systemPrepCmd = script
					systemPrepFound = true
					fmt.Println("使用通用系统准备脚本")
				}
			}
		}
	}

	// 执行系统准备脚本（无论是否是自定义脚本）
	if systemPrepFound {
		systemPrepOutput, err := client.RunCommandWithOutput(systemPrepCmd, func(line string) {
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			fmt.Printf("自定义系统准备脚本执行失败: %v\n输出: %s\n", err, systemPrepOutput)
			return fmt.Errorf("系统准备失败: %v", err)
		} else {
			fmt.Println("系统准备脚本执行成功")
		}
	} else {
		// 执行默认系统准备操作
		fmt.Println("使用默认系统准备操作")

		// 3.1 禁用swap
		fmt.Println("\n3.1 执行禁用swap操作...")
		swapCmd := `
		sudo swapoff -a
		sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
		`
		swapOutput, err := client.RunCommandWithOutput(swapCmd, func(line string) {
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			fmt.Printf("禁用swap操作失败: %v\n输出: %s\n", err, swapOutput)
			return fmt.Errorf("禁用swap失败: %v", err)
		} else {
			fmt.Println("禁用swap操作成功")
		}

		// 3.2 设置内核参数
		fmt.Println("\n3.2 执行内核参数配置...")
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
			return fmt.Errorf("内核参数配置失败: %v", err)
		} else {
			fmt.Println("内核参数配置成功")
		}
	}

	// 3.3 验证系统准备结果
	fmt.Println("\n3.3 验证系统准备结果...")
	sysPrepVerifyCmd := `
	# 验证swap是否已禁用
	echo "1. 验证swap是否已禁用..."
	swap_status=$(swapon --show | wc -l)
	if [ "$swap_status" -eq 0 ]; then
		echo "✓ Swap已禁用"
	else
		echo "✗ Swap仍在启用"
		false
	fi

	# 验证内核模块是否已加载
	echo -e "\n2. 验证内核模块是否已加载..."
	br_netfilter_loaded=$(lsmod | grep br_netfilter | wc -l)
	overlay_loaded=$(lsmod | grep overlay | wc -l)
	if [ "$br_netfilter_loaded" -gt 0 ] && [ "$overlay_loaded" -gt 0 ]; then
		echo "✓ 内核模块已加载"
	else
		echo "✗ 内核模块未加载"
		false
	fi
	`
	sysPrepVerifyOutput, err := client.RunCommandWithOutput(sysPrepVerifyCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("系统准备结果验证失败: %v\n输出: %s\n", err, sysPrepVerifyOutput)
		return fmt.Errorf("系统准备验证失败: %v", err)
	} else {
		fmt.Println("系统准备结果验证成功")
	}

	// 添加延迟，确保系统准备完全执行
	fmt.Println("\n等待5秒，确保系统准备完全执行...")
	if _, err := client.RunCommand("sleep 5"); err != nil {
		fmt.Printf("等待命令执行失败: %v\n", err)
	}

	// 4. IP转发配置
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("=== 步骤4: IP转发配置 ===")
	if m.logManager != nil {
		stepLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "IP转发配置",
			Output:    "配置IP转发设置",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(stepLog)
	}

	// 确保IP转发配置被正确设置，即使系统准备脚本中已有配置，再单独执行一次确保生效
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
		fmt.Printf("IP转发配置脚本执行失败: %v\n输出: %s\n", err, ensureIpForwardOutput)
		return fmt.Errorf("IP转发配置失败: %v", err)
	} else {
		fmt.Println("IP转发配置脚本执行成功")
	}

	// 添加延迟，确保IP转发配置完全生效
	fmt.Println("\n等待3秒，确保IP转发配置完全生效...")
	if _, err := client.RunCommand("sleep 3"); err != nil {
		fmt.Printf("等待命令执行失败: %v\n", err)
	}

	// 4.1 最终验证IP转发状态
	fmt.Println("\n4.1 最终验证IP转发状态...")
	finalCheckCmd := `
	# 最终验证IP转发状态
	final_ip_forward=$(sudo sysctl -n net.ipv4.ip_forward)
	echo "最终IP转发值: $final_ip_forward"
	
	# 检查/proc/sys/net/ipv4/ip_forward文件内容
	echo "=== 检查/proc/sys/net/ipv4/ip_forward文件内容 ==="
	proc_ip_forward=$(cat /proc/sys/net/ipv4/ip_forward)
	echo "文件内容: $proc_ip_forward"
	
	# 验证IP转发是否已启用
	if [ "$final_ip_forward" -eq 1 ] && [ "$proc_ip_forward" -eq 1 ]; then
		echo "✓ IP转发已成功启用"
	else
		echo "✗ IP转发未成功启用"
		false
	fi
	`
	finalCheckOutput, err := client.RunCommandWithOutput(finalCheckCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("IP转发最终验证失败: %v\n输出: %s\n", err, finalCheckOutput)
		return fmt.Errorf("IP转发验证失败: %v", err)
	} else {
		fmt.Println("IP转发最终验证成功")
	}

	// 5. 设置容器运行时
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("=== 步骤5: 容器运行时安装 ===")
	if m.logManager != nil {
		stepLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "容器运行时安装",
			Output:    "安装containerd容器运行时",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(stepLog)
	}
	containerRuntime := "containerd"
	if err := m.installContainerRuntime(client, distro, containerRuntime); err != nil {
		if m.logManager != nil {
			failLog := log.LogEntry{
				NodeID:    nodeID,
				NodeName:  nodeName,
				Operation: "部署工作节点",
				Command:   "容器运行时安装",
				Output:    fmt.Sprintf("容器运行时安装失败: %v", err),
				Status:    "failed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			m.logManager.CreateLog(failLog)
		}
		return err
	}
	if m.logManager != nil {
		successLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "容器运行时安装",
			Output:    "containerd容器运行时安装成功",
			Status:    "success",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(successLog)
	}

	// 6. 安装kubeadm和kubelet
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("=== 步骤6: Kubernetes组件安装 ===")
	if m.logManager != nil {
		stepLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "Kubernetes组件安装",
			Output:    "安装kubeadm、kubelet和kubectl",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(stepLog)
	}
	if err := m.installKubernetesComponents(client, distro); err != nil {
		if m.logManager != nil {
			failLog := log.LogEntry{
				NodeID:    nodeID,
				NodeName:  nodeName,
				Operation: "部署工作节点",
				Command:   "Kubernetes组件安装",
				Output:    fmt.Sprintf("Kubernetes组件安装失败: %v", err),
				Status:    "failed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			m.logManager.CreateLog(failLog)
		}
		return err
	}
	if m.logManager != nil {
		successLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "Kubernetes组件安装",
			Output:    "Kubernetes组件安装成功",
			Status:    "success",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(successLog)
	}

	// 7. 部署完成验证
	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("=== 步骤7: 部署完成验证 ===")
	if m.logManager != nil {
		stepLog := log.LogEntry{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Operation: "部署工作节点",
			Command:   "部署完成验证",
			Output:    "验证部署完成情况",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.logManager.CreateLog(stepLog)
	}
	finalVerifyCmd := `
	// 验证容器运行时状态
	echo "1. 验证容器运行时状态..."
	if systemctl is-active --quiet containerd; then
		echo "✓ containerd运行正常"
	else
		echo "✗ containerd运行异常"
		false
	fi

	// 验证kubelet状态
	echo -e "\n2. 验证kubelet状态..."
	if systemctl is-active --quiet kubelet; then
		echo "✓ kubelet运行正常"
	else
		echo "✗ kubelet运行异常"
		false
	fi

	// 验证kubeadm命令是否可用
	echo -e "\n3. 验证kubeadm命令..."
	if command -v kubeadm &> /dev/null; then
		kubeadm_version=$(kubeadm version --short 2>&1)
		echo "✓ kubeadm已安装: $kubeadm_version"
	else
		echo "✗ kubeadm未安装"
		false
	fi

	// 验证kubectl命令是否可用
	echo -e "\n4. 验证kubectl命令..."
	if command -v kubectl &> /dev/null; then
		kubectl_version=$(kubectl version --short 2>&1 | grep -i client)
		echo "✓ kubectl已安装: $kubectl_version"
	else
		echo "✗ kubectl未安装"
		false
	fi

	// 验证系统状态
	echo -e "\n5. 验证系统状态..."
	final_ip_forward=$(sudo sysctl -n net.ipv4.ip_forward)
	if [ "$final_ip_forward" -eq 1 ]; then
		echo "✓ IP转发已启用"
	else
		echo "✗ IP转发未启用"
		false
	fi

	// 验证swap是否已禁用
	echo -e "\n6. 验证swap状态..."
	swap_status=$(swapon --show | wc -l)
	if [ "$swap_status" -eq 0 ]; then
		echo "✓ Swap已禁用"
	else
		echo "✗ Swap仍在启用"
		false
	fi
	`
	finalVerifyOutput, err := client.RunCommandWithOutput(finalVerifyCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("部署完成验证失败: %v\n输出: %s\n", err, finalVerifyOutput)
		if m.logManager != nil {
			failLog := log.LogEntry{
				NodeID:    nodeID,
				NodeName:  nodeName,
				Operation: "部署工作节点",
				Command:   "部署完成验证",
				Output:    fmt.Sprintf("部署完成验证失败: %v", err),
				Status:    "failed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			m.logManager.CreateLog(failLog)
			// 记录部署失败最终日志
			finalFailLog := log.LogEntry{
				NodeID:    nodeID,
				NodeName:  nodeName,
				Operation: "部署工作节点",
				Command:   "部署结束",
				Output:    "Kubernetes工作节点部署失败",
				Status:    "failed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			m.logManager.CreateLog(finalFailLog)
		}
		return fmt.Errorf("部署完成验证失败: %v", err)
	} else {
		fmt.Println("\n" + strings.Repeat("=", 80))
		fmt.Println("=== Kubernetes工作节点部署完成! ===")
		fmt.Println(strings.Repeat("=", 80))
		if m.logManager != nil {
			successLog := log.LogEntry{
				NodeID:    nodeID,
				NodeName:  nodeName,
				Operation: "部署工作节点",
				Command:   "部署完成验证",
				Output:    "部署完成验证成功",
				Status:    "success",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			m.logManager.CreateLog(successLog)
			// 记录部署成功最终日志
			finalSuccessLog := log.LogEntry{
				NodeID:    nodeID,
				NodeName:  nodeName,
				Operation: "部署工作节点",
				Command:   "部署结束",
				Output:    "Kubernetes工作节点部署成功",
				Status:    "success",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			m.logManager.CreateLog(finalSuccessLog)
		}
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
						fmt.Printf("Using custom script for container runtime installation: %v\n", runtime)
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

	// 验证容器运行时安装
	verifyCmd := `
	// 验证容器运行时命令是否可用
	echo "验证容器运行时命令..."
	if command -v containerd &> /dev/null; then
		containerd_version=$(containerd --version 2>&1 | head -n 1)
		echo "✓ containerd已安装: $containerd_version"
	else
		echo "✗ containerd未安装"
		false
	fi

	// 验证containerd状态
	echo -e "\n验证containerd状态..."
	if systemctl is-active --quiet containerd; then
		echo "✓ containerd运行正常"
	else
		echo "✗ containerd运行异常"
		false
	fi
	`
	verifyOutput, err := client.RunCommandWithOutput(verifyCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("容器运行时验证失败: %v\n输出: %s\n", err, verifyOutput)
		return fmt.Errorf("容器运行时验证失败: %v", err)
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

	// 验证Kubernetes组件安装
	k8sVerifyCmd := `
	// 验证kubelet状态
	echo "1. 验证kubelet状态..."
	if systemctl is-active --quiet kubelet; then
		echo "✓ kubelet运行正常"
	else
		echo "✗ kubelet运行异常"
		false
	fi

	// 验证kubeadm命令是否可用
	echo -e "\n2. 验证kubeadm命令..."
	if command -v kubeadm &> /dev/null; then
		kubeadm_version=$(kubeadm version --short 2>&1)
		echo "✓ kubeadm已安装: $kubeadm_version"
	else
		echo "✗ kubeadm未安装"
		false
	fi

	// 验证kubectl命令是否可用
	echo -e "\n3. 验证kubectl命令..."
	if command -v kubectl &> /dev/null; then
		kubectl_version=$(kubectl version --short 2>&1 | grep -i client)
		echo "✓ kubectl已安装: $kubectl_version"
	else
		echo "✗ kubectl未安装"
		false
	fi

	// 验证kubelet已启用
	echo -e "\n4. 验证kubelet是否已启用..."
	if systemctl is-enabled --quiet kubelet; then
		echo "✓ kubelet已设置为开机自启"
	else
		echo "✗ kubelet未设置为开机自启"
		false
	fi
	`
	k8sVerifyOutput, err := client.RunCommandWithOutput(k8sVerifyCmd, func(line string) {
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		fmt.Printf("Kubernetes组件验证失败: %v\n输出: %s\n", err, k8sVerifyOutput)
		return fmt.Errorf("Kubernetes组件验证失败: %v", err)
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
