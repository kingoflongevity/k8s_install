package script

import (
	"os"
	"sync"
	"time"
)

// ScriptManager 脚本管理器
type ScriptManager struct {
	mutex     sync.RWMutex
	scripts   map[string]string
	scriptDir string
	db        interface{}
}

// NewScriptManager 创建新的脚本管理器
func NewScriptManager(scriptDir string) (*ScriptManager, error) {
	// 确保脚本目录存在
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		return nil, err
	}

	manager := &ScriptManager{
		scriptDir: scriptDir,
		scripts:   make(map[string]string),
	}

	// 首先加载默认脚本，确保我们有最新的默认脚本版本
	manager.loadDefaultScripts()

	// 然后尝试加载已保存的自定义脚本，这会覆盖默认脚本
	if err := manager.LoadScripts(); err != nil {
		// 如果加载失败，保存默认脚本，确保下次能正确加载
		manager.SaveScripts()
	}

	// 确保所有默认脚本都存在，如果自定义脚本中缺少某些脚本，使用默认脚本补充
	manager.ensureDefaultScripts()

	return manager, nil
}

// SetDB 设置数据库连接，用于将脚本存储到数据库中
func (m *ScriptManager) SetDB(db interface{}) {
	m.db = db
}

// loadDefaultScripts 加载默认脚本，确保使用最新的脚本内容
// 注意：调用此方法前必须确保已经持有写锁，否则会导致死锁
func (m *ScriptManager) loadDefaultScripts() {

	// 默认系统准备脚本
	m.scripts["system_prep"] = `# 系统准备脚本
# 禁用swap
sudo swapoff -a
sudo sed -i '/ swap / s/^#/' /etc/fstab

# 安装并启动时间同步服务
echo "=== 安装并配置时间同步 ==="
if command -v apt-get &> /dev/null; then
    sudo apt update -y
    sudo apt install -y chrony
    sudo systemctl enable --now chronyd || sudo systemctl enable --now chrony
    sudo timedatectl set-timezone Asia/Shanghai
    sudo systemctl restart chronyd || sudo systemctl restart chrony
    chronyc sources
elif command -v dnf &> /dev/null || command -v yum &> /dev/null; then
    if command -v dnf &> /dev/null; then
        sudo dnf install -y chrony
    else
        sudo yum install -y chrony
    fi
    sudo systemctl enable --now chronyd
    sudo timedatectl set-timezone Asia/Shanghai
    sudo systemctl restart chronyd || sudo systemctl restart chrony
    chronyc sources
fi

# 关闭防火墙（实验环境建议关闭）
echo "=== 配置防火墙 ==="
if command -v ufw &> /dev/null; then
    sudo systemctl stop ufw || true
    sudo systemctl disable ufw || true
elif command -v firewall-cmd &> /dev/null; then
    sudo systemctl stop firewalld || true
    sudo systemctl disable firewalld || true
fi

# 禁用SELINUX（仅适用于RHEL/CentOS系统）
echo "=== 配置SELinux ==="
if command -v setenforce &> /dev/null; then
    sudo setenforce 0 2>/dev/null || true
    sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config 2>/dev/null || true
fi

# 加载K8s所需内核模块
echo "=== 加载Kubernetes所需内核模块 ==="
sudo cat <<EOF > /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

# 设置内核参数
echo "=== 设置内核参数 ==="
# 使用EOF方式写入IP转发配置文件
echo "1. 正在配置IP转发..."
sudo cat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf
net.ipv4.ip_forward = 1
EOF

# 验证IP转发配置文件
echo "2. 验证IP转发配置文件..."
sudo cat /etc/sysctl.d/99-kubernetes-ipforward.conf

# 设置其他Kubernetes所需内核参数
echo "3. 正在配置其他Kubernetes内核参数..."
sudo cat <<EOF > /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

# 验证其他内核参数配置文件
echo "4. 验证其他内核参数配置文件..."
sudo cat /etc/sysctl.d/k8s.conf

# 应用所有内核参数
echo "5. 正在应用内核参数..."
sudo sysctl --system

# 立即设置IP转发值，确保即时生效
echo "6. 确保IP转发即时生效..."
sudo sysctl -w net.ipv4.ip_forward=1
sudo sysctl -w net.bridge.bridge-nf-call-iptables=1
sudo sysctl -w net.bridge.bridge-nf-call-ip6tables=1

# 验证内核参数设置
echo "7. 最终验证内核参数..."
sudo sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward

# 检查/proc/sys/net/ipv4/ip_forward文件内容
echo "8. 检查/proc/sys/net/ipv4/ip_forward文件内容..."
cat /proc/sys/net/ipv4/ip_forward`

	// 默认containerd安装脚本
	m.scripts["containerd_install"] = `# containerd安装脚本
echo "=== 安装containerd ==="
if ! command -v containerd &> /dev/null; then
    echo "containerd未安装，正在安装..."
    if command -v apt-get &> /dev/null; then
        # Ubuntu/Debian系统
        echo "=== 使用apt-get安装containerd ==="
        sudo apt update -y
        sudo apt install -y containerd
    elif command -v dnf &> /dev/null; then
        # CentOS/RHEL 8+系统
        echo "=== 使用dnf安装containerd ==="
        sudo dnf install -y containerd
    elif command -v yum &> /dev/null; then
        # CentOS/RHEL 7系统
        echo "=== 使用yum安装containerd ==="
        sudo yum install -y containerd
    else
        echo "=== 警告: 不支持的包管理器，尝试手动安装containerd ==="
        # 尝试从GitHub下载并安装containerd
        if command -v curl &> /dev/null && command -v tar &> /dev/null; then
            CONTAINERD_VERSION="1.6.28"
            ARCH="amd64"
            echo "从GitHub下载containerd v${CONTAINERD_VERSION}..."
            sudo curl -L https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz -o /tmp/containerd.tar.gz
            sudo mkdir -p /usr/local/bin /usr/local/lib /etc/containerd
            sudo tar Cxzvf /usr/local /tmp/containerd.tar.gz
            # 创建systemd服务文件
            sudo cat > /etc/systemd/system/containerd.service <<-'EOF'
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd
Restart=always
RestartSec=5
Delegate=yes
KillMode=process
OOMScoreAdjust=-999
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity

[Install]
WantedBy=multi-user.target
EOF
            sudo systemctl daemon-reload
            sudo systemctl enable containerd
        fi
    fi
fi`

	// 默认containerd配置脚本
	m.scripts["containerd_config"] = `# containerd配置脚本
echo "=== 配置并启动containerd ==="
# 确保containerd配置目录存在
sudo mkdir -p /etc/containerd

# 生成默认配置，覆盖现有配置以确保正确性
echo "生成containerd默认配置..."
sudo containerd config default | sudo tee /etc/containerd/config.toml

# 确保使用systemd cgroup驱动
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# 启动前先停止可能运行的containerd进程
echo "停止可能运行的containerd进程..."
sudo pkill -f containerd || true
sleep 2

# 清理旧的containerd socket和状态文件
echo "清理旧的containerd socket和状态文件..."
sudo rm -f /run/containerd/containerd.sock
sudo rm -rf /var/run/containerd
sudo mkdir -p /var/run/containerd

# 启动并启用containerd服务
echo "启动containerd服务..."
sudo systemctl daemon-reload
sudo systemctl restart containerd
sudo systemctl enable containerd

# 等待containerd启动，增加等待时间
echo "等待containerd启动..."
sleep 10

# 检查containerd状态
echo "=== 检查containerd状态 ==="
if command -v systemctl &> /dev/null; then
    systemctl_status=$(sudo systemctl is-active containerd)
    echo "containerd服务状态: $systemctl_status"
    
    # 显示containerd服务详细状态
    echo "containerd服务详细状态:"
    sudo systemctl status containerd --no-pager
fi

# 检查containerd socket是否存在
echo "=== 检查containerd socket ==="
cri_socket="/run/containerd/containerd.sock"
if [ -S "$cri_socket" ]; then
    echo "CRI socket $cri_socket 存在"
    # 测试socket连接
    echo "测试containerd连接..."
    if command -v ctr &> /dev/null; then
        ctr version
    fi
else
    echo "警告: CRI socket $cri_socket 不存在，检查containerd日志..."
    sudo journalctl -u containerd --no-pager -n 30
    
    # 尝试手动启动containerd
    echo "尝试手动启动containerd..."
    containerd --version
    containerd &
    sleep 5
    
    # 再次检查socket
    if [ -S "$cri_socket" ]; then
        echo "手动启动成功，CRI socket $cri_socket 现在存在"
    else
        echo "手动启动失败，CRI socket $cri_socket 仍然不存在"
    fi
fi`

	// 默认Kubernetes组件安装脚本
	m.scripts["k8s_components"] = `# Kubernetes组件安装脚本
echo "=== 安装Kubernetes组件 ==="
# 确保所有命令都使用sudo权限
if command -v apt-get &> /dev/null; then
    # Ubuntu/Debian系统
    sudo apt-get update -y
    sudo apt-get install -y kubelet kubeadm kubectl
    sudo systemctl enable --now kubelet
elif command -v dnf &> /dev/null; then
    # CentOS/RHEL 8+系统
    sudo dnf install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
    sudo systemctl enable --now kubelet
elif command -v yum &> /dev/null; then
    # CentOS/RHEL 7系统
    sudo yum install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
    sudo systemctl enable --now kubelet
fi`

	// 默认Kubernetes初始化脚本
	m.scripts["k8s_init"] = `# 初始化Kubernetes集群
# 执行kubeadm init
echo "=== 执行kubeadm init ==="
sudo kubeadm init --pod-network-cidr=10.244.0.0/16 --upload-certs

# 检查kubeadm init是否成功
if [ $? -eq 0 ]; then
    echo "=== kubeadm init 成功 ==="
    
    # 配置kubectl
echo "=== 配置kubectl ==="
mkdir -p $HOME/.kube
    
    # 检查admin.conf是否存在
    if [ -f /etc/kubernetes/admin.conf ]; then
        echo "✓ 找到admin.conf文件，正在配置kubectl..."
        sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
        sudo chown $(id -u):$(id -g) $HOME/.kube/config
        echo "✓ kubectl配置成功"
    else
        echo "✗ 未找到admin.conf文件，可能初始化过程中出现问题"
    fi
    
    # 安装CNI网络插件（使用Flannel）
    if [ -f $HOME/.kube/config ]; then
        echo "=== 安装Flannel网络插件 ==="
        kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
    else
        echo "✗ 无法安装CNI插件，kubectl配置失败"
    fi
else
    echo "✗ kubeadm init 失败"
    # 显示更多错误信息
    echo "=== 显示kubeadm日志 ==="
    sudo journalctl -u kubelet --no-pager -n 50
fi`
}

// GetScripts 获取所有脚本
func (m *ScriptManager) GetScripts() map[string]string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回脚本的副本，避免外部修改
	scriptsCopy := make(map[string]string)
	for k, v := range m.scripts {
		scriptsCopy[k] = v
	}

	return scriptsCopy
}

// GetScript 获取指定脚本
func (m *ScriptManager) GetScript(name string) (string, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	script, ok := m.scripts[name]
	return script, ok
}

// saveScriptsToDB 将脚本保存到数据库
func (m *ScriptManager) saveScriptsToDB() error {
	// 检查数据库连接是否存在
	if m.db == nil {
		return nil
	}

	// 使用类型断言获取*sql.DB
	if db, ok := m.db.(interface {
		Exec(string, ...interface{}) (interface{}, error)
		Begin() (interface{}, error)
	}); ok {
		// 开始事务
		tx, err := db.Begin()
		if err != nil {
			return err
		}

		// 检查事务是否有Commit和Rollback方法
		if txWithCommit, ok := tx.(interface{ Commit() error }); ok {
			if txWithRollback, ok := tx.(interface{ Rollback() error }); ok {
				// 检查事务是否有Exec方法
				if txWithExec, ok := tx.(interface {
					Exec(string, ...interface{}) (interface{}, error)
				}); ok {
					// 先删除所有现有脚本
					if _, err := txWithExec.Exec("DELETE FROM scripts"); err != nil {
						txWithRollback.Rollback()
						return err
					}

					// 获取当前时间
					now := time.Now()

					// 插入所有脚本
					for name, content := range m.scripts {
						if _, err := txWithExec.Exec(
							"INSERT INTO scripts (name, content, created_at, updated_at) VALUES (?, ?, ?, ?)",
							name, content, now, now,
						); err != nil {
							txWithRollback.Rollback()
							return err
						}
					}

					// 提交事务
					return txWithCommit.Commit()
				}
			}
		}
	}

	return nil
}

// SaveScripts 只保存脚本到数据库
func (m *ScriptManager) SaveScripts() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 只保存到数据库，不保存到文件
	if err := m.saveScriptsToDB(); err != nil {
		return err
	}

	return nil
}

// LoadScripts 只从数据库加载脚本
func (m *ScriptManager) LoadScripts() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 清空当前脚本，确保只使用数据库中的脚本
	m.scripts = make(map[string]string)

	// 从数据库加载脚本
	if m.db != nil {
		// 使用类型断言获取*sql.DB
		if db, ok := m.db.(interface {
			Query(string, ...interface{}) (interface{}, error)
		}); ok {
			// 查询所有脚本
			rows, err := db.Query("SELECT name, content FROM scripts")
			if err == nil {
				// 检查是否有rows.Close()方法
				if rowsWithClose, ok := rows.(interface{ Close() error }); ok {
					defer rowsWithClose.Close()
				}

				// 检查是否有rows.Next()和rows.Scan()方法
				if rowsWithNext, ok := rows.(interface{ Next() bool }); ok {
					if rowsWithScan, ok := rows.(interface{ Scan(...interface{}) error }); ok {
						// 遍历结果集
						for rowsWithNext.Next() {
							var name, content string
							if err := rowsWithScan.Scan(&name, &content); err != nil {
								continue
							}
							// 将脚本添加到map中
							m.scripts[name] = content
						}
					}
				}
			}
		}
	}

	// 如果数据库中没有脚本，使用默认脚本
	if len(m.scripts) == 0 {
		// 加载默认脚本
		m.loadDefaultScripts()
		// 将默认脚本保存到数据库
		m.saveScriptsToDB()
	}

	return nil
}

// UpdateScript 更新指定脚本
func (m *ScriptManager) UpdateScript(name, content string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.scripts[name] = content
}

// UpdateScripts 更新所有脚本
func (m *ScriptManager) UpdateScripts(scripts map[string]string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for k, v := range scripts {
		m.scripts[k] = v
	}
}

// ensureDefaultScripts 确保所有默认脚本都存在，增强版本：
// 1. 保留用户自定义的脚本
// 2. 为缺失的脚本使用最新的默认脚本
// 3. 确保脚本内容的完整性和正确性
func (m *ScriptManager) ensureDefaultScripts() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 加载最新的默认脚本到临时映射中
	latestDefaultScripts := make(map[string]string)

	// 默认系统准备脚本
	latestDefaultScripts["system_prep"] = `# 系统准备脚本
# 禁用swap
sudo swapoff -a
sudo sed -i '/ swap / s/^#/' /etc/fstab

# 安装并启动时间同步服务
echo "=== 安装并配置时间同步 ==="
if command -v apt-get &> /dev/null; then
    sudo apt update -y
    sudo apt install -y chrony
    sudo systemctl enable --now chronyd || sudo systemctl enable --now chrony
    sudo timedatectl set-timezone Asia/Shanghai
    sudo systemctl restart chronyd || sudo systemctl restart chrony
    chronyc sources
elif command -v dnf &> /dev/null || command -v yum &> /dev/null; then
    if command -v dnf &> /dev/null; then
        sudo dnf install -y chrony
    else
        sudo yum install -y chrony
    fi
    sudo systemctl enable --now chronyd
    sudo timedatectl set-timezone Asia/Shanghai
    sudo systemctl restart chronyd || sudo systemctl restart chrony
    chronyc sources
fi

# 关闭防火墙（实验环境建议关闭）
echo "=== 配置防火墙 ==="
if command -v ufw &> /dev/null; then
    sudo systemctl stop ufw || true
    sudo systemctl disable ufw || true
elif command -v firewall-cmd &> /dev/null; then
    sudo systemctl stop firewalld || true
    sudo systemctl disable firewalld || true
fi

# 禁用SELINUX（仅适用于RHEL/CentOS系统）
echo "=== 配置SELinux ==="
if command -v setenforce &> /dev/null; then
    sudo setenforce 0 2>/dev/null || true
    sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config 2>/dev/null || true
fi

# 加载K8s所需内核模块
echo "=== 加载Kubernetes所需内核模块 ==="
sudo cat <<EOF > /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

# 设置内核参数
echo "=== 设置内核参数 ==="
# 使用EOF方式写入IP转发配置文件
echo "1. 正在配置IP转发..."
sudo cat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf
net.ipv4.ip_forward = 1
EOF

# 验证IP转发配置文件
echo "2. 验证IP转发配置文件..."
sudo cat /etc/sysctl.d/99-kubernetes-ipforward.conf

# 设置其他Kubernetes所需内核参数
echo "3. 正在配置其他Kubernetes内核参数..."
sudo cat <<EOF > /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

# 验证其他内核参数配置文件
echo "4. 验证其他内核参数配置文件..."
sudo cat /etc/sysctl.d/k8s.conf

# 应用所有内核参数
echo "5. 正在应用内核参数..."
sudo sysctl --system

# 立即设置IP转发值，确保即时生效
echo "6. 确保IP转发即时生效..."
sudo sysctl -w net.ipv4.ip_forward=1
sudo sysctl -w net.bridge.bridge-nf-call-iptables=1
sudo sysctl -w net.bridge.bridge-nf-call-ip6tables=1

# 验证内核参数设置
echo "7. 最终验证内核参数..."
sudo sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward

# 检查/proc/sys/net/ipv4/ip_forward文件内容
echo "8. 检查/proc/sys/net/ipv4/ip_forward文件内容..."
cat /proc/sys/net/ipv4/ip_forward`

	// 默认containerd安装脚本
	latestDefaultScripts["containerd_install"] = `# containerd安装脚本
echo "=== 安装containerd ==="
if ! command -v containerd &> /dev/null; then
    echo "containerd未安装，正在安装..."
    if command -v apt-get &> /dev/null; then
        # Ubuntu/Debian系统
        echo "=== 使用apt-get安装containerd ==="
        sudo apt update -y
        sudo apt install -y containerd
    elif command -v dnf &> /dev/null; then
        # CentOS/RHEL 8+系统
        echo "=== 使用dnf安装containerd ==="
        sudo dnf install -y containerd
    elif command -v yum &> /dev/null; then
        # CentOS/RHEL 7系统
        echo "=== 使用yum安装containerd ==="
        sudo yum install -y containerd
    else
        echo "=== 警告: 不支持的包管理器，尝试手动安装containerd ==="
        # 尝试从GitHub下载并安装containerd
        if command -v curl &> /dev/null && command -v tar &> /dev/null; then
            CONTAINERD_VERSION="1.6.28"
            ARCH="amd64"
            echo "从GitHub下载containerd v${CONTAINERD_VERSION}..."
            sudo curl -L https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz -o /tmp/containerd.tar.gz
            sudo mkdir -p /usr/local/bin /usr/local/lib /etc/containerd
            sudo tar Cxzvf /usr/local /tmp/containerd.tar.gz
            # 创建systemd服务文件
            sudo cat > /etc/systemd/system/containerd.service <<-'EOF'
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd
Restart=always
RestartSec=5
Delegate=yes
KillMode=process
OOMScoreAdjust=-999
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity

[Install]
WantedBy=multi-user.target
EOF
            sudo systemctl daemon-reload
            sudo systemctl enable containerd
        fi
    fi
fi`

	// 默认containerd配置脚本
	latestDefaultScripts["containerd_config"] = `# containerd配置脚本
echo "=== 配置并启动containerd ==="

# 1. 确保containerd配置目录存在
echo "1. 确保containerd配置目录存在..."
sudo mkdir -p /etc/containerd /run/containerd /var/lib/containerd

# 2. 生成默认配置，覆盖现有配置以确保正确性
echo "2. 生成containerd默认配置..."
sudo containerd config default | sudo tee /etc/containerd/config.toml

# 3. 确保使用systemd cgroup驱动
echo "3. 配置systemd cgroup驱动..."
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# 4. 确保containerd服务文件存在
echo "4. 确保containerd服务文件存在..."
if [ ! -f /etc/systemd/system/containerd.service ]; then
    echo "创建containerd服务文件..."
    sudo cat > /etc/systemd/system/containerd.service <<-'EOF'
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/bin/containerd
Restart=always
RestartSec=5
Delegate=yes
KillMode=process
OOMScoreAdjust=-999
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity

[Install]
WantedBy=multi-user.target
EOF
fi

# 5. 检查containerd二进制文件位置
echo "5. 检查containerd二进制文件位置..."
if [ ! -f /usr/bin/containerd ]; then
    # 查找containerd二进制文件
    containerd_path=$(which containerd 2>/dev/null || echo "/usr/local/bin/containerd")
    echo "containerd二进制文件位置: $containerd_path"
    
    # 如果在/usr/local/bin，创建符号链接到/usr/bin
    if [ "$containerd_path" = "/usr/local/bin/containerd" ] && [ ! -f /usr/bin/containerd ]; then
        echo "创建containerd符号链接..."
        sudo ln -sf $containerd_path /usr/bin/containerd
        # 也创建ctr的符号链接
        if [ -f /usr/local/bin/ctr ]; then
            sudo ln -sf /usr/local/bin/ctr /usr/bin/ctr
        fi
    fi
fi

# 6. 启动前先停止可能运行的containerd进程
echo "6. 停止可能运行的containerd进程..."
sudo pkill -f containerd || true
sleep 2

# 7. 清理旧的containerd socket和状态文件
echo "7. 清理旧的containerd socket和状态文件..."
sudo rm -rf /run/containerd /var/run/containerd /var/lib/containerd/*
sudo mkdir -p /run/containerd /var/run/containerd /var/lib/containerd

# 8. 启动并启用containerd服务
echo "8. 启动containerd服务..."
sudo systemctl daemon-reload
sudo systemctl enable containerd
# 使用--now参数确保立即启动
sudo systemctl restart containerd

# 9. 等待containerd启动，增加等待时间
echo "9. 等待containerd启动..."
sleep 15

# 10. 检查containerd状态
echo "=== 检查containerd状态 ==="
if command -v systemctl &> /dev/null; then
    systemctl_status=$(sudo systemctl is-active containerd)
    echo "containerd服务状态: $systemctl_status"
    
    # 显示containerd服务详细状态
    echo "containerd服务详细状态:"
    sudo systemctl status containerd --no-pager
    
    # 如果服务未运行，显示日志并尝试修复
    if [ "$systemctl_status" != "active" ]; then
        echo "=== 显示containerd错误日志 ==="
        sudo journalctl -u containerd --no-pager -n 50
        
        echo "=== 尝试修复containerd配置 ==="
        # 重新生成配置
        sudo containerd config default | sudo tee /etc/containerd/config.toml
        sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
        # 再次启动
        sudo systemctl restart containerd
        sleep 10
        # 再次检查状态
        systemctl_status=$(sudo systemctl is-active containerd)
        echo "修复后containerd服务状态: $systemctl_status"
    fi
fi

# 11. 检查containerd socket是否存在
echo "=== 检查containerd socket ==="
cri_socket="/run/containerd/containerd.sock"
attempt=1
max_attempts=3
while [ ! -S "$cri_socket" ] && [ $attempt -le $max_attempts ]; do
    echo "$attempt/$max_attempts: CRI socket $cri_socket 不存在，尝试手动启动containerd..."
    
    # 停止可能存在的containerd进程
    sudo pkill -f containerd || true
    sleep 2
    
    # 清理旧的socket和状态文件
    sudo rm -rf /run/containerd /var/run/containerd /var/lib/containerd/*
    sudo mkdir -p /run/containerd /var/run/containerd /var/lib/containerd
    
    # 手动启动containerd
    echo "手动启动containerd..."
    containerd --version
    # 使用nohup确保containerd在后台运行
    nohup sudo containerd > /tmp/containerd.log 2>&1 &
    CONTAINERD_PID=$!
    echo "containerd进程ID: $CONTAINERD_PID"
    
    # 等待10秒让containerd启动
    sleep 10
    
    # 检查socket
    if [ -S "$cri_socket" ]; then
        echo "✓ 手动启动成功，CRI socket $cri_socket 已创建"
        break
    else
        echo "✗ 手动启动失败，CRI socket $cri_socket 仍不存在"
        echo "=== 显示containerd手动启动日志 ==="
        cat /tmp/containerd.log | head -n 50
        # 杀死可能的进程
        sudo kill -9 $CONTAINERD_PID 2>/dev/null || true
        sleep 2
        attempt=$((attempt + 1))
    fi
done

# 12. 测试containerd连接
echo "=== 测试containerd连接 ==="
if [ -S "$cri_socket" ]; then
    echo "CRI socket $cri_socket 存在"
    # 测试socket连接
    echo "测试containerd连接..."
    if command -v ctr &> /dev/null; then
        ctr version
    elif [ -f /usr/local/bin/ctr ]; then
        /usr/local/bin/ctr version
    else
        echo "无法找到ctr命令，跳过连接测试"
    fi
else
    echo "✗ 最终失败: CRI socket $cri_socket 仍然不存在"
    echo "=== 显示最终containerd日志 ==="
    sudo journalctl -u containerd --no-pager -n 100
    exit 1
fi`

	// 添加Kubernetes组件安装脚本
	latestDefaultScripts["k8s_components"] = `# Kubernetes组件安装脚本
echo "=== 安装Kubernetes组件 ==="
# 确保所有命令都使用sudo权限
if command -v apt-get &> /dev/null; then
    # Ubuntu/Debian系统
    sudo apt-get update -y
    sudo apt-get install -y kubelet kubeadm kubectl
    sudo systemctl enable --now kubelet
elif command -v dnf &> /dev/null; then
    # CentOS/RHEL 8+系统
    sudo dnf install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
    sudo systemctl enable --now kubelet
elif command -v yum &> /dev/null; then
    # CentOS/RHEL 7系统
    sudo yum install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
    sudo systemctl enable --now kubelet
fi`

	// 添加Kubernetes初始化脚本
	latestDefaultScripts["k8s_init"] = `# 初始化Kubernetes集群
# 执行kubeadm init
echo "=== 执行kubeadm init ==="
sudo kubeadm init --pod-network-cidr=10.244.0.0/16 --upload-certs

# 检查kubeadm init是否成功
if [ $? -eq 0 ]; then
    echo "=== kubeadm init 成功 ==="
    
    # 配置kubectl
echo "=== 配置kubectl ==="
mkdir -p $HOME/.kube
    
    # 检查admin.conf是否存在
    if [ -f /etc/kubernetes/admin.conf ]; then
        echo "✓ 找到admin.conf文件，正在配置kubectl..."
        sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
        sudo chown $(id -u):$(id -g) $HOME/.kube/config
        echo "✓ kubectl配置成功"
    else
        echo "✗ 未找到admin.conf文件，可能初始化过程中出现问题"
    fi
    
    # 安装CNI网络插件（使用Flannel）
    if [ -f $HOME/.kube/config ]; then
        echo "=== 安装Flannel网络插件 ==="
        kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
    else
        echo "✗ 无法安装CNI插件，kubectl配置失败"
    fi
else
    echo "✗ kubeadm init 失败"
    # 显示更多错误信息
    echo "=== 显示kubeadm日志 ==="
    sudo journalctl -u kubelet --no-pager -n 50
fi`

	// 确保用户自定义的脚本被保留，同时添加缺失的默认脚本
	for scriptName, latestScriptContent := range latestDefaultScripts {
		// 如果用户已有自定义脚本，则保留
		if _, exists := m.scripts[scriptName]; !exists {
			// 如果用户没有该脚本，则使用最新的默认脚本
			m.scripts[scriptName] = latestScriptContent
		}
	}

	// 保存更新后的脚本到数据库，确保下次能正确加载
	// 直接保存到数据库，避免调用SaveScripts()函数再次获取锁，导致死锁
	m.saveScriptsToDB()
}
