package kubeadm

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s-installer/node"
	"k8s-installer/ssh"
)

// Node 节点信息
type Node struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	NodeType   string `json:"nodeType"`
}

// SSHConfig SSH连接配置
type SSHConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
}

// InitConfiguration 初始化配置
type InitConfiguration struct {
	LocalAPIEndpoint LocalAPIEndpoint `json:"localAPIEndpoint"`
	NodeRegistration NodeRegistration `json:"nodeRegistration"`
}

// LocalAPIEndpoint 本地API端点
type LocalAPIEndpoint struct {
	AdvertiseAddress string `json:"advertiseAddress"`
	BindPort         int    `json:"bindPort"`
}

// NodeRegistration 节点注册
type NodeRegistration struct {
	CRISocket string `json:"criSocket"`
}

// ClusterConfiguration 集群配置
type ClusterConfiguration struct {
	KubernetesVersion string     `json:"kubernetesVersion"`
	Networking        Networking `json:"networking"`
}

// Networking 网络配置
type Networking struct {
	PodSubnet     string `json:"podSubnet"`
	ServiceSubnet string `json:"serviceSubnet"`
	DNSDomain     string `json:"dnsDomain"`
}

// KubeadmConfig Kubeadm配置
type KubeadmConfig struct {
	APIVersion           string               `json:"apiVersion"`
	Kind                 string               `json:"kind"`
	InitConfiguration    InitConfiguration    `json:"initConfiguration"`
	ClusterConfiguration ClusterConfiguration `json:"clusterConfiguration"`
}

// DeployK8sCluster 部署Kubernetes集群
func DeployK8sCluster(nodes []node.Node, kubeVersion, arch, distro string, scriptManager interface{}) (string, error) {
	// 实现完整的集群部署逻辑
	var result strings.Builder

	// 1. 找出master节点和worker节点
	var masterNodes []node.Node
	var workerNodes []node.Node
	for _, node := range nodes {
		if node.NodeType == "master" {
			masterNodes = append(masterNodes, node)
		} else {
			workerNodes = append(workerNodes, node)
		}
	}

	if len(masterNodes) == 0 {
		return "", fmt.Errorf("至少需要一个master节点")
	}

	if len(masterNodes) > 1 {
		return "", fmt.Errorf("目前只支持单master节点部署")
	}

	masterNode := masterNodes[0]
	result.WriteString(fmt.Sprintf("=== 开始部署Kubernetes集群 ===\n"))
	result.WriteString(fmt.Sprintf("Master节点: %s (%s)\n", masterNode.Name, masterNode.IP))
	result.WriteString(fmt.Sprintf("Worker节点数量: %d\n", len(workerNodes)))
	result.WriteString(fmt.Sprintf("Kubernetes版本: %s\n", kubeVersion))
	result.WriteString(fmt.Sprintf("架构: %s\n", arch))
	result.WriteString(fmt.Sprintf("发行版: %s\n\n", distro))

	// 2. 为每个节点执行部署流程
	allNodes := append(masterNodes, workerNodes...)
	for _, node := range allNodes {
		result.WriteString(fmt.Sprintf("=== 部署节点: %s (%s) ===\n", node.Name, node.IP))

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
			result.WriteString(fmt.Sprintf("创建SSH客户端失败: %v\n", err))
			return result.String(), err
		}
		defer client.Close()

		// 设置节点信息，用于日志记录
		client.SetNodeInfo(node.ID, node.Name)

		// 3. 检测节点的操作系统类型
		distroCmd := `
if [ -f /etc/os-release ]; then
	. /etc/os-release
	echo $ID
fi
`
		distroOutput, err := client.RunCommand(distroCmd)
		if err != nil {
			result.WriteString(fmt.Sprintf("检测操作系统类型失败: %v\n", err))
			return result.String(), err
		}
		nodeDistro := strings.TrimSpace(distroOutput)
		result.WriteString(fmt.Sprintf("操作系统: %s\n", nodeDistro))

		// 4. 执行系统准备脚本
		result.WriteString("\n=== 执行系统准备 ===\n")
		var systemPrepCmd string
		var systemPrepFound bool
		var systemPrepScriptName string // 声明在外部，确保作用域覆盖整个函数

		// 从脚本管理器获取系统准备脚本
		if scriptManager != nil {
			if scriptGetter, ok := scriptManager.(interface {
				GetScript(name string) (string, bool)
			}); ok {
				// 尝试获取特定发行版的系统准备脚本，使用与前端完全一致的命名格式
				// 前端命名格式：${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}
				// 将步骤名称转换为小写并替换所有空格为下划线
				stepName := strings.ReplaceAll(strings.ToLower("系统准备"), " ", "_")
				systemPrepScriptName = fmt.Sprintf("%s_%s", nodeDistro, stepName)
				if script, scriptFound := scriptGetter.GetScript(systemPrepScriptName); scriptFound {
					systemPrepCmd = script
					systemPrepFound = true
					result.WriteString(fmt.Sprintf("使用自定义系统准备脚本: %s\n", systemPrepScriptName))
				} else {
					// 尝试获取通用系统准备脚本
					if script, scriptFound := scriptGetter.GetScript("system_prep"); scriptFound {
						systemPrepCmd = script
						systemPrepFound = true
						result.WriteString("使用自定义系统准备脚本\n")
					}
				}
			}
		}

		// 如果没有找到自定义脚本，使用默认脚本
		if !systemPrepFound {
			systemPrepCmd = `# 系统准备脚本
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
sudo cat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf
net.ipv4.ip_forward = 1
EOF

# 设置其他Kubernetes所需内核参数
sudo cat <<EOF > /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

sudo sysctl --system

# 验证内核参数设置
echo "=== 验证内核参数 ==="
sudo sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward`
			result.WriteString("使用默认系统准备脚本\n")
		}

		// 执行系统准备脚本并实时输出
		result.WriteString("\n=== 执行系统准备脚本 ===\n")
		// 确保systemPrepScriptName有定义
		if systemPrepScriptName == "" {
			systemPrepScriptName = "system_prep_default"
		}
		result.WriteString(fmt.Sprintf("脚本名称: %s\n", systemPrepScriptName))
		result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		systemPrepOutput, err := client.RunCommandWithOutput(systemPrepCmd, func(line string) {
			result.WriteString("[脚本输出] " + line + "\n")
			fmt.Println("[脚本输出] " + line) // 实时打印到控制台
		})
		if err != nil {
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			result.WriteString(fmt.Sprintf("系统准备脚本执行出现错误: %v\n详细输出:\n%s\n", err, systemPrepOutput))
			result.WriteString("警告: 系统准备脚本执行失败，但将继续尝试IP转发配置...\n")
			// 不返回错误，继续执行IP转发配置
		} else {
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			result.WriteString("系统准备脚本执行成功\n")
		}

		// 添加延迟，确保系统准备脚本完全执行
		result.WriteString("\n=== 等待5秒，确保系统准备脚本完全执行 ===\n")
		if _, err := client.RunCommand("sleep 5"); err != nil {
			result.WriteString(fmt.Sprintf("等待命令执行失败: %v\n", err))
		}

		// 确保IP转发配置被正确设置，即使系统准备脚本中已有配置，再单独执行一次确保生效
		result.WriteString("\n=== 执行IP转发配置脚本 ===\n")
		result.WriteString("脚本名称: ip_forward_config\n")
		result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
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
sudo modprobe br_netfilter || echo "br_netfilter模块已加载或加载失败"
sudo modprobe overlay || echo "overlay模块已加载或加载失败"

# 8. 直接写入/proc/sys/net/ipv4/ip_forward文件确保立即生效，添加重试机制
echo "7. 直接写入/proc/sys/net/ipv4/ip_forward文件确保立即生效..."
for i in {1..5}; do
    if sudo bash -c 'echo 1 > /proc/sys/net/ipv4/ip_forward'; then
        echo "✓ 直接写入/proc/sys/net/ipv4/ip_forward文件成功"
        break
    else
        echo "✗ 直接写入/proc/sys/net/ipv4/ip_forward文件失败，正在重试 ($i/5)..."
        sleep 1
    fi
done

# 9. 验证直接写入结果
echo "8. 验证直接写入结果..."
direct_value=$(cat /proc/sys/net/ipv4/ip_forward)
echo "直接写入文件后，内容为: $direct_value"

# 10. 应用所有内核参数
echo "9. 正在应用内核参数..."
sudo sysctl --system

# 11. 立即设置IP转发值，确保即时生效
echo "10. 确保IP转发即时生效..."
sudo sysctl -w net.ipv4.ip_forward=1
sudo sysctl -w net.bridge.bridge-nf-call-iptables=1
sudo sysctl -w net.bridge.bridge-nf-call-ip6tables=1

# 12. 等待2秒，确保设置生效
sleep 2

# 13. 验证内核参数设置
echo "11. 最终验证内核参数..."
sudo sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward

# 14. 再次验证sysctl值
echo "12. 再次验证sysctl值..."
sysctl_value=$(sudo sysctl -n net.ipv4.ip_forward)
echo "sysctl获取的IP转发值: $sysctl_value"

# 15. 再次检查/proc/sys/net/ipv4/ip_forward文件内容
echo "13. 再次检查/proc/sys/net/ipv4/ip_forward文件内容..."
proc_value=$(cat /proc/sys/net/ipv4/ip_forward)
echo "/proc/sys/net/ipv4/ip_forward文件内容: $proc_value"

# 16. 验证文件权限
echo "14. 验证配置文件权限..."
sudo ls -la /etc/sysctl.d/99-kubernetes-ipforward.conf /etc/sysctl.d/k8s.conf 2>/dev/null || echo "配置文件可能未生成"

# 17. 列出/etc/sysctl.d目录下的所有配置文件，确认文件已生成
echo "15. 列出/etc/sysctl.d目录下的所有配置文件..."
sudo ls -la /etc/sysctl.d/

# 18. 最终确认IP转发状态
echo "16. 最终确认IP转发状态..."
if [ "$proc_value" = "1" ] && [ "$sysctl_value" = "1" ]; then
    echo "✓ IP转发已成功设置为1"
else
    echo "✗ IP转发设置失败，当前值: proc=$proc_value, sysctl=$sysctl_value"
    # 最后一次尝试
echo "进行最后一次修复尝试..."
sudo bash -c 'echo 1 > /proc/sys/net/ipv4/ip_forward'
sudo sysctl -w net.ipv4.ip_forward=1
final_value=$(cat /proc/sys/net/ipv4/ip_forward)
echo "最后尝试后的值: $final_value"
fi
`
		ensureIpForwardOutput, err := client.RunCommandWithOutput(ensureIpForwardCmd, func(line string) {
			result.WriteString("[脚本输出] " + line + "\n")
			fmt.Println("[脚本输出] " + line) // 实时打印到控制台
		})
		if err != nil {
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			result.WriteString(fmt.Sprintf("IP转发配置脚本执行出现错误: %v\n详细输出:\n%s\n", err, ensureIpForwardOutput))
			// 不返回错误，继续执行，因为我们将在init阶段再次检查
		} else {
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			result.WriteString("IP转发配置脚本执行成功\n")
			// 检查配置文件是否生成
			if !strings.Contains(ensureIpForwardOutput, "✓ 配置文件已生成") {
				result.WriteString("警告: 配置文件可能未成功生成，请检查目标服务器\n")
			}
		}

		// 添加延迟，确保IP转发配置完全生效
		result.WriteString("\n=== 等待3秒，确保IP转发配置完全生效 ===\n")
		if _, err := client.RunCommand("sleep 3"); err != nil {
			result.WriteString(fmt.Sprintf("等待命令执行失败: %v\n", err))
		}

		// 最终验证IP转发状态
		result.WriteString("\n=== 最终验证IP转发状态 ===\n")
		finalCheckCmd := `
# 最终验证IP转发状态
final_ip_forward=$(sudo sysctl -n net.ipv4.ip_forward)
echo "最终IP转发值: $final_ip_forward"

# 检查/proc/sys/net/ipv4/ip_forward文件内容
echo "=== 检查/proc/sys/net/ipv4/ip_forward文件内容 ==="
cat /proc/sys/net/ipv4/ip_forward
`
		finalCheckOutput, err := client.RunCommandWithOutput(finalCheckCmd, func(line string) {
			result.WriteString(line + "\n")
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			result.WriteString(fmt.Sprintf("最终IP转发验证失败: %v\n输出: %s\n", err, finalCheckOutput))
			// 不返回错误，继续执行
		} else {
			result.WriteString("最终IP转发验证完成\n")
		}

		// 5. 执行容器运行时安装脚本
		result.WriteString("\n=== 安装容器运行时 ===\n")
		var containerdInstallCmd string
		var containerdInstallFound bool
		var containerdInstallScriptName string // 声明在外部，确保作用域覆盖整个函数

		// 从脚本管理器获取容器运行时安装脚本
		if scriptManager != nil {
			if scriptGetter, ok := scriptManager.(interface {
				GetScript(name string) (string, bool)
			}); ok {
				// 尝试获取特定发行版的容器运行时安装脚本，使用与前端完全一致的命名格式
				// 前端命名格式：${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}
				// 将步骤名称转换为小写并替换所有空格为下划线
				stepName := strings.ReplaceAll(strings.ToLower("安装容器运行时"), " ", "_")
				containerdInstallScriptName = fmt.Sprintf("%s_%s", nodeDistro, stepName)
				if script, scriptFound := scriptGetter.GetScript(containerdInstallScriptName); scriptFound {
					containerdInstallCmd = script
					containerdInstallFound = true
					result.WriteString(fmt.Sprintf("使用自定义容器运行时安装脚本: %s\n", containerdInstallScriptName))
				} else {
					// 尝试获取通用容器运行时安装脚本
					if script, scriptFound := scriptGetter.GetScript("containerd_install"); scriptFound {
						containerdInstallCmd = script
						containerdInstallFound = true
						result.WriteString("使用自定义容器运行时安装脚本\n")
					}
				}
			}
		}

		// 如果没有找到自定义脚本，使用默认脚本
		if !containerdInstallFound {
			containerdInstallCmd = `# containerd安装脚本
echo "=== 安装containerd ==="
if ! command -v containerd &> /dev/null; then
    echo "containerd未安装，正在安装..."
    if command -v apt-get &> /dev/null; then
        # Ubuntu/Debian系统
        echo "=== 使用apt-get安装containerd ==="
        apt update -y
        apt install -y containerd
    elif command -v dnf &> /dev/null; then
        # CentOS/RHEL 8+系统
        echo "=== 使用dnf安装containerd ==="
        dnf install -y containerd
    elif command -v yum &> /dev/null; then
        # CentOS/RHEL 7系统
        echo "=== 使用yum安装containerd ==="
        yum install -y containerd
    else
        echo "=== 警告: 不支持的包管理器，尝试手动安装containerd ==="
        # 尝试从GitHub下载并安装containerd
        if command -v curl &> /dev/null && command -v tar &> /dev/null; then
            CONTAINERD_VERSION="1.6.28"
            ARCH="amd64"
            echo "从GitHub下载containerd v${CONTAINERD_VERSION}..."
            curl -L https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz -o /tmp/containerd.tar.gz
            mkdir -p /usr/local/bin /usr/local/lib /etc/containerd
            tar Cxzvf /usr/local /tmp/containerd.tar.gz
            # 创建systemd服务文件
            cat > /etc/systemd/system/containerd.service <<-'EOF'
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
            systemctl daemon-reload
            systemctl enable containerd
        fi
    fi
fi`
			result.WriteString("使用默认容器运行时安装脚本\n")
		}

		// 执行容器运行时安装脚本并实时输出
		result.WriteString("\n=== 执行容器运行时安装脚本 ===\n")
		// 确保containerdInstallScriptName有定义
		if containerdInstallScriptName == "" {
			containerdInstallScriptName = "containerd_install_default"
		}
		result.WriteString(fmt.Sprintf("脚本名称: %s\n", containerdInstallScriptName))
		result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		containerdInstallOutput, err := client.RunCommandWithOutput(containerdInstallCmd, func(line string) {
			result.WriteString("[脚本输出] " + line + "\n")
			fmt.Println("[脚本输出] " + line) // 实时打印到控制台
		})
		if err != nil {
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			result.WriteString(fmt.Sprintf("容器运行时安装失败: %v\n详细输出:\n%s\n", err, containerdInstallOutput))
			return result.String(), err
		}
		result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		result.WriteString("容器运行时安装成功\n")

		// 5. 执行容器运行时配置脚本
		result.WriteString("\n=== 配置容器运行时 ===\n")
		var containerdConfigCmd string
		var containerdConfigFound bool
		var containerdConfigScriptName string

		// 从脚本管理器获取容器运行时配置脚本
		if scriptManager != nil {
			if scriptGetter, ok := scriptManager.(interface {
				GetScript(name string) (string, bool)
			}); ok {
				// 尝试获取特定发行版的容器运行时配置脚本，使用与前端完全一致的命名格式
				stepName := strings.ReplaceAll(strings.ToLower("配置容器运行时"), " ", "_")
				containerdConfigScriptName = fmt.Sprintf("%s_%s", nodeDistro, stepName)
				if script, scriptFound := scriptGetter.GetScript(containerdConfigScriptName); scriptFound {
					containerdConfigCmd = script
					containerdConfigFound = true
					result.WriteString(fmt.Sprintf("使用自定义容器运行时配置脚本: %s\n", containerdConfigScriptName))
				} else {
					// 尝试获取通用容器运行时配置脚本
					if script, scriptFound := scriptGetter.GetScript("containerd_config"); scriptFound {
						containerdConfigCmd = script
						containerdConfigFound = true
						result.WriteString("使用自定义容器运行时配置脚本\n")
					}
				}
			}
		}

		// 如果没有找到自定义脚本，使用默认脚本
		if !containerdConfigFound {
			containerdConfigCmd = `# containerd配置脚本
# 配置containerd
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
			result.WriteString("使用默认容器运行时配置脚本\n")
		}

		// 执行容器运行时配置脚本并实时输出
		result.WriteString("\n=== 执行containerd配置脚本 ===\n")
		// 确保containerdConfigScriptName有定义
		if containerdConfigScriptName == "" {
			containerdConfigScriptName = "containerd_config_default"
		}
		result.WriteString(fmt.Sprintf("脚本名称: %s\n", containerdConfigScriptName))
		result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		containerdConfigOutput, err := client.RunCommandWithOutput(containerdConfigCmd, func(line string) {
			result.WriteString("[脚本输出] " + line + "\n")
			fmt.Println("[脚本输出] " + line) // 实时打印到控制台
		})
		if err != nil {
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			result.WriteString(fmt.Sprintf("容器运行时配置失败: %v\n详细输出:\n%s\n", err, containerdConfigOutput))
			return result.String(), err
		}
		result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		result.WriteString("容器运行时配置成功\n")

		// 7. 添加Kubernetes仓库
		result.WriteString("\n=== 添加Kubernetes仓库 ===\n")
		var addK8sRepoCmd string
		var addK8sRepoFound bool
		var addK8sRepoScriptName string // 声明在外部，确保作用域覆盖整个函数

		// 从脚本管理器获取添加Kubernetes仓库脚本
		if scriptManager != nil {
			if scriptGetter, ok := scriptManager.(interface {
				GetScript(name string) (string, bool)
			}); ok {
				// 尝试获取特定发行版的添加Kubernetes仓库脚本，使用与前端完全一致的命名格式
				// 前端命名格式：${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}
				// 将步骤名称转换为小写并替换所有空格为下划线
				stepName := strings.ReplaceAll(strings.ToLower("添加kubernetes仓库"), " ", "_")
				addK8sRepoScriptName = fmt.Sprintf("%s_%s", nodeDistro, stepName)
				if script, scriptFound := scriptGetter.GetScript(addK8sRepoScriptName); scriptFound {
					addK8sRepoCmd = script
					addK8sRepoFound = true
					result.WriteString(fmt.Sprintf("使用自定义添加Kubernetes仓库脚本: %s\n", addK8sRepoScriptName))
				}
			}
		}

		// 如果没有找到自定义脚本，使用默认脚本
		if !addK8sRepoFound {
			// 根据发行版选择不同的添加仓库命令
			switch nodeDistro {
			case "ubuntu", "debian":
				addK8sRepoCmd = fmt.Sprintf(`# 添加Kubernetes仓库（Ubuntu/Debian）
echo "=== 添加Kubernetes仓库 ==="
apt-get update -y
apt-get install -y apt-transport-https ca-certificates curl gpg

# 创建keyring目录
mkdir -p -m 755 /etc/apt/keyrings

# 下载并安装GPG密钥
curl -fsSL -L https://pkgs.k8s.io/core:/stable:/%s/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# 添加Kubernetes repo
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/%s/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list

# 更新仓库缓存
apt-get update -y`, kubeVersion, kubeVersion)
			case "centos", "rhel", "rocky", "almalinux":
				addK8sRepoCmd = fmt.Sprintf(`# 添加Kubernetes仓库（CentOS/RHEL/Rocky/AlmaLinux）
echo "=== 添加Kubernetes仓库 ==="
cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/%s/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/%s/rpm/repodata/repomd.xml.key
exclude=kubelet kubeadm kubectl
EOF

# 更新仓库缓存
if command -v dnf &> /dev/null; then
    dnf makecache -y
else
    yum makecache -y
fi`, kubeVersion, kubeVersion)
			default:
				result.WriteString(fmt.Sprintf("不支持的发行版: %s\n", nodeDistro))
				return result.String(), fmt.Errorf("不支持的发行版: %s", nodeDistro)
			}
			result.WriteString("使用默认添加Kubernetes仓库脚本\n")
		}

		// 执行添加Kubernetes仓库脚本并实时输出
		result.WriteString("\n=== 执行添加Kubernetes仓库脚本 ===\n")
		// 确保addK8sRepoScriptName有定义
		if addK8sRepoScriptName == "" {
			addK8sRepoScriptName = "add_k8s_repo_default"
		}
		result.WriteString(fmt.Sprintf("脚本名称: %s\n", addK8sRepoScriptName))
		result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		addK8sRepoOutput, err := client.RunCommandWithOutput(addK8sRepoCmd, func(line string) {
			result.WriteString("[脚本输出] " + line + "\n")
			fmt.Println("[脚本输出] " + line) // 实时打印到控制台
		})
		if err != nil {
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			result.WriteString(fmt.Sprintf("添加Kubernetes仓库失败: %v\n详细输出:\n%s\n", err, addK8sRepoOutput))
			return result.String(), err
		}
		result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		result.WriteString("添加Kubernetes仓库成功\n")

		// 添加延迟，确保仓库更新完全执行
		result.WriteString("\n=== 等待3秒，确保仓库更新完全执行 ===\n")
		if _, err := client.RunCommand("sleep 3"); err != nil {
			result.WriteString(fmt.Sprintf("等待命令执行失败: %v\n", err))
		}

		// 8. 安装Kubernetes组件
		result.WriteString("\n=== 安装Kubernetes组件 ===\n")
		var k8sComponentsCmd string
		var k8sComponentsFound bool
		var k8sComponentsScriptName string // 声明在外部，确保作用域覆盖整个函数

		// 从脚本管理器获取Kubernetes组件安装脚本
		if scriptManager != nil {
			if scriptGetter, ok := scriptManager.(interface {
				GetScript(name string) (string, bool)
			}); ok {
				// 尝试获取特定发行版的Kubernetes组件安装脚本，使用与前端完全一致的命名格式
				// 前端命名格式：${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}
				// 将步骤名称转换为小写并替换所有空格为下划线
				stepName := strings.ReplaceAll(strings.ToLower("安装kubernetes组件"), " ", "_")
				k8sComponentsScriptName = fmt.Sprintf("%s_%s", nodeDistro, stepName)
				if script, scriptFound := scriptGetter.GetScript(k8sComponentsScriptName); scriptFound {
					k8sComponentsCmd = script
					k8sComponentsFound = true
					result.WriteString(fmt.Sprintf("使用自定义Kubernetes组件安装脚本: %s\n", k8sComponentsScriptName))
				} else {
					// 尝试获取通用Kubernetes组件安装脚本
					if script, scriptFound := scriptGetter.GetScript("k8s_components"); scriptFound {
						k8sComponentsCmd = script
						k8sComponentsFound = true
						result.WriteString("使用自定义Kubernetes组件安装脚本\n")
					} else {
						// 尝试获取旧格式的脚本，保持向后兼容
						oldK8sComponentsScriptName := fmt.Sprintf("k8s_components_%s", nodeDistro)
						if script, scriptFound := scriptGetter.GetScript(oldK8sComponentsScriptName); scriptFound {
							k8sComponentsCmd = script
							k8sComponentsFound = true
							result.WriteString(fmt.Sprintf("使用旧格式自定义Kubernetes组件安装脚本: %s\n", oldK8sComponentsScriptName))
						}
					}
				}
			}
		}

		// 如果没有找到自定义脚本，使用默认脚本
		if !k8sComponentsFound {
			// 根据发行版选择不同的安装命令
			switch nodeDistro {
			case "ubuntu", "debian":
				k8sComponentsCmd = fmt.Sprintf(`# 安装Kubernetes组件（Ubuntu/Debian）
echo "=== 添加Kubernetes仓库 ==="
apt-get update -y
apt-get install -y apt-transport-https ca-certificates curl gpg

# 创建keyring目录
mkdir -p -m 755 /etc/apt/keyrings

# 下载并安装GPG密钥
curl -fsSL -L https://pkgs.k8s.io/core:/stable:/%s/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# 添加Kubernetes repo
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/%s/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list

# 更新仓库缓存
apt-get update -y

# 安装Kubernetes组件
echo "=== 安装kubelet、kubeadm和kubectl ==="
apt-get install -y kubelet kubeadm kubectl

# 启动kubelet
echo "=== 启动kubelet服务 ==="
systemctl enable --now kubelet`, kubeVersion, kubeVersion)
			case "centos", "rhel", "rocky", "almalinux":
				k8sComponentsCmd = fmt.Sprintf(`# 安装Kubernetes组件（CentOS/RHEL/Rocky/AlmaLinux）
echo "=== 添加Kubernetes仓库 ==="
cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/%s/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/%s/rpm/repodata/repomd.xml.key
exclude=kubelet kubeadm kubectl
EOF

# 更新仓库缓存
if command -v dnf &> /dev/null; then
    dnf makecache -y
else
    yum makecache -y
fi

# 安装Kubernetes组件
echo "=== 安装kubelet、kubeadm和kubectl ==="
if command -v dnf &> /dev/null; then
    dnf install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
else
    yum install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
fi

# 启动kubelet
echo "=== 启动kubelet服务 ==="
systemctl enable --now kubelet`, kubeVersion, kubeVersion)
			default:
				result.WriteString(fmt.Sprintf("不支持的发行版: %s\n", nodeDistro))
				return result.String(), fmt.Errorf("不支持的发行版: %s", nodeDistro)
			}
			result.WriteString("使用默认Kubernetes组件安装脚本\n")
		}

		// 执行Kubernetes组件安装脚本并实时输出
		result.WriteString("\n=== 执行Kubernetes组件安装脚本 ===\n")
		// 确保k8sComponentsScriptName有定义
		if k8sComponentsScriptName == "" {
			k8sComponentsScriptName = "k8s_components_default"
		}
		result.WriteString(fmt.Sprintf("脚本名称: %s\n", k8sComponentsScriptName))
		result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		k8sComponentsOutput, err := client.RunCommandWithOutput(k8sComponentsCmd, func(line string) {
			result.WriteString("[脚本输出] " + line + "\n")
			fmt.Println("[脚本输出] " + line) // 实时打印到控制台
		})
		if err != nil {
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			result.WriteString(fmt.Sprintf("Kubernetes组件安装失败: %v\n详细输出:\n%s\n", err, k8sComponentsOutput))
			return result.String(), err
		}
		result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		result.WriteString("Kubernetes组件安装成功\n")

		// 添加延迟，确保Kubernetes组件安装完全执行
		result.WriteString("\n=== 等待5秒，确保Kubernetes组件安装完全执行 ===\n")
		if _, err := client.RunCommand("sleep 5"); err != nil {
			result.WriteString(fmt.Sprintf("等待命令执行失败: %v\n", err))
		}

		result.WriteString(fmt.Sprintf("=== 节点 %s 部署完成 ===\n\n", node.Name))
	}

	// 3. 初始化Master节点
	result.WriteString("=== 初始化Master节点 ===\n")
	masterSSHConfig := ssh.SSHConfig{
		Host:       masterNode.IP,
		Port:       masterNode.Port,
		Username:   masterNode.Username,
		Password:   masterNode.Password,
		PrivateKey: masterNode.PrivateKey,
	}

	masterClient, err := ssh.NewSSHClient(masterSSHConfig)
	if err != nil {
		result.WriteString(fmt.Sprintf("创建Master节点SSH客户端失败: %v\n", err))
		return result.String(), err
	}
	defer masterClient.Close()

	// 检测Master节点的操作系统类型
	result.WriteString("\n=== 检测Master节点操作系统类型 ===\n")
	distroCmd := `
if [ -f /etc/os-release ]; then
	. /etc/os-release
	echo $ID
fi
`
	masterDistro, err := masterClient.RunCommand(distroCmd)
	if err != nil {
		result.WriteString(fmt.Sprintf("检测Master节点操作系统类型失败: %v\n", err))
		return result.String(), err
	}
	masterDistro = strings.TrimSpace(masterDistro)
	result.WriteString(fmt.Sprintf("Master节点操作系统: %s\n", masterDistro))

	// 在执行init命令前再次验证和应用IP转发配置，确保万无一失
	result.WriteString("\n=== 最后验证和应用IP转发配置 ===\n")
	result.WriteString("脚本名称: final_ip_forward_verification\n")
	result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	finalIpForwardCmd := `
# 1. 确保IP转发配置文件存在并包含正确的配置，设置适当的权限
 echo "=== 再次配置IP转发 ==="
sudo bash -c 'cat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf
net.ipv4.ip_forward = 1
EOF'

# 2. 设置配置文件权限，确保系统可以读取
echo "=== 设置配置文件权限 ==="
sudo chmod 644 /etc/sysctl.d/99-kubernetes-ipforward.conf

# 3. 确保其他Kubernetes所需内核参数配置正确
echo "=== 确保其他Kubernetes内核参数配置正确 ==="
sudo bash -c 'cat <<EOF > /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF'
sudo chmod 644 /etc/sysctl.d/k8s.conf

# 4. 加载必要的内核模块，确保模块已加载
echo "=== 加载必要的内核模块 ==="
sudo modprobe br_netfilter || echo "br_netfilter模块已加载或加载失败"
sudo modprobe overlay || echo "overlay模块已加载或加载失败"

# 5. 应用所有内核参数，使用sudo确保权限
echo "=== 再次应用内核参数 ==="
sudo sysctl --system

# 6. 立即直接设置IP转发值，确保即时生效，使用bash -c确保权限
echo "=== 立即直接设置IP转发值 ==="
sudo bash -c 'sysctl -w net.ipv4.ip_forward=1'
sudo bash -c 'sysctl -w net.bridge.bridge-nf-call-iptables=1'
sudo bash -c 'sysctl -w net.bridge.bridge-nf-call-ip6tables=1'

# 7. 等待1秒，确保设置生效
sleep 1

# 8. 再次验证IP转发状态，使用bash -c确保权限
echo "=== 最终验证IP转发状态 ==="
final_ip_forward=$(sudo bash -c 'sysctl -n net.ipv4.ip_forward')
echo "最终IP转发值: $final_ip_forward"

# 9. 检查/proc/sys/net/ipv4/ip_forward文件内容，确保文件存在且内容正确，添加重试机制
        echo "=== 再次检查/proc/sys/net/ipv4/ip_forward文件内容 ==="
        # 重试写入/proc/sys/net/ipv4/ip_forward文件，最多5次
        for i in {1..5}; do
            if [ -f /proc/sys/net/ipv4/ip_forward ]; then
                echo "文件存在，当前内容为: $(cat /proc/sys/net/ipv4/ip_forward)"
                # 直接写入文件，确保内容正确
                if sudo bash -c 'echo 1 > /proc/sys/net/ipv4/ip_forward'; then
                    current_value=$(cat /proc/sys/net/ipv4/ip_forward)
                    echo "直接写入文件后，内容为: $current_value"
                    # 如果写入后值为1，退出循环
                    if [ "$current_value" = "1" ]; then
                        echo "✓ IP转发值已成功设置为1"
                        break
                    fi
                fi
            else
                echo "文件不存在，尝试创建并写入"
                sudo bash -c 'mkdir -p /proc/sys/net/ipv4'
                sudo bash -c 'echo 1 > /proc/sys/net/ipv4/ip_forward'
                echo "创建并写入后，内容为: $(cat /proc/sys/net/ipv4/ip_forward)"
            fi
            echo "✗ IP转发值设置失败，正在重试 ($i/5)..."
            sleep 1
        done
        
        # 验证最终结果
        echo "=== 验证最终IP转发设置 ==="
        final_value=$(cat /proc/sys/net/ipv4/ip_forward)
        if [ "$final_value" = "1" ]; then
            echo "✓ IP转发已成功设置，最终值为: $final_value"
        else
            echo "✗ IP转发设置失败，最终值为: $final_value"
            # 作为最后的手段，尝试使用echo命令直接写入
            echo "=== 作为最后的手段，尝试使用echo命令直接写入 ==="
            sudo sh -c "echo 1 > /proc/sys/net/ipv4/ip_forward"
            echo "最终尝试后，内容为: $(cat /proc/sys/net/ipv4/ip_forward)"
        fi

# 10. 最后再次应用所有内核参数，确保所有设置都生效
echo "=== 最后再次应用内核参数 ==="
sudo sysctl --system

# 11. 最终验证所有关键内核参数
echo "=== 最终验证所有关键内核参数 ==="
sudo bash -c 'sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward'
`
	finalIpForwardOutput, err := masterClient.RunCommandWithOutput(finalIpForwardCmd, func(line string) {
		result.WriteString("[脚本输出] " + line + "\n")
		fmt.Println("[脚本输出] " + line) // 实时打印到控制台
	})
	if err != nil {
		result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		result.WriteString(fmt.Sprintf("最后验证和应用IP转发配置失败: %v\n详细输出:\n%s\n", err, finalIpForwardOutput))
		// 不返回错误，继续执行，但会在init阶段再次检查
		result.WriteString("警告: IP转发配置验证失败，但将继续执行Master节点初始化，因为kubeadm init会再次检查\n")
	} else {
		result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
		result.WriteString("最后验证和应用IP转发配置成功\n")
		// 检查IP转发值是否确实为1
		if !strings.Contains(finalIpForwardOutput, "最终IP转发值: 1") || !strings.Contains(finalIpForwardOutput, "直接写入文件后，内容为: 1") {
			result.WriteString("警告: IP转发值可能未正确设置为1，建议检查\n")
		} else {
			result.WriteString("✓ IP转发值已正确设置为1\n")
		}
	}

	// 构建kubeadm配置
	kubeadmConfig := fmt.Sprintf(`apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: %s
networking:
  podSubnet: 10.244.0.0/16
  serviceSubnet: 10.96.0.0/12
  dnsDomain: cluster.local
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: %s
  bindPort: 6443
nodeRegistration:
  criSocket: unix:///run/containerd/containerd.sock`, kubeVersion, masterNode.IP)

	// 从脚本管理器获取初始化Kubernetes集群脚本
	var initCmd string
	var initFound bool
	var initScriptName string

	// 从脚本管理器获取Kubernetes初始化脚本
	if scriptManager != nil {
		if scriptGetter, ok := scriptManager.(interface {
			GetScript(name string) (string, bool)
		}); ok {
			// 尝试获取特定发行版的Kubernetes初始化脚本，使用与前端完全一致的命名格式
			// 前端命名格式：${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}
			// 将步骤名称转换为小写并替换所有空格为下划线
			stepName := strings.ReplaceAll(strings.ToLower("初始化kubernetes集群"), " ", "_")
			initScriptName = fmt.Sprintf("%s_%s", masterDistro, stepName)
			if script, scriptFound := scriptGetter.GetScript(initScriptName); scriptFound {
				initCmd = script
				initFound = true
				result.WriteString(fmt.Sprintf("使用自定义Kubernetes初始化脚本: %s\n", initScriptName))
			}
		}
	}

	// 如果没有找到自定义脚本，使用默认脚本
	if !initFound {
		initCmd = fmt.Sprintf(`cat > /tmp/kubeadm-config.yaml << 'EOF'
%s
EOF

# 重置集群，清理旧配置
				echo "=== 重置集群，清理旧配置 ==="
				sudo kubeadm reset --force
				
				# 清理CNI配置
				echo "=== 清理CNI配置 ==="
				sudo rm -rf /etc/cni/net.d
				
				# 重置iptables规则
				echo "=== 重置iptables规则 ==="
				sudo iptables -F
				sudo iptables -t nat -F
				sudo iptables -t mangle -F
				sudo iptables -X
				
				# 重置ip6tables规则
				echo "=== 重置ip6tables规则 ==="
				sudo ip6tables -F
				sudo ip6tables -t nat -F
				sudo ip6tables -t mangle -F
				sudo ip6tables -X
				
				# 如果使用IPVS，重置IPVS表
				echo "=== 重置IPVS表 ==="
				if command -v ipvsadm &> /dev/null; then
				    sudo ipvsadm --clear
				fi
				
				# 清理kubeconfig文件
				echo "=== 清理kubeconfig文件 ==="
				sudo rm -rf ~/.kube
				rm -rf $HOME/.kube
				
				# 清理集群配置文件
				echo "=== 清理集群配置文件 ==="
				sudo rm -f /etc/kubernetes/admin.conf
				sudo rm -f /etc/kubernetes/kubelet.conf
				sudo rm -f /etc/kubernetes/controller-manager.conf
				sudo rm -f /etc/kubernetes/scheduler.conf
				sudo rm -rf /etc/kubernetes/manifests
				
				# 清理旧的etcd数据
				echo "=== 清理旧的etcd数据 ==="
				sudo rm -rf /var/lib/etcd
				
				# 清理旧的kubelet数据
				echo "=== 清理旧的kubelet数据 ==="
				sudo rm -rf /var/lib/kubelet

# 在执行kubeadm init前检查并确保containerd正常运行
echo "=== 检查并确保containerd正常运行 ==="

# 1. 检查containerd服务状态
echo "1. 检查containerd服务状态..."
containerd_status=$(sudo systemctl is-active containerd 2>/dev/null || echo "inactive")
echo "containerd服务状态: $containerd_status"

# 2. 如果containerd没有运行，尝试启动它
if [ "$containerd_status" != "active" ]; then
    echo "2. containerd未运行，尝试启动..."
    sudo systemctl daemon-reload
    sudo systemctl start containerd
    # 等待5秒让containerd启动
    sleep 5
    # 再次检查状态
    containerd_status=$(sudo systemctl is-active containerd 2>/dev/null || echo "inactive")
    echo "启动后containerd服务状态: $containerd_status"
fi

# 3. 检查containerd socket是否存在
echo "3. 检查containerd socket是否存在..."
cri_socket="/run/containerd/containerd.sock"
if [ ! -S "$cri_socket" ]; then
    echo "4. containerd socket不存在，尝试手动启动containerd..."
    # 停止可能存在的containerd进程
    sudo pkill -f containerd || true
    sleep 2
    # 清理旧的socket和状态文件
    sudo rm -rf /run/containerd /var/run/containerd
    sudo mkdir -p /var/run/containerd
    # 手动启动containerd
    containerd --version
    containerd &
    # 等待10秒让containerd启动
    sleep 10
    # 再次检查socket
    if [ -S "$cri_socket" ]; then
        echo "5. 手动启动成功，containerd socket已创建"
    else
        echo "6. 手动启动失败，containerd socket仍不存在"
        echo "=== 显示containerd日志 ==="
        sudo journalctl -u containerd --no-pager -n 50
        echo "=== 尝试使用systemd状态检查 ==="
        sudo systemctl status containerd --no-pager
        echo "✗ 无法启动containerd，kubeadm init将失败"
        exit 1
    fi
else
    echo "4. containerd socket已存在"
fi

# 5. 测试containerd连接
echo "5. 测试containerd连接..."
if command -v ctr &> /dev/null; then
    ctr_version=$(ctr version 2>&1 || echo "连接失败")
    echo "containerd版本信息: $ctr_version"
fi

# 6. 最终确认containerd状态
echo "6. 最终确认containerd状态..."
final_status=$(sudo systemctl is-active containerd 2>/dev/null || echo "inactive")
final_socket=$(if [ -S "$cri_socket" ]; then echo "存在"; else echo "不存在"; fi)
echo "最终containerd服务状态: $final_status"
echo "最终containerd socket状态: $final_socket"

# 初始化Master节点
echo "=== 执行kubeadm init ==="
sudo kubeadm init --config /tmp/kubeadm-config.yaml --upload-certs

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
fi`, kubeadmConfig)
		result.WriteString("使用默认Kubernetes初始化脚本\n")
	}

	initOutput, err := masterClient.RunCommandWithOutput(initCmd, func(line string) {
		result.WriteString(line + "\n")
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		result.WriteString(fmt.Sprintf("Master节点初始化失败: %v\n输出: %s\n", err, initOutput))
		return result.String(), err
	}
	result.WriteString("Master节点初始化成功\n\n")

	// 4. 获取Join命令
	result.WriteString("=== 获取Join命令 ===\n")
	joinCmdCmd := `kubeadm token create --print-join-command`
	joinCmd, err := masterClient.RunCommand(joinCmdCmd)
	if err != nil {
		result.WriteString(fmt.Sprintf("获取Join命令失败: %v\n", err))
		return result.String(), err
	}
	joinCmd = strings.TrimSpace(joinCmd)
	result.WriteString(fmt.Sprintf("Join命令: %s\n\n", joinCmd))

	// 5. 将Worker节点加入集群
	for _, workerNode := range workerNodes {
		result.WriteString(fmt.Sprintf("=== 将Worker节点 %s 加入集群 ===\n", workerNode.Name))

		// 创建SSH客户端
		workerSSHConfig := ssh.SSHConfig{
			Host:       workerNode.IP,
			Port:       workerNode.Port,
			Username:   workerNode.Username,
			Password:   workerNode.Password,
			PrivateKey: workerNode.PrivateKey,
		}

		workerClient, err := ssh.NewSSHClient(workerSSHConfig)
		if err != nil {
			result.WriteString(fmt.Sprintf("创建Worker节点SSH客户端失败: %v\n", err))
			return result.String(), err
		}
		defer workerClient.Close()

		// 将Worker节点加入集群
		joinOutput, err := workerClient.RunCommandWithOutput(joinCmd, func(line string) {
			result.WriteString(line + "\n")
			fmt.Println(line) // 实时打印到控制台
		})
		if err != nil {
			result.WriteString(fmt.Sprintf("Worker节点 %s 加入集群失败: %v\n输出: %s\n", workerNode.Name, err, joinOutput))
			return result.String(), err
		}
		result.WriteString(fmt.Sprintf("Worker节点 %s 加入集群成功\n\n", workerNode.Name))
	}

	// 6. 验证集群状态
	result.WriteString("=== 验证集群状态 ===\n")
	verifyCmd := `# 验证集群状态
 echo "=== 等待集群就绪（60秒） ==="
 sleep 60
 
 echo "=== 查看节点状态 ==="
 kubectl get nodes
 
 echo "=== 查看Pod状态 ==="
 kubectl get pods -A`

	verifyOutput, err := masterClient.RunCommandWithOutput(verifyCmd, func(line string) {
		result.WriteString(line + "\n")
		fmt.Println(line) // 实时打印到控制台
	})
	if err != nil {
		result.WriteString(fmt.Sprintf("验证集群状态失败: %v\n输出: %s\n", err, verifyOutput))
		// 验证失败不影响部署流程，继续执行
	}

	result.WriteString("=== Kubernetes集群部署完成 ===\n")
	result.WriteString(fmt.Sprintf("Master节点: %s (%s)\n", masterNode.Name, masterNode.IP))
	result.WriteString(fmt.Sprintf("Worker节点数量: %d\n", len(workerNodes)))
	result.WriteString(fmt.Sprintf("Kubernetes版本: %s\n", kubeVersion))

	return result.String(), nil
}

// DownloadKubeadmPackage 下载Kubeadm包
func DownloadKubeadmPackage(version, arch, distro, sourceURL string, log func(format string, args ...interface{})) (string, error) {
	// 简化实现，返回一个固定路径
	return GetPackagePath("kubeadm", version, arch, distro), nil
}

// DeployKubeadmPackage 部署Kubeadm包到远程节点
func DeployKubeadmPackage(packagePath, nodeIP, username, password string, port int, privateKey string, log func(format string, args ...interface{})) error {
	// 简化实现，直接返回成功
	log("部署Kubeadm包到节点: %s", nodeIP)
	return nil
}

// RunCommandOnRemoteWithOutput 在远程节点执行命令并实时输出结果
func RunCommandOnRemoteWithOutput(sshConfig SSHConfig, callback ssh.OutputCallback, cmd ...string) (string, error) {
	// 创建SSH客户端
	client, err := ssh.NewSSHClient(ssh.SSHConfig{
		Host:       sshConfig.Host,
		Port:       sshConfig.Port,
		Username:   sshConfig.Username,
		Password:   sshConfig.Password,
		PrivateKey: sshConfig.PrivateKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	// 执行命令，实时获取输出
	return client.RunCommandWithOutput(strings.Join(cmd, " "), callback)
}

// RunCommandOnRemote 在远程节点执行命令
func RunCommandOnRemote(sshConfig SSHConfig, cmd ...string) (string, error) {
	// 创建SSH客户端
	client, err := ssh.NewSSHClient(ssh.SSHConfig{
		Host:       sshConfig.Host,
		Port:       sshConfig.Port,
		Username:   sshConfig.Username,
		Password:   sshConfig.Password,
		PrivateKey: sshConfig.PrivateKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	// 执行命令
	return client.RunCommand(strings.Join(cmd, " "))
}

// InitMaster 初始化master节点
func InitMaster(sshConfig SSHConfig, config KubeadmConfig) (string, error) {
	// 构建kubeadm配置文件内容
	// 使用正确的kubeadm配置格式，每个配置对象都有自己的apiVersion和kind
	// 使用---分隔符分隔不同的配置对象
	kubeadmConfig := `apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: ` + config.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress + `
  bindPort: ` + strconv.Itoa(config.InitConfiguration.LocalAPIEndpoint.BindPort) + `
nodeRegistration:
  criSocket: ` + config.InitConfiguration.NodeRegistration.CRISocket + `
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: ` + config.ClusterConfiguration.KubernetesVersion + `
networking:
  podSubnet: ` + config.ClusterConfiguration.Networking.PodSubnet + `
  serviceSubnet: ` + config.ClusterConfiguration.Networking.ServiceSubnet + `
  dnsDomain: ` + config.ClusterConfiguration.Networking.DNSDomain + `
`

	// 构建完整的执行命令，包含重置步骤
	cmd := `# 重置集群，清理旧配置
sudo kubeadm reset --force

# 清理CNI配置
sudo rm -rf /etc/cni/net.d

# 重置iptables规则
sudo iptables -F
sudo iptables -t nat -F
sudo iptables -t mangle -F
sudo iptables -X

# 重置ip6tables规则
sudo ip6tables -F
sudo ip6tables -t nat -F
sudo ip6tables -t mangle -F
sudo ip6tables -X

# 如果使用IPVS，重置IPVS表
if command -v ipvsadm &> /dev/null; then
    sudo ipvsadm --clear
fi

# 清理kubeconfig文件
sudo rm -rf ~/.kube
rm -rf $HOME/.kube
# 清理集群配置文件
sudo rm -f /etc/kubernetes/admin.conf
sudo rm -f /etc/kubernetes/kubelet.conf
sudo rm -f /etc/kubernetes/controller-manager.conf
sudo rm -f /etc/kubernetes/scheduler.conf
sudo rm -rf /etc/kubernetes/manifests

# 清理旧的etcd数据
sudo rm -rf /var/lib/etcd

# 清理旧的kubelet数据
sudo rm -rf /var/lib/kubelet

cat > /tmp/kubeadm-config.yaml << 'EOF'
` + kubeadmConfig + `EOF

# 初始化master节点
kubeadm init --config /tmp/kubeadm-config.yaml

# 配置kubectl
mkdir -p $HOME/.kube
cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
chown $(id -u):$(id -g) $HOME/.kube/config
`

	// 创建SSH客户端
	client, err := ssh.NewSSHClient(ssh.SSHConfig{
		Host:       sshConfig.Host,
		Port:       sshConfig.Port,
		Username:   sshConfig.Username,
		Password:   sshConfig.Password,
		PrivateKey: sshConfig.PrivateKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create SSH client: %v", err)
	}
	defer client.Close()

	// 执行命令并实时输出
	var fullOutput strings.Builder
	_, err = client.RunCommandWithOutput(cmd, func(line string) {
		fullOutput.WriteString(line + "\n")
		fmt.Println(line) // 实时打印到控制台
	})

	if err != nil {
		return fullOutput.String(), err
	}

	return fullOutput.String(), nil
}

// GetJoinCommand 获取join命令
func GetJoinCommand(sshConfig SSHConfig) (string, error) {
	cmd := `kubeadm token create --print-join-command`
	return RunCommandOnRemote(sshConfig, "bash", "-c", cmd)
}

// JoinWorker 将worker节点加入集群
func JoinWorker(sshConfig SSHConfig, token, caCertHash, controlPlaneEndpoint string) (string, error) {
	cmd := fmt.Sprintf(`kubeadm join %s --token %s --discovery-token-ca-cert-hash %s`, controlPlaneEndpoint, token, caCertHash)
	return RunCommandOnRemote(sshConfig, "bash", "-c", cmd)
}

// CheckKubeadmVersion 检查kubeadm版本
func CheckKubeadmVersion(sshConfig SSHConfig) (string, error) {
	cmd := `kubeadm version --short`
	return RunCommandOnRemote(sshConfig, "bash", "-c", cmd)
}

// PullKubernetesImages 拉取Kubernetes镜像
func PullKubernetesImages(sshConfig SSHConfig, version string) (string, error) {
	cmd := fmt.Sprintf(`kubeadm config images pull --kubernetes-version %s`, version)
	return RunCommandOnRemote(sshConfig, "bash", "-c", cmd)
}

// ResetCluster 重置集群，添加完整的清理步骤
func ResetCluster(sshConfig SSHConfig) (string, error) {
	cmd := `# 执行kubeadm reset
sudo kubeadm reset --force

# 清理CNI配置
sudo rm -rf /etc/cni/net.d

# 重置iptables规则
sudo iptables -F
sudo iptables -t nat -F
sudo iptables -t mangle -F
sudo iptables -X

# 重置ip6tables规则
sudo ip6tables -F
sudo ip6tables -t nat -F
sudo ip6tables -t mangle -F
sudo ip6tables -X

# 如果使用IPVS，重置IPVS表
if command -v ipvsadm &> /dev/null; then
    sudo ipvsadm --clear
fi

# 清理kubeconfig文件
sudo rm -rf ~/.kube
rm -rf $HOME/.kube
# 清理集群配置文件
sudo rm -f /etc/kubernetes/admin.conf
sudo rm -f /etc/kubernetes/kubelet.conf
sudo rm -f /etc/kubernetes/controller-manager.conf
sudo rm -f /etc/kubernetes/scheduler.conf
sudo rm -rf /etc/kubernetes/manifests

# 清理旧的etcd数据
sudo rm -rf /var/lib/etcd

# 清理旧的kubelet数据
sudo rm -rf /var/lib/kubelet

# 清理旧的容器数据
sudo systemctl stop containerd || true
sudo systemctl stop docker || true
sudo rm -rf /var/lib/containerd
sudo rm -rf /var/lib/docker
sudo rm -rf /run/containerd
sudo rm -rf /var/run/containerd
sudo rm -f /run/containerd/containerd.sock

# 重启服务以确保所有更改生效
sudo systemctl restart containerd || true
sudo systemctl restart docker || true`
	return RunCommandOnRemote(sshConfig, "bash", "-c", cmd)
}
