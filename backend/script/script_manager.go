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

// latestDefaultScripts 包级别的默认脚本映射
var latestDefaultScripts map[string]string

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
sudo sed -i '/ swap / s/^/#/' /etc/fstab

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
    sudo sed -i 's/^SELINUX=disabled$/SELINUX=permissive/' /etc/selinux/config 2>/dev/null || true
    # 验证SELinux配置
    sudo grep -E '^SELINUX=' /etc/selinux/config 2>/dev/null || true
    # 再次确认SELinux状态
    sudo getenforce 2>/dev/null || true
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
    elif command -v dnf &> /dev/null || command -v yum &> /dev/null; then
        # CentOS/RHEL系统
        echo "=== 添加Docker仓库 ==="
        # 安装必要的依赖
        if command -v dnf &> /dev/null; then
            sudo dnf install -y dnf-plugins-core
            sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            sudo dnf install -y containerd.io
        else
            sudo yum install -y yum-utils
            sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            sudo yum install -y containerd.io
        fi
    else
        echo "=== 警告: 不支持的包管理器，尝试手动安装containerd ==="
        # 尝试从GitHub下载并安装containerd
        if command -v curl &> /dev/null && command -v tar &> /dev/null; then
            CONTAINERD_VERSION="1.6.28"
            ARCH="amd64"
            echo "从GitHub下载containerd v${CONTAINERD_VERSION}..."
            sudo curl -fsSL -o /tmp/containerd.tar.gz https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz
            sudo mkdir -p /usr/local/bin /usr/local/lib /etc/containerd
            sudo tar Cxzvf /usr/local /tmp/containerd.tar.gz
            sudo rm -f /tmp/containerd.tar.gz
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
else
    echo "containerd已安装，跳过安装步骤"
fi`

	// 默认containerd配置脚本
	m.scripts["containerd_config"] = `# containerd配置脚本
echo "=== 配置并启动containerd ==="
# 确保containerd配置目录存在
sudo mkdir -p /etc/containerd /run/containerd /var/run/containerd

# 生成默认配置，覆盖现有配置以确保正确性
echo "生成containerd默认配置..."
sudo containerd config default | sudo tee /etc/containerd/config.toml

# 确保使用systemd cgroup驱动
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# 启动前先停止可能运行的containerd进程
echo "停止可能运行的containerd进程..."
# 使用更安全的方式停止containerd服务
if command -v systemctl &> /dev/null; then
    sudo systemctl stop containerd || true
else
    # 如果没有systemctl，尝试使用service命令
    if command -v service &> /dev/null; then
        sudo service containerd stop || true
    else
        # 最后尝试使用pkill，但更精确地匹配进程
        sudo pkill -x containerd || true
    fi
fi
sleep 1

# 清理旧的containerd socket和状态文件
echo "清理旧的containerd socket和状态文件..."
sudo rm -f /run/containerd/containerd.sock
sudo rm -rf /var/run/containerd
sudo mkdir -p /var/run/containerd

# 启动并启用containerd服务
echo "启动containerd服务..."
sudo systemctl daemon-reload
sudo systemctl enable containerd
sudo systemctl start containerd

# 等待containerd启动，减少等待时间
echo "等待containerd启动..."
sleep 8

# 检查containerd状态
echo "=== 检查containerd状态 ==="
cri_socket="/run/containerd/containerd.sock"
containerd_ready=false

# 快速检查socket是否存在
if [ -S "$cri_socket" ]; then
    containerd_ready=true
    echo "✓ CRI socket $cri_socket 存在"
else
    # 如果socket不存在，再检查systemctl状态
    if command -v systemctl &> /dev/null; then
        systemctl_status=$(sudo systemctl is-active containerd 2>/dev/null || echo "inactive")
        echo "containerd服务状态: $systemctl_status"
        
        if [ "$systemctl_status" = "active" ]; then
            # 服务已启动但socket不存在，等待3秒后再次检查
            echo "服务已启动，等待socket创建..."
            sleep 3
            if [ -S "$cri_socket" ]; then
                containerd_ready=true
                echo "✓ CRI socket $cri_socket 现在存在"
            fi
        fi
    fi
fi

# 检查containerd socket是否存在
echo "=== 检查containerd socket ==="
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
    echo "✓ containerd配置完成"
else
    echo "警告: CRI socket $cri_socket 不存在，检查containerd日志..."
    if command -v journalctl &> /dev/null; then
        sudo journalctl -u containerd --no-pager -n 30
    fi
    
    # 尝试手动启动containerd
    echo "尝试手动启动containerd..."
    # 停止可能存在的containerd进程
    sudo pkill -x containerd || true
    sleep 1
    # 清理旧的socket和状态文件
    sudo rm -rf /run/containerd /var/run/containerd
    sudo mkdir -p /run/containerd /var/run/containerd
    
    containerd --version
    # 使用nohup确保containerd在后台运行
    nohup sudo containerd > /tmp/containerd.log 2>&1 &
    CONTAINERD_PID=$!
    echo "containerd进程ID: $CONTAINERD_PID"
    
    # 等待5秒
    sleep 5
    
    # 再次检查socket
    if [ -S "$cri_socket" ]; then
        echo "手动启动成功，CRI socket $cri_socket 现在存在"
        echo "✓ containerd配置完成"
    else
        echo "手动启动失败，CRI socket $cri_socket 仍然不存在"
        echo "=== 显示containerd手动启动日志 ==="
        cat /tmp/containerd.log | head -n 50
    fi
fi`

	// 默认Kubernetes组件安装脚本
	m.scripts["k8s_components"] = `# Kubernetes组件安装脚本
echo "=== 安装Kubernetes组件 ==="

# 处理版本号，移除v前缀（如果存在）
KUBE_VERSION=${version}
KUBE_VERSION=${KUBE_VERSION#v}  # 移除v前缀

# 确保所有命令都使用sudo权限
if command -v apt-get &> /dev/null; then
    # Ubuntu/Debian系统
    sudo apt-get update -y
    sudo apt-get install -y kubelet=${version} kubeadm=${version} kubectl=${version}
    sudo systemctl enable --now kubelet
elif command -v dnf &> /dev/null; then
    # CentOS/RHEL 8+系统
    # 添加Kubernetes仓库
    echo "=== 添加Kubernetes仓库 ==="
    sudo cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
exclude=kubelet kubeadm kubectl
EOF
    
    # 更新仓库缓存
    sudo dnf clean all
    sudo dnf makecache -y
    
    # 安装Kubernetes组件，使用正确的版本格式（没有v前缀）
    sudo dnf install -y kubelet-${KUBE_VERSION} kubeadm-${KUBE_VERSION} kubectl-${KUBE_VERSION} --disableexcludes=kubernetes
    sudo systemctl enable --now kubelet
elif command -v yum &> /dev/null; then
    # CentOS/RHEL 7系统
    # 添加Kubernetes仓库
    echo "=== 添加Kubernetes仓库 ==="
    sudo cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
exclude=kubelet kubeadm kubectl
EOF
    
    # 更新仓库缓存
    sudo yum clean all
    sudo yum makecache -y
    
    # 安装Kubernetes组件，使用正确的版本格式（没有v前缀）
    sudo yum install -y kubelet-${KUBE_VERSION} kubeadm-${KUBE_VERSION} kubectl-${KUBE_VERSION} --disableexcludes=kubernetes
    sudo systemctl enable --now kubelet
fi`

	// 默认Kubernetes初始化脚本
	m.scripts["k8s_init"] = `# 初始化Kubernetes集群
# 执行kubeadm init
echo "=== 执行kubeadm init ==="
sudo kubeadm init --kubernetes-version=${version} --image-repository=registry.aliyuncs.com/google_containers --cri-socket=unix:///run/containerd/containerd.sock --pod-network-cidr=10.244.0.0/16 --upload-certs

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

// GetDefaultScripts 获取默认脚本（包含最新的完整脚本）
func (m *ScriptManager) GetDefaultScripts() map[string]string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 优先使用latestDefaultScripts，如果不存在则使用当前scripts
	defaultScripts := make(map[string]string)

	// 合并latestDefaultScripts和当前脚本，latestDefaultScripts优先
	for k, v := range m.scripts {
		defaultScripts[k] = v
	}
	for k, v := range latestDefaultScripts {
		defaultScripts[k] = v
	}

	return defaultScripts
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

	// 初始化或更新包级别的默认脚本映射
	if latestDefaultScripts == nil {
		latestDefaultScripts = make(map[string]string)
	}

	// 默认系统准备脚本
	latestDefaultScripts["system_prep"] = `# 系统准备脚本
# 禁用swap
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab

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
    sudo sed -i 's/^SELINUX=disabled$/SELINUX=permissive/' /etc/selinux/config 2>/dev/null || true
    # 验证SELinux配置
    sudo grep -E '^SELINUX=' /etc/selinux/config 2>/dev/null || true
    # 再次确认SELinux状态
    sudo getenforce 2>/dev/null || true
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
    elif command -v dnf &> /dev/null || command -v yum &> /dev/null; then
        # CentOS/RHEL系统
        echo "=== 添加Docker仓库 ==="
        # 安装必要的依赖
        if command -v dnf &> /dev/null; then
            sudo dnf install -y dnf-plugins-core
            sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            sudo dnf install -y containerd.io
        else
            sudo yum install -y yum-utils
            sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            sudo yum install -y containerd.io
        fi
    else
        echo "=== 警告: 不支持的包管理器，尝试手动安装containerd ==="
        # 尝试从GitHub下载并安装containerd
        if command -v curl &> /dev/null && command -v tar &> /dev/null; then
            CONTAINERD_VERSION="1.6.28"
            ARCH="amd64"
            echo "从GitHub下载containerd v${CONTAINERD_VERSION}..."
            sudo curl -fsSL -o /tmp/containerd.tar.gz https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz
            sudo mkdir -p /usr/local/bin /usr/local/lib /etc/containerd
            sudo tar Cxzvf /usr/local /tmp/containerd.tar.gz
            sudo rm -f /tmp/containerd.tar.gz
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
else
    echo "containerd已安装，跳过安装步骤"
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

# 6. 启动前先停止可能运行的containerd服务
echo "6. 停止可能运行的containerd服务..."
# 使用systemctl stop安全停止containerd服务
if command -v systemctl &> /dev/null; then
    sudo systemctl stop containerd || true
else
    # 如果没有systemctl，尝试使用service命令
    if command -v service &> /dev/null; then
        sudo service containerd stop || true
    else
        # 最后尝试使用pkill，但更精确地匹配进程
        sudo pkill -x containerd || true
    fi
fi
sleep 1

# 7. 清理旧的containerd socket和状态文件
echo "7. 清理旧的containerd socket和状态文件..."
# 只清理socket和运行时状态文件，保留数据目录
sudo rm -rf /run/containerd /var/run/containerd
sudo mkdir -p /run/containerd /var/run/containerd

# 8. 启动并启用containerd服务
echo "8. 启动并启用containerd服务..."
sudo systemctl daemon-reload
sudo systemctl enable containerd
# 使用--now参数确保立即启动
sudo systemctl start containerd

# 9. 等待containerd启动，减少等待时间
echo "9. 等待containerd启动..."
sleep 8

# 10. 检查containerd状态
echo "=== 检查containerd状态 ==="
cri_socket="/run/containerd/containerd.sock"
containerd_ready=false

# 快速检查socket是否存在
if [ -S "$cri_socket" ]; then
    containerd_ready=true
    echo "✓ CRI socket $cri_socket 存在"
else
    # 如果socket不存在，再检查systemctl状态
    if command -v systemctl &> /dev/null; then
        systemctl_status=$(sudo systemctl is-active containerd 2>/dev/null || echo "inactive")
        echo "containerd服务状态: $systemctl_status"
        
        if [ "$systemctl_status" = "active" ]; then
            # 服务已启动但socket不存在，等待3秒后再次检查
            echo "服务已启动，等待socket创建..."
            sleep 3
            if [ -S "$cri_socket" ]; then
                containerd_ready=true
                echo "✓ CRI socket $cri_socket 现在存在"
            fi
        fi
    fi
fi

# 11. 如果containerd未就绪，尝试一次修复
if [ "$containerd_ready" = false ]; then
    echo "=== 尝试修复containerd配置 ==="
    # 重新生成配置
    sudo containerd config default | sudo tee /etc/containerd/config.toml
    sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
    
    # 重启服务
    sudo systemctl restart containerd
    echo "等待3秒让containerd重启..."
    sleep 3
    
    # 再次检查
    if [ -S "$cri_socket" ]; then
        containerd_ready=true
        echo "✓ 修复成功，CRI socket $cri_socket 已创建"
    else
        # 最后尝试手动启动一次
        echo "尝试手动启动containerd..."
        sudo pkill -x containerd || true
        sleep 1
        sudo rm -rf /run/containerd /var/run/containerd
        sudo mkdir -p /run/containerd /var/run/containerd
        
        # 手动启动containerd
        nohup sudo containerd > /tmp/containerd.log 2>&1 &
        CONTAINERD_PID=$!
        echo "containerd进程ID: $CONTAINERD_PID"
        
        # 等待5秒
        sleep 5
        
        if [ -S "$cri_socket" ]; then
            containerd_ready=true
            echo "✓ 手动启动成功，CRI socket $cri_socket 已创建"
        else
            echo "✗ 最终失败: CRI socket $cri_socket 仍然不存在"
            echo "=== 显示containerd日志 ==="
            if command -v journalctl &> /dev/null; then
                sudo journalctl -u containerd --no-pager -n 30
            fi
            cat /tmp/containerd.log | head -n 50
            exit 1
        fi
    fi
fi

# 12. 测试containerd连接
echo "=== 测试containerd连接 ==="
if command -v ctr &> /dev/null; then
    ctr version || echo "ctr version命令执行失败，但containerd socket已存在"
elif [ -f /usr/local/bin/ctr ]; then
    /usr/local/bin/ctr version || echo "ctr version命令执行失败，但containerd socket已存在"
else
    echo "无法找到ctr命令，跳过连接测试"
fi
echo "✓ containerd配置完成"`

	// 添加Kubernetes组件安装脚本
	latestDefaultScripts["k8s_components"] = `# Kubernetes组件安装脚本
echo "=== 安装Kubernetes组件 ==="
# 确保所有命令都使用sudo权限
if command -v apt-get &> /dev/null; then
    # Ubuntu/Debian系统
    sudo apt-get update -y
    sudo apt-get install -y kubelet=${version} kubeadm=${version} kubectl=${version}
    sudo systemctl enable --now kubelet
elif command -v dnf &> /dev/null; then
    # CentOS/RHEL 8+系统
    sudo dnf install -y kubelet-${version} kubeadm-${version} kubectl-${version} --disableexcludes=kubernetes
    sudo systemctl enable --now kubelet
elif command -v yum &> /dev/null; then
    # CentOS/RHEL 7系统
    sudo yum install -y kubelet-${version} kubeadm-${version} kubectl-${version} --disableexcludes=kubernetes
    sudo systemctl enable --now kubelet
fi`

	// 添加Kubernetes初始化脚本
	latestDefaultScripts["k8s_init"] = `# 初始化Kubernetes集群
# 执行kubeadm init
echo "=== 执行kubeadm init ==="
sudo kubeadm init --kubernetes-version=${version} --image-repository=registry.aliyuncs.com/google_containers --cri-socket=unix:///run/containerd/containerd.sock --pod-network-cidr=10.244.0.0/16 --upload-certs

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

	// 添加Worker节点加入集群脚本
	latestDefaultScripts["k8s_join"] = `# Worker节点加入集群脚本
# 执行kubeadm join将Worker节点加入集群
# 注意：此脚本需要在Master节点上生成join命令后使用

# 检查join命令是否提供
if [ -z "$1" ]; then
    echo "错误：请提供从Master节点获取的join命令作为参数"
    echo "例如：bash k8s_join.sh 'kubeadm join 192.168.1.100:6443 --token xxx --discovery-token-ca-cert-hash xxx'"
    exit 1
fi

JOIN_COMMAND="$1"
echo "=== 执行kubeadm join命令 ==="
echo "执行命令：$JOIN_COMMAND"

# 执行join命令
sudo $JOIN_COMMAND

# 检查join是否成功
if [ $? -eq 0 ]; then
    echo "=== Worker节点加入集群成功 ==="
    echo "✓ Worker节点已成功加入Kubernetes集群"
    echo ""
    echo "提示：您可以在Master节点上使用以下命令验证节点是否加入成功："
    echo "kubectl get nodes"
else
    echo "✗ Worker节点加入集群失败"
    # 显示更多错误信息
    echo "=== 显示kubelet日志 ==="
    sudo journalctl -u kubelet --no-pager -n 50
    exit 1
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
