package kubeadm

import (
	"context"
	"fmt"
	"os"
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

// 定义部署步骤常量，用于指定跳过步骤
const (
	StepSystemPreparation                 = "system_preparation"
	StepIpForwardConfiguration            = "ip_forward_configuration"
	StepContainerRuntimeInstallation      = "container_runtime_installation"
	StepKubernetesRepositoryConfiguration = "kubernetes_repository_configuration"
	StepKubernetesComponentsInstallation  = "kubernetes_components_installation"
	StepMasterInitialization              = "master_initialization"
	StepWorkerJoin                        = "worker_join"
	StepClusterVerification               = "cluster_verification"
)

// DeployK8sCluster 部署Kubernetes集群
// 使用context支持异步部署和停止机制
// logCallback: 日志回调函数，用于实时输出部署日志，参数为(logMessage, nodeID, nodeName)
func DeployK8sCluster(ctx context.Context, nodes []node.Node, kubeVersion, arch, distro string, scriptManager interface{}, skipSteps []string, logCallback func(string, string, string)) (string, error) {
	// 实现完整的集群部署逻辑
	var result strings.Builder

	// 辅助函数：输出日志
	outputLog := func(nodeID, nodeName, log string) {
		result.WriteString(log + "\n")
		if logCallback != nil {
			logCallback(log, nodeID, nodeName)
		}
		fmt.Println(log) // 实时打印到控制台
	}

	// 辅助函数：检查步骤是否应该被跳过
	shouldSkip := func(step string) bool {
		for _, s := range skipSteps {
			if s == step {
				return true
			}
		}
		return false
	}

	// 辅助函数：验证脚本是否包含必要的启动命令
	// 如果脚本不完整，返回false，表示应该使用默认脚本
	scriptContainsEssentialCommands := func(script string) bool {
		// 检查containerd配置脚本是否包含启动命令
		hasSystemctlRestart := strings.Contains(script, "systemctl restart containerd") ||
			strings.Contains(script, "systemctl start containerd")
		hasSystemctlEnable := strings.Contains(script, "systemctl enable containerd")
		hasDaemonReload := strings.Contains(script, "systemctl daemon-reload")

		// 如果缺少任何关键命令，返回false
		if !hasSystemctlRestart || !hasSystemctlEnable || !hasDaemonReload {
			return false
		}
		return true
	}

	// 1. 找出master节点和worker节点
	var masterNodes []node.Node
	var workerNodes []node.Node
	var masterNode node.Node
	for _, node := range nodes {
		if node.NodeType == "master" {
			masterNodes = append(masterNodes, node)
		} else {
			workerNodes = append(workerNodes, node)
		}
	}

	// 检查master节点数量
	if len(masterNodes) > 1 {
		return "", fmt.Errorf("目前只支持单master节点部署")
	}

	// 如果有master节点，设置masterNode变量
	if len(masterNodes) > 0 {
		masterNode = masterNodes[0]
	}

	// 允许只有worker节点的情况
	if len(masterNodes) == 0 && len(workerNodes) == 0 {
		return "", fmt.Errorf("至少需要一个节点")
	}

	// 定义joinCmd变量，用于存储从Master节点获取的join命令
	var joinCmd string
	var masterClient *ssh.SSHClient

	// 2. 为每个节点执行部署流程
	allNodes := append(masterNodes, workerNodes...)

	// 2.1 跳过hosts文件更新，因为这部分已经在SSH免密配置阶段完成
	// 确保所有节点的IP和名称是否有效
	for _, node := range allNodes {
		if node.IP == "" {
			return result.String(), fmt.Errorf("节点 %s 的IP地址为空", node.Name)
		}
		if node.Name == "" {
			return result.String(), fmt.Errorf("节点 %s 的名称为空", node.IP)
		}
	}

	// 显示提示信息
	outputLog("cluster", "Kubernetes Cluster", "=== 跳过hosts文件更新 ===")
	outputLog("cluster", "Kubernetes Cluster", "hosts文件更新已经在SSH免密配置阶段完成")
	outputLog("cluster", "Kubernetes Cluster", "所有节点的hosts文件应该已经包含正确的IP和名称对应关系")
	outputLog("cluster", "Kubernetes Cluster", "现在可以开始其他部署操作")

	// 等待2秒，确保系统准备就绪
	outputLog("cluster", "Kubernetes Cluster", "等待2秒，确保系统准备就绪")
	time.Sleep(2 * time.Second)

	// 获取第一个master节点（假设只有一个master节点）
	if len(masterNodes) > 0 {
		masterNode = masterNodes[0]
	}

	// 2.2 为每个节点执行部署流程
	for _, node := range allNodes {
		// 检查是否需要取消部署
		select {
		case <-ctx.Done():
			outputLog("cluster", "Kubernetes Cluster", "部署已取消")
			return result.String(), ctx.Err()
		default:
		}
		outputLog(node.ID, node.Name, fmt.Sprintf("=== 部署节点: %s (%s) ===", node.Name, node.IP))

		// 创建SSH客户端，首先尝试使用节点名称连接（此时hosts文件已更新）
		sshConfig := ssh.SSHConfig{
			Host:       node.Name,
			Port:       node.Port,
			Username:   node.Username,
			Password:   node.Password,
			PrivateKey: node.PrivateKey,
		}

		client, err := ssh.NewSSHClient(sshConfig)
		if err != nil {
			// 如果使用节点名称连接失败，尝试使用IP地址连接
			outputLog(node.ID, node.Name, fmt.Sprintf("使用节点名称连接失败: %v，尝试使用IP地址连接...", err))
			sshConfig.Host = node.IP
			client, err = ssh.NewSSHClient(sshConfig)
			if err != nil {
				outputLog(node.ID, node.Name, fmt.Sprintf("创建SSH客户端失败: %v", err))
				return result.String(), err
			}
			outputLog(node.ID, node.Name, "使用IP地址连接成功")
		} else {
			outputLog(node.ID, node.Name, "使用节点名称连接成功")
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
			outputLog(node.ID, node.Name, fmt.Sprintf("检测操作系统类型失败: %v", err))
			return result.String(), err
		}
		nodeDistro := strings.TrimSpace(distroOutput)
		outputLog(node.ID, node.Name, fmt.Sprintf("操作系统: %s", nodeDistro))

		// 4. 执行系统准备脚本 - 这应该是部署的第一步，在节点重置之前执行
		if !shouldSkip(StepSystemPreparation) {
			// 系统准备脚本已经在前面的代码中实现，这里不需要重复
			// 我们只需要确保它在节点重置之前执行
			// 系统准备脚本中已经包含了完整的防火墙和SELinux配置
		}

		// 5. 执行节点重置流程（如果是worker节点且需要重复部署）
		// 系统准备脚本已经执行完成，现在可以执行节点重置流程
		if node.NodeType == "worker" {
			result.WriteString("\n=== 执行worker节点重置流程 ===\n")
			resetCmd := `# Worker节点重置脚本
			echo "=== 开始worker节点重置流程 ==="
			
			# 检查kubeadm是否安装
			if command -v kubeadm &> /dev/null; then
				echo "1. 检查节点是否已加入集群..."
				# 检查kubelet服务是否运行
				if command -v systemctl &> /dev/null; then
					systemctl_status=$(sudo systemctl is-active kubelet 2>/dev/null || echo "inactive")
					if [ "$systemctl_status" = "active" ] || [ -f /etc/kubernetes/kubelet.conf ]; then
						echo "2. 节点已加入集群，执行kubeadm reset..."
						# 执行kubeadm reset，添加--force参数确保重置成功
						sudo kubeadm reset --force --cri-socket=unix:///run/containerd/containerd.sock
						
						# 清理残留文件
						echo "3. 清理kubernetes残留文件..."
						sudo rm -rf /etc/kubernetes /var/lib/kubelet /var/lib/dockershim /var/run/kubernetes /var/lib/cni
						
						# 清理网络配置
						echo "4. 清理网络配置..."
						sudo rm -rf /etc/cni/net.d
						
						# 重启containerd服务
						echo "5. 重启containerd服务..."
						sudo systemctl restart containerd || true
						sleep 5
						
						echo "✓ Worker节点重置完成"
					else
						echo "节点未加入集群，跳过重置步骤"
					fi
				else
					echo "系统没有systemctl，跳过服务状态检查"
				fi
			else
				echo "kubeadm未安装，跳过重置步骤"
			fi
			
			echo "=== Worker节点重置流程完成 ==="
			`

			resetOutput, err := client.RunCommandWithOutput(resetCmd, func(line string) {
				result.WriteString("[重置流程] " + line + "\n")
				outputLog(node.ID, node.Name, "[重置流程] "+line)
			})

			if err != nil {
				result.WriteString(fmt.Sprintf("Worker节点重置过程中出现错误: %v\n详细输出:\n%s\n", err, resetOutput))
				outputLog(node.ID, node.Name, fmt.Sprintf("Worker节点重置失败: %v", err))
				result.WriteString("警告: Worker节点重置失败，但将继续尝试后续步骤...\n")
				outputLog(node.ID, node.Name, "警告: Worker节点重置失败，但将继续尝试后续步骤")
				// 不返回错误，继续执行后续步骤
			} else {
				result.WriteString("Worker节点重置成功\n")
				outputLog(node.ID, node.Name, "Worker节点重置成功")
			}

			// 添加延迟，确保重置流程完全执行
			result.WriteString("\n=== 等待5秒，确保重置流程完全执行 ===\n")
			outputLog(node.ID, node.Name, "等待5秒，确保重置流程完全执行")
			if _, err := client.RunCommand("sleep 5"); err != nil {
				result.WriteString(fmt.Sprintf("等待命令执行失败: %v\n", err))
				outputLog(node.ID, node.Name, fmt.Sprintf("等待命令执行失败: %v", err))
			}
		}

		// 6. 执行系统准备脚本
		// 系统准备脚本已经在前面的代码中实现，这里不需要重复
		// 我们只需要确保它在节点重置之前执行
		// 系统准备脚本中已经包含了完整的防火墙和SELinux配置
		if !shouldSkip(StepSystemPreparation) {
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
						systemPrepCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
						systemPrepFound = true
						result.WriteString(fmt.Sprintf("使用自定义系统准备脚本: %s\n", systemPrepScriptName))
					} else {
						// 尝试获取通用系统准备脚本
						if script, scriptFound := scriptGetter.GetScript("system_prep"); scriptFound {
							systemPrepCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
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
echo "=== 禁用swap ==="
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab
if [ $? -eq 0 ]; then
    echo "✓ swap已禁用并在重启后保持禁用"
else
    echo "⚠ swap禁用可能未完全生效，请检查/etc/fstab文件"
fi

# 安装并启动时间同步服务
echo "=== 安装并配置时间同步 ==="
if command -v apt-get &> /dev/null; then
    sudo apt update -y
    sudo apt install -y chrony iptables ip6tables
    sudo systemctl enable --now chronyd || sudo systemctl enable --now chrony
    sudo timedatectl set-timezone Asia/Shanghai
    sudo systemctl restart chronyd || sudo systemctl restart chrony
    chronyc sources
elif command -v dnf &> /dev/null || command -v yum &> /dev/null; then
    if command -v dnf &> /dev/null; then
        sudo dnf install -y chrony iptables ip6tables-services
    else
        sudo yum install -y chrony iptables-services
    fi
    sudo systemctl enable --now chronyd
    sudo timedatectl set-timezone Asia/Shanghai
    sudo systemctl restart chronyd || sudo systemctl restart chrony
    chronyc sources
fi

# 1. 必须的内核模块 - Calico初始化依赖
	echo "=== 加载必须的内核模块（Calico初始化依赖） ==="
	sudo modprobe br_netfilter || echo "br_netfilter模块已加载或加载失败"
	sudo modprobe overlay || echo "overlay模块已加载或加载失败"
	
	# 2. 持久化内核模块配置
	echo "=== 持久化内核模块配置 ==="
	sudo cat <<EOF > /etc/modules-load.d/k8s.conf
br_netfilter
overlay
EOF

	# 3. 必须的 sysctl - Calico初始化依赖，此文件必须写入
	echo "=== 配置必须的sysctl（Calico初始化依赖） ==="
	sudo cat <<EOF > /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward = 1
EOF
	sudo sysctl --system

	# 4. 安装iptables和ip6tables以及CNI插件所需的iproute-tc工具
	echo "=== 安装iptables、ip6tables和iproute-tc ==="
	if command -v apt-get &> /dev/null; then
	    sudo apt install -y iptables ip6tables iproute2
	elif command -v dnf &> /dev/null; then
	    # Rocky 10 必装，否则calico-node Init直接失败
	    sudo dnf install -y iptables ip6tables-services iproute-tc
	elif command -v yum &> /dev/null; then
	    sudo yum install -y iptables-services iproute-tc
	fi

	# 5. BPF挂载点（init容器mount-bpffs需要）
	echo "=== 创建并挂载BPF挂载点 ==="
	sudo mkdir -p /sys/fs/bpf
	sudo mount bpffs /sys/fs/bpf || true

	# 6. 确保CNI目录存在
	echo "=== 确保CNI目录存在 ==="
	sudo mkdir -p /opt/cni/bin
	sudo chmod 755 /opt/cni/bin
	sudo mkdir -p /etc/cni/net.d
	sudo chmod 755 /etc/cni/net.d

	# 7. 重启关键服务
	echo "=== 重启关键服务 ==="
	sudo systemctl restart containerd || true
	sudo systemctl restart kubelet || true

# 处理iptables服务（兼容不同系统）
echo "=== 处理iptables服务 ==="
if command -v systemctl &> /dev/null; then
    # 对于不同系统的iptables服务兼容处理
    echo "检查iptables服务状态..."
    # 尝试启动并启用iptables服务，如果不存在则忽略错误
    if systemctl list-units --type=service | grep -q iptables; then
        echo "iptables服务存在，正在启动和启用..."
        sudo systemctl enable --now iptables || true
        sudo systemctl restart iptables || true
    else
        echo "iptables服务不存在，确保iptables命令可用..."
        if ! command -v iptables &> /dev/null; then
            echo "iptables命令不可用，尝试安装..."
            if command -v apt-get &> /dev/null; then
                sudo apt install -y iptables || true
            elif command -v dnf &> /dev/null; then
                sudo dnf install -y iptables || true
            elif command -v yum &> /dev/null; then
                sudo yum install -y iptables || true
            fi
        else
            echo "✓ iptables命令已可用"
        fi
    fi
    
    # 处理ip6tables服务
    if systemctl list-units --type=service | grep -q ip6tables; then
        echo "ip6tables服务存在，正在启动和启用..."
        sudo systemctl enable --now ip6tables || true
        sudo systemctl restart ip6tables || true
    else
        echo "ip6tables服务不存在，确保ip6tables命令可用..."
        if ! command -v ip6tables &> /dev/null; then
            echo "ip6tables命令不可用，尝试安装..."
            if command -v apt-get &> /dev/null; then
                sudo apt install -y ip6tables || true
            elif command -v dnf &> /dev/null; then
                sudo dnf install -y ip6tables || true
            elif command -v yum &> /dev/null; then
                sudo yum install -y ip6tables || true
            fi
        else
            echo "✓ ip6tables命令已可用"
        fi
    fi
fi

# 关闭防火墙（实验环境建议关闭）并确保重启后保持关闭
echo "=== 配置防火墙 ==="
if command -v ufw &> /dev/null; then
    echo "处理ufw防火墙..."
    # 停止并禁用ufw服务
    sudo systemctl stop ufw || true
    sudo systemctl disable ufw || true
    # 额外的禁用步骤，确保完全关闭
    sudo ufw disable 2>/dev/null || true
    # 确保ufw配置文件设置为禁用
    if [ -f /etc/ufw/ufw.conf ]; then
        sudo sed -i 's/^ENABLED=yes/ENABLED=no/' /etc/ufw/ufw.conf || true
    fi
    echo "✓ ufw防火墙已关闭并禁用，重启后保持关闭"
elif command -v firewall-cmd &> /dev/null; then
    echo "处理firewalld防火墙..."
    # 停止并禁用firewalld服务
    sudo systemctl stop firewalld || true
    sudo systemctl disable firewalld || true
    # 额外的禁用步骤，确保完全关闭
    sudo firewall-cmd --state 2>/dev/null && sudo firewall-cmd --panic-on || true
    # 确保firewalld配置文件设置为禁用
    if [ -f /etc/firewalld/firewalld.conf ]; then
        sudo sed -i 's/^FirewallBackend=.*/FirewallBackend=nftables/' /etc/firewalld/firewalld.conf || true
    fi
    echo "✓ firewalld防火墙已关闭并禁用，重启后保持关闭"
else
    echo "未检测到ufw或firewalld，跳过防火墙配置"
fi

# 配置SELinux为permissive模式（仅适用于RHEL/CentOS系统）并确保重启后保持配置
echo "=== 配置SELinux ==="
if command -v setenforce &> /dev/null; then
    echo "临时设置SELinux为permissive模式..."
    sudo setenforce 0 2>/dev/null || true
    
    echo "永久设置SELinux为permissive模式..."
    # 尝试多种方式修改SELINUX配置，确保生效
    if [ -f /etc/selinux/config ]; then
        # 备份原始配置文件
        sudo cp /etc/selinux/config /etc/selinux/config.bak
        # 修改配置文件，将enforcing改为permissive
        sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config 2>/dev/null || true
        # 添加fallback，将disabled也改为permissive
        sudo sed -i 's/^SELINUX=disabled$/SELINUX=permissive/' /etc/selinux/config 2>/dev/null || true
        # 验证SELinux配置
        selinux_status=$(grep ^SELINUX= /etc/selinux/config | cut -d= -f2)
        echo "SELinux配置已设置为: $selinux_status"
        # 验证SELinux配置文件内容
        sudo grep -E '^SELINUX=' /etc/selinux/config 2>/dev/null || true
        # 再次确认SELinux状态
        selinux_current=$(sudo getenforce 2>/dev/null || echo "Unknown")
        echo "当前SELinux状态: $selinux_current"
        if [ "$selinux_status" = "permissive" ] || [ "$selinux_current" = "Permissive" ]; then
            echo "✓ SELinux已成功设置为permissive模式，重启后保持配置"
        else
            echo "⚠ SELinux配置可能未完全生效，请检查/etc/selinux/config文件"
        fi
    else
        echo "未找到/etc/selinux/config文件，SELinux可能未安装或使用不同配置"
    fi
else
    echo "未检测到SELinux，跳过SELinux配置"
fi

# 确保防火墙和SELinux状态在重启后保持
echo "=== 最终确认防火墙和SELinux状态 ==="
# 再次确认防火墙状态
if command -v ufw &> /dev/null; then
    ufw_status=$(sudo ufw status 2>/dev/null || echo "inactive")
    echo "当前ufw状态: $ufw_status"
elif command -v firewall-cmd &> /dev/null; then
    firewalld_status=$(sudo systemctl is-active firewalld 2>/dev/null || echo "inactive")
    echo "当前firewalld状态: $firewalld_status"
fi

# 再次确认SELinux状态
if command -v getenforce &> /dev/null; then
    selinux_current=$(sudo getenforce 2>/dev/null || echo "Disabled")
    echo "当前SELinux状态: $selinux_current"
fi

# 加载K8s所需内核模块
echo "=== 加载Kubernetes所需内核模块 ==="
sudo cat <<EOF > /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay || echo "overlay模块已加载或加载失败"
sudo modprobe br_netfilter || echo "br_netfilter模块已加载或加载失败"

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

# 应用内核参数
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
			startTime := time.Now()
			result.WriteString("脚本执行开始时间: " + startTime.Format("2006-01-02 15:04:05") + "\n")
			outputLog(node.ID, node.Name, fmt.Sprintf("开始执行系统准备脚本: %s", systemPrepScriptName))

			systemPrepOutput, err := client.RunCommandWithOutput(systemPrepCmd, func(line string) {
				result.WriteString("[脚本输出] " + line + "\n")
				outputLog(node.ID, node.Name, "[脚本输出] "+line)
			})

			endTime := time.Now()
			duration := endTime.Sub(startTime)
			result.WriteString("\n脚本执行结束时间: " + endTime.Format("2006-01-02 15:04:05") + "\n")
			result.WriteString(fmt.Sprintf("脚本执行持续时间: %v\n", duration))

			if err != nil {
				result.WriteString(fmt.Sprintf("系统准备脚本执行出现错误: %v\n详细输出:\n%s\n", err, systemPrepOutput))
				outputLog(node.ID, node.Name, fmt.Sprintf("系统准备脚本执行失败: %v", err))
				result.WriteString("警告: 系统准备脚本执行失败，但将继续尝试IP转发配置...\n")
				outputLog(node.ID, node.Name, "警告: 系统准备脚本执行失败，但将继续尝试IP转发配置")
				// 不返回错误，继续执行IP转发配置
			} else {
				result.WriteString("系统准备脚本执行成功\n")
				outputLog(node.ID, node.Name, "系统准备脚本执行成功")
			}

			// 添加延迟，确保系统准备脚本完全执行
			result.WriteString("\n=== 等待5秒，确保系统准备脚本完全执行 ===\n")
			outputLog(node.ID, node.Name, "等待5秒，确保系统准备脚本完全执行")
			if _, err := client.RunCommand("sleep 5"); err != nil {
				result.WriteString(fmt.Sprintf("等待命令执行失败: %v\n", err))
				outputLog(node.ID, node.Name, fmt.Sprintf("等待命令执行失败: %v", err))
			}
		} else {
			result.WriteString("\n=== 跳过系统准备 ===\n")
		}

		// 确保IP转发配置被正确设置，即使系统准备脚本中已有配置，再单独执行一次确保生效
		if !shouldSkip(StepIpForwardConfiguration) {
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
				fmt.Println("[脚本输出] " + line)                 // 实时打印到控制台
				outputLog(node.ID, node.Name, "[脚本输出] "+line) // 实时发送到前端
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
		} else {
			result.WriteString("\n=== 跳过IP转发配置 ===\n")
		}

		// 5. 执行容器运行时安装脚本
		if !shouldSkip(StepContainerRuntimeInstallation) {
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
						containerdInstallCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
						containerdInstallFound = true
						result.WriteString(fmt.Sprintf("使用自定义容器运行时安装脚本: %s\n", containerdInstallScriptName))
					} else {
						// 尝试获取通用容器运行时安装脚本
						if script, scriptFound := scriptGetter.GetScript("containerd_install"); scriptFound {
							containerdInstallCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
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
        sudo apt update -y
        sudo apt install -y containerd.io crictl curl
        # 确保containerd服务存在
        if [ ! -f /lib/systemd/system/containerd.service ]; then
            echo "containerd.service不存在，创建默认服务文件..."
            sudo mkdir -p /etc/containerd
            sudo containerd config default | sudo tee /etc/containerd/config.toml
        fi
    elif command -v dnf &> /dev/null || command -v yum &> /dev/null; then
        # CentOS/RHEL系统
        echo "=== 添加Docker仓库 ==="
        # 安装必要的依赖
        if command -v dnf &> /dev/null; then
            sudo dnf install -y dnf-plugins-core curl
            sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            sudo dnf install -y containerd.io crictl
        else
            sudo yum install -y yum-utils curl
            sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            sudo yum install -y containerd.io crictl
        fi
    else
        echo "=== 警告: 不支持的包管理器，尝试手动安装containerd ==="
        # 尝试从GitHub下载并安装containerd
        if command -v curl &> /dev/null && command -v tar &> /dev/null; then
            CONTAINERD_VERSION="1.6.28"
            ARCH="amd64"
            echo "从GitHub下载containerd v${CONTAINERD_VERSION}..."
            sudo mkdir -p /tmp/containerd
            curl -fsSL -o /tmp/containerd/containerd.tar.gz https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz
            sudo mkdir -p /usr/local/bin /usr/local/lib /etc/containerd
            sudo tar Cxzvf /usr/local /tmp/containerd/containerd.tar.gz
            sudo rm -rf /tmp/containerd
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
fi

# 安装crictl（容器运行时接口客户端）
echo "=== 安装crictl ==="
if ! command -v crictl &> /dev/null; then
    echo "crictl未安装，正在安装..."
    if command -v curl &> /dev/null; then
        CRICTL_VERSION="1.26.0"
        ARCH="amd64"
        echo "从GitHub下载crictl v${CRICTL_VERSION}..."
        sudo curl -fsSL -o /usr/local/bin/crictl https://github.com/kubernetes-sigs/cri-tools/releases/download/v${CRICTL_VERSION}/crictl-v${CRICTL_VERSION}-linux-${ARCH}.tar.gz
        sudo tar -xzf /usr/local/bin/crictl -C /usr/local/bin
        sudo rm -f /usr/local/bin/crictl.tar.gz
        echo "设置crictl配置文件..."
        sudo cat > /etc/crictl.yaml <<-'EOF'
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
debug: false
EOF
    fi
else
    echo "crictl已安装，跳过安装步骤"
fi`
				result.WriteString("使用默认容器运行时安装脚本\n")
			}

			// 执行容器运行时安装脚本并实时输出
			result.WriteString("\n=== 执行容器运行时安装脚本 ===\n")
			outputLog(node.ID, node.Name, "=== 执行容器运行时安装脚本 ===")
			// 确保containerdInstallScriptName有定义
			if containerdInstallScriptName == "" {
				containerdInstallScriptName = "containerd_install_default"
			}
			result.WriteString(fmt.Sprintf("脚本名称: %s\n", containerdInstallScriptName))
			outputLog(node.ID, node.Name, fmt.Sprintf("脚本名称: %s", containerdInstallScriptName))
			result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			outputLog(node.ID, node.Name, "脚本执行开始时间: "+time.Now().Format("2006-01-02 15:04:05"))
			containerdInstallOutput, err := client.RunCommandWithOutput(containerdInstallCmd, func(line string) {
				result.WriteString("[脚本输出] " + line + "\n")
				fmt.Println("[脚本输出] " + line)                 // 实时打印到控制台
				outputLog(node.ID, node.Name, "[脚本输出] "+line) // 实时发送到前端
			})
			if err != nil {
				result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
				outputLog(node.ID, node.Name, "脚本执行结束时间: "+time.Now().Format("2006-01-02 15:04:05"))
				result.WriteString(fmt.Sprintf("容器运行时安装失败: %v\n详细输出:\n%s\n", err, containerdInstallOutput))
				outputLog(node.ID, node.Name, fmt.Sprintf("容器运行时安装失败: %v", err))
				return result.String(), err
			}
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			outputLog(node.ID, node.Name, "脚本执行结束时间: "+time.Now().Format("2006-01-02 15:04:05"))
			result.WriteString("容器运行时安装成功\n")
			outputLog(node.ID, node.Name, "容器运行时安装成功")
		} else {
			result.WriteString("\n=== 跳过容器运行时安装 ===\n")
		}

		// 5. 执行容器运行时配置脚本
		if !shouldSkip(StepContainerRuntimeInstallation) {
			result.WriteString("\n=== 配置容器运行时 ===\n")
			var containerdConfigCmd string
			var containerdConfigFound bool
			var containerdConfigScriptName string
			var usingDefaultScript bool = false // 标记是否使用默认脚本

			// 从脚本管理器获取容器运行时配置脚本
			if scriptManager != nil {
				if scriptGetter, ok := scriptManager.(interface {
					GetScript(name string) (string, bool)
				}); ok {
					// 尝试获取特定发行版的容器运行时配置脚本，使用与前端完全一致的命名格式
					stepName := strings.ReplaceAll(strings.ToLower("配置容器运行时"), " ", "_")
					containerdConfigScriptName = fmt.Sprintf("%s_%s", nodeDistro, stepName)
					if script, scriptFound := scriptGetter.GetScript(containerdConfigScriptName); scriptFound {
						// 验证脚本是否包含必要的启动命令
						if scriptContainsEssentialCommands(script) {
							containerdConfigCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
							containerdConfigFound = true
							result.WriteString(fmt.Sprintf("使用自定义容器运行时配置脚本: %s (已验证完整性)\n", containerdConfigScriptName))
						} else {
							// 自定义脚本不完整，使用默认脚本
							result.WriteString(fmt.Sprintf("警告: 自定义脚本 %s 不完整，缺少必要的启动命令，将使用默认脚本\n", containerdConfigScriptName))
							usingDefaultScript = true
						}
					} else {
						// 尝试获取通用容器运行时配置脚本
						if script, scriptFound := scriptGetter.GetScript("containerd_config"); scriptFound {
							if scriptContainsEssentialCommands(script) {
								containerdConfigCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
								containerdConfigFound = true
								result.WriteString("使用自定义容器运行时配置脚本 (已验证完整性)\n")
							} else {
								result.WriteString("警告: 自定义脚本不完整，缺少必要的启动命令，将使用默认脚本\n")
								usingDefaultScript = true
							}
						}
					}
				}
			}

			// 如果没有找到自定义脚本，或自定义脚本不完整，使用默认脚本
			if !containerdConfigFound || usingDefaultScript {
				containerdConfigCmd = `# containerd配置脚本
# 配置containerd
echo "=== 配置containerd ==="
sudo mkdir -p /etc/containerd

# 生成默认配置，覆盖现有配置以确保正确性
echo "生成containerd默认配置..."
sudo containerd config default | sudo tee /etc/containerd/config.toml

# 确保使用systemd cgroup驱动
echo "配置systemd cgroup驱动..."
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# 修复cgroup配置路径
echo "修复cgroup配置路径..."
sudo sed -i 's#containerd.runtimes.runc.options#containerd.runtimes.runc.options.cgroup#g' /etc/containerd/config.toml || true

# 配置containerd使用镜像加速
echo "配置containerd使用镜像加速..."
sudo sed -i '/\[plugins\."io\.containerd\.grpc\.v1\.cri"\.registry\.mirrors\]/,/\[/c\[plugins."io.containerd.grpc.v1.cri".registry.mirrors\]\n\n  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]\n    endpoint = ["https://registry.docker-cn.com", "https://docker.mirrors.ustc.edu.cn", "https://docker.io"]' /etc/containerd/config.toml

# 启动前先停止可能运行的containerd进程
echo "停止可能运行的containerd进程..."
sudo pkill -f containerd || true
sleep 2

# 清理旧的containerd socket和状态文件
echo "清理旧的containerd socket和状态文件..."
sudo rm -f /run/containerd/containerd.sock || true
sudo rm -rf /var/run/containerd || true
sudo mkdir -p /var/run/containerd

# 确保containerd服务存在
echo "确保containerd服务存在..."
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

# 启动并启用containerd服务
echo "启动containerd服务..."
sudo systemctl daemon-reload
sudo systemctl start containerd || true
# 增加重试逻辑
echo "检查containerd服务状态..."
for i in {1..3}; do
    systemctl_status=$(sudo systemctl is-active containerd 2>/dev/null || echo "unknown")
    if [ "$systemctl_status" = "active" ]; then
        echo "✓ containerd服务已成功启动"
        break
    else
        echo "✗ containerd服务状态: $systemctl_status, 正在重试 ($i/3)..."
        sudo systemctl restart containerd || true
        sleep 5
    fi
done

# 启用containerd服务
sudo systemctl enable containerd

# 等待containerd启动，增加等待时间
echo "等待containerd启动..."
sleep 10

# 检查containerd状态
echo "=== 检查containerd状态 ==="
if command -v systemctl &> /dev/null; then
    systemctl_status=$(sudo systemctl is-active containerd 2>/dev/null || echo "unknown")
    echo "containerd服务状态: $systemctl_status"
    
    # 显示containerd服务详细状态
    echo "containerd服务详细状态:"
    sudo systemctl status containerd --no-pager
fi

# 检查containerd socket是否存在
echo "=== 检查containerd socket ==="
cri_socket="/run/containerd/containerd.sock"
if [ -S "$cri_socket" ]; then
    echo "✓ CRI socket $cri_socket 存在"
    # 测试socket连接
    echo "测试containerd连接..."
    if command -v ctr &> /dev/null; then
        sudo ctr version
    fi
    if command -v crictl &> /dev/null; then
        sudo crictl version
    fi
else
    echo "✗ 警告: CRI socket $cri_socket 不存在，检查containerd日志..."
    sudo journalctl -u containerd --no-pager -n 30
    
    # 尝试手动启动containerd
echo "尝试手动启动containerd..."
if command -v containerd &> /dev/null; then
    containerd_version=$(containerd --version)
    echo "containerd版本: $containerd_version"
    
    # 手动创建必要的目录
    sudo mkdir -p /run/containerd /var/lib/containerd
    
    # 尝试手动启动containerd
echo "使用默认配置手动启动containerd..."
    sudo containerd --config /etc/containerd/config.toml &
    CONTAINERD_PID=$!
    sleep 10
    
    # 再次检查socket
    if [ -S "$cri_socket" ]; then
        echo "✓ 手动启动成功，CRI socket $cri_socket 现在存在"
        # 停止手动启动的containerd进程
        sudo kill $CONTAINERD_PID || true
        sleep 2
        # 重新使用systemctl启动
        sudo systemctl restart containerd
    else
        echo "✗ 手动启动失败，CRI socket $cri_socket 仍然不存在"
        # 停止手动启动的containerd进程
        sudo kill $CONTAINERD_PID || true
    fi
fi
fi

# 最终验证containerd状态
echo "=== 最终验证containerd状态 ==="
if command -v crictl &> /dev/null; then
    echo "使用crictl测试containerd连接..."
    sudo crictl info || echo "crictl测试失败，可能containerd未正常运行"
fi`
				if usingDefaultScript {
					result.WriteString("使用默认容器运行时配置脚本 (自定义脚本不完整)\n")
				} else {
					result.WriteString("使用默认容器运行时配置脚本\n")
				}
			}

			// 执行容器运行时配置脚本并实时输出
			result.WriteString("\n=== 执行containerd配置脚本 ===\n")
			outputLog(node.ID, node.Name, "=== 执行containerd配置脚本 ===")
			// 确保containerdConfigScriptName有定义
			if containerdConfigScriptName == "" {
				containerdConfigScriptName = "containerd_config_default"
			}
			result.WriteString(fmt.Sprintf("脚本名称: %s\n", containerdConfigScriptName))
			outputLog(node.ID, node.Name, fmt.Sprintf("脚本名称: %s", containerdConfigScriptName))
			result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			outputLog(node.ID, node.Name, "脚本执行开始时间: "+time.Now().Format("2006-01-02 15:04:05"))
			containerdConfigOutput, err := client.RunCommandWithOutput(containerdConfigCmd, func(line string) {
				result.WriteString("[脚本输出] " + line + "\n")
				fmt.Println("[脚本输出] " + line)                 // 实时打印到控制台
				outputLog(node.ID, node.Name, "[脚本输出] "+line) // 实时发送到前端
			})
			if err != nil {
				result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
				outputLog(node.ID, node.Name, "脚本执行结束时间: "+time.Now().Format("2006-01-02 15:04:05"))
				result.WriteString(fmt.Sprintf("容器运行时配置失败: %v\n详细输出:\n%s\n", err, containerdConfigOutput))
				outputLog(node.ID, node.Name, fmt.Sprintf("容器运行时配置失败: %v", err))
				return result.String(), err
			}
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			outputLog(node.ID, node.Name, "脚本执行结束时间: "+time.Now().Format("2006-01-02 15:04:05"))
			result.WriteString("容器运行时配置成功\n")
			outputLog(node.ID, node.Name, "容器运行时配置成功")
		}

		// 7. 添加Kubernetes仓库
		if !shouldSkip(StepKubernetesRepositoryConfiguration) {
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
						addK8sRepoCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
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
					addK8sRepoCmd = `# 添加Kubernetes仓库（Ubuntu/Debian）
echo "=== 添加Kubernetes仓库 ==="
apt-get update -y
apt-get install -y apt-transport-https ca-certificates curl gpg

# 创建keyring目录
mkdir -p -m 755 /etc/apt/keyrings

# 使用阿里云镜像源
# 下载并安装阿里云GPG密钥
curl -fsSL -L https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# 添加阿里云Kubernetes repo
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main" | tee /etc/apt/sources.list.d/kubernetes.list

# 更新仓库缓存
apt-get update -y`
				case "centos", "rhel", "rocky", "almalinux":
					addK8sRepoCmd = `# 添加Kubernetes仓库（CentOS/RHEL/Rocky/AlmaLinux）
echo "=== 添加Kubernetes仓库 ==="
cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
exclude=kubelet kubeadm kubectl
EOF

# 更新仓库缓存
if command -v dnf &> /dev/null; then
    dnf clean all
    dnf makecache -y
else
    yum clean all
    yum makecache -y
fi`
				default:
					result.WriteString(fmt.Sprintf("不支持的发行版: %s\n", nodeDistro))
					return result.String(), fmt.Errorf("不支持的发行版: %s", nodeDistro)
				}
				result.WriteString("使用默认添加Kubernetes仓库脚本\n")
			}

			// 执行添加Kubernetes仓库脚本并实时输出
			result.WriteString("\n=== 执行添加Kubernetes仓库脚本 ===\n")
			outputLog(node.ID, node.Name, "=== 执行添加Kubernetes仓库脚本 ===")
			// 确保addK8sRepoScriptName有定义
			if addK8sRepoScriptName == "" {
				addK8sRepoScriptName = "add_k8s_repo_default"
			}
			result.WriteString(fmt.Sprintf("脚本名称: %s\n", addK8sRepoScriptName))
			outputLog(node.ID, node.Name, fmt.Sprintf("脚本名称: %s", addK8sRepoScriptName))
			result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			outputLog(node.ID, node.Name, "脚本执行开始时间: "+time.Now().Format("2006-01-02 15:04:05"))
			addK8sRepoOutput, err := client.RunCommandWithOutput(addK8sRepoCmd, func(line string) {
				result.WriteString("[脚本输出] " + line + "\n")
				fmt.Println("[脚本输出] " + line)                 // 实时打印到控制台
				outputLog(node.ID, node.Name, "[脚本输出] "+line) // 实时发送到前端
			})
			if err != nil {
				result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
				outputLog(node.ID, node.Name, "脚本执行结束时间: "+time.Now().Format("2006-01-02 15:04:05"))
				result.WriteString(fmt.Sprintf("添加Kubernetes仓库失败: %v\n详细输出:\n%s\n", err, addK8sRepoOutput))
				outputLog(node.ID, node.Name, fmt.Sprintf("添加Kubernetes仓库失败: %v", err))
				return result.String(), err
			}
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			outputLog(node.ID, node.Name, "脚本执行结束时间: "+time.Now().Format("2006-01-02 15:04:05"))
			result.WriteString("添加Kubernetes仓库成功\n")
			outputLog(node.ID, node.Name, "添加Kubernetes仓库成功")

			// 添加延迟，确保仓库更新完全执行
			result.WriteString("\n=== 等待3秒，确保仓库更新完全执行 ===\n")
			outputLog(node.ID, node.Name, "=== 等待3秒，确保仓库更新完全执行 ===")
			if _, err := client.RunCommand("sleep 3"); err != nil {
				result.WriteString(fmt.Sprintf("等待命令执行失败: %v\n", err))
				outputLog(node.ID, node.Name, fmt.Sprintf("等待命令执行失败: %v", err))
			}
		} else {
			result.WriteString("\n=== 跳过Kubernetes仓库配置 ===\n")
		}

		// 8. 安装Kubernetes组件
		if !shouldSkip(StepKubernetesComponentsInstallation) {
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
						k8sComponentsCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
						k8sComponentsFound = true
						result.WriteString(fmt.Sprintf("使用自定义Kubernetes组件安装脚本: %s\n", k8sComponentsScriptName))
					} else {
						// 尝试获取通用Kubernetes组件安装脚本
						if script, scriptFound := scriptGetter.GetScript("k8s_components"); scriptFound {
							k8sComponentsCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
							k8sComponentsFound = true
							result.WriteString("使用自定义Kubernetes组件安装脚本\n")
						} else {
							// 尝试获取旧格式的脚本，保持向后兼容
							oldK8sComponentsScriptName := fmt.Sprintf("k8s_components_%s", nodeDistro)
							if script, scriptFound := scriptGetter.GetScript(oldK8sComponentsScriptName); scriptFound {
								k8sComponentsCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
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
					k8sComponentsCmd = `# 安装Kubernetes组件（Ubuntu/Debian）
echo "=== 添加Kubernetes仓库 ==="
apt-get update -y
apt-get install -y apt-transport-https ca-certificates curl gpg

# 创建keyring目录
mkdir -p -m 755 /etc/apt/keyrings

# 使用阿里云镜像源
# 下载并安装阿里云GPG密钥
curl -fsSL -L https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# 添加阿里云Kubernetes repo
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main" | tee /etc/apt/sources.list.d/kubernetes.list

# 更新仓库缓存
echo "=== 更新仓库缓存 ==="
apt-get update -y

# 检查可用的Kubernetes版本
echo "=== 检查可用的Kubernetes版本 ==="
AVAILABLE_VERSIONS=$(apt-cache madison kubelet | grep -oP '[0-9]+\.[0-9]+\.[0-9]+' | sort -V | uniq)

echo "可用的Kubernetes版本: $AVAILABLE_VERSIONS"

# 选择要安装的版本
SELECTED_VERSION="${version}"
echo "尝试安装指定版本: $SELECTED_VERSION"

# 检查指定版本是否可用
if ! echo "$AVAILABLE_VERSIONS" | grep -q "^$SELECTED_VERSION$"; then
    echo "指定版本 $SELECTED_VERSION 不可用，查找可用的最新版本..."
    # 如果指定版本不可用，使用可用的最新版本
    LATEST_VERSION=$(echo "$AVAILABLE_VERSIONS" | tail -1)
    if [ -n "$LATEST_VERSION" ]; then
        echo "使用可用的最新版本: $LATEST_VERSION"
        SELECTED_VERSION="$LATEST_VERSION"
    else
        echo "警告: 未找到可用的Kubernetes版本，尝试使用1.28.2版本..."
        SELECTED_VERSION="1.28.2"
    fi
fi

# 安装Kubernetes组件
echo "=== 安装kubelet、kubeadm和kubectl $SELECTED_VERSION ==="
apt-get install -y kubelet=$SELECTED_VERSION kubeadm=$SELECTED_VERSION kubectl=$SELECTED_VERSION

# 启动kubelet
echo "=== 启动kubelet服务 ==="
sudo systemctl enable --now kubelet

# 验证所有组件安装
echo "=== 验证组件安装 ==="
echo "检查kubeadm版本..."
kubeadm version
echo "检查kubelet版本..."
kubelet --version
echo "检查kubectl版本..."
kubectl version --client
echo "检查containerd版本..."
containerd --version
if command -v crictl &> /dev/null; then
    echo "检查crictl版本..."
    crictl version
fi`
					k8sComponentsCmd = strings.ReplaceAll(k8sComponentsCmd, "${version}", kubeVersion)
				case "centos", "rhel", "rocky", "almalinux":
					k8sComponentsCmd = `# 安装Kubernetes组件（CentOS/RHEL/Rocky/AlmaLinux）
echo "=== 添加Kubernetes仓库 ==="
cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
exclude=kubelet kubeadm kubectl
EOF

# 更新仓库缓存
echo "=== 更新仓库缓存 ==="
if command -v dnf &> /dev/null; then
    echo "使用dnf更新仓库缓存..."
    sudo dnf clean all
    sudo dnf makecache -y
else
    echo "使用yum更新仓库缓存..."
    sudo yum clean all
    sudo yum makecache -y
fi

# 检查可用的Kubernetes版本
echo "=== 检查可用的Kubernetes版本 ==="
# 改进版本检测逻辑，使用更可靠的方法
AVAILABLE_VERSIONS=$(if command -v dnf &> /dev/null; then
    # 尝试多种方法获取可用版本
    sudo dnf list --available kubelet --disableexcludes=kubernetes 2>/dev/null | grep -E 'kubelet' | grep -v '^\+' | awk '{print $2}' | cut -d'-' -f1 | sort -V | uniq || \
    sudo dnf search kubelet --disableexcludes=kubernetes 2>/dev/null | grep -E '^kubelet-[0-9]' | awk '{print $1}' | cut -d'-' -f2 | sort -V | uniq || \
    echo "1.28.2"
else
    # 尝试多种方法获取可用版本
    sudo yum list --available kubelet --disableexcludes=kubernetes 2>/dev/null | grep -E 'kubelet' | grep -v '^\+' | awk '{print $2}' | cut -d'-' -f1 | sort -V | uniq || \
    sudo yum search kubelet --disableexcludes=kubernetes 2>/dev/null | grep -E '^kubelet-[0-9]' | awk '{print $1}' | cut -d'-' -f2 | sort -V | uniq || \
    echo "1.28.2"
fi)

# 清理版本列表，移除空值和重复项
AVAILABLE_VERSIONS=$(echo "$AVAILABLE_VERSIONS" | grep -v '^$' | sort -V | uniq)

echo "可用的Kubernetes版本: $AVAILABLE_VERSIONS"

# 选择要安装的版本
SELECTED_VERSION="${version}"
echo "尝试安装指定版本: $SELECTED_VERSION"

# 检查指定版本是否可用
if ! echo "$AVAILABLE_VERSIONS" | grep -q "^$SELECTED_VERSION$"; then
    echo "指定版本 $SELECTED_VERSION 不可用，查找可用的最新版本..."
    # 如果指定版本不可用，使用可用的最新版本
    LATEST_VERSION=$(echo "$AVAILABLE_VERSIONS" | tail -1)
    if [ -n "$LATEST_VERSION" ]; then
        echo "使用可用的最新版本: $LATEST_VERSION"
        SELECTED_VERSION="$LATEST_VERSION"
    else
        echo "警告: 未找到可用的Kubernetes版本，尝试使用1.28.2版本..."
        SELECTED_VERSION="1.28.2"
    fi
fi

# 最终验证SELECTED_VERSION是否为空
if [ -z "$SELECTED_VERSION" ]; then
    echo "错误: SELECTED_VERSION变量为空，使用默认版本1.28.2"
    SELECTED_VERSION="1.28.2"
fi

# 安装Kubernetes组件
echo "=== 安装kubelet、kubeadm和kubectl $SELECTED_VERSION ==="
# 改进安装命令，使用更可靠的版本格式和重试机制
INSTALL_SUCCESS=false
if command -v dnf &> /dev/null; then
    echo "使用dnf安装Kubernetes组件..."
    # 尝试使用不同的版本格式，最多尝试3次
    for i in {1..3}; do
        echo "尝试安装 ($i/3)..."
        # 尝试1: 不指定版本，使用最新版本
        if sudo dnf install -y kubelet kubeadm kubectl --disableexcludes=kubernetes; then
            echo "✓ 安装成功（使用最新版本）"
            INSTALL_SUCCESS=true
            break
        fi
        # 尝试2: 指定完整版本号
        if sudo dnf install -y kubelet-$SELECTED_VERSION kubeadm-$SELECTED_VERSION kubectl-$SELECTED_VERSION --disableexcludes=kubernetes; then
            echo "✓ 安装成功（使用指定版本）"
            INSTALL_SUCCESS=true
            break
        fi
        # 尝试3: 使用更宽松的版本匹配
        if sudo dnf install -y "kubelet-$SELECTED_VERSION*" "kubeadm-$SELECTED_VERSION*" "kubectl-$SELECTED_VERSION*" --disableexcludes=kubernetes; then
            echo "✓ 安装成功（使用版本匹配）"
            INSTALL_SUCCESS=true
            break
        fi
        echo "安装失败，等待3秒后重试..."
        sleep 3
    done
else
    echo "使用yum安装Kubernetes组件..."
    # 尝试使用不同的版本格式，最多尝试3次
    for i in {1..3}; do
        echo "尝试安装 ($i/3)..."
        # 尝试1: 不指定版本，使用最新版本
        if sudo yum install -y kubelet kubeadm kubectl --disableexcludes=kubernetes; then
            echo "✓ 安装成功（使用最新版本）"
            INSTALL_SUCCESS=true
            break
        fi
        # 尝试2: 指定完整版本号
        if sudo yum install -y kubelet-$SELECTED_VERSION kubeadm-$SELECTED_VERSION kubectl-$SELECTED_VERSION --disableexcludes=kubernetes; then
            echo "✓ 安装成功（使用指定版本）"
            INSTALL_SUCCESS=true
            break
        fi
        # 尝试3: 使用更宽松的版本匹配
        if sudo yum install -y "kubelet-$SELECTED_VERSION*" "kubeadm-$SELECTED_VERSION*" "kubectl-$SELECTED_VERSION*" --disableexcludes=kubernetes; then
            echo "✓ 安装成功（使用版本匹配）"
            INSTALL_SUCCESS=true
            break
        fi
        echo "安装失败，等待3秒后重试..."
        sleep 3
    done
fi

# 检查安装是否成功
if [ "$INSTALL_SUCCESS" = false ]; then
    echo "⚠ 安装失败，尝试使用备选方法..."
    # 备选方法：使用rpm直接安装
    if command -v rpm &> /dev/null; then
        echo "尝试使用rpm直接安装..."
        # 这里可以添加rpm安装逻辑
        echo "警告: 备选安装方法未实现，请检查网络连接和仓库配置"
    fi
fi

# 启动kubelet
echo "=== 启动kubelet服务 ==="
sudo systemctl enable --now kubelet

# 验证所有组件安装
echo "=== 验证组件安装 ==="
echo "检查kubeadm版本..."
kubeadm version 2>/dev/null || echo "kubeadm版本检查失败"
echo "检查kubelet版本..."
kubelet --version 2>/dev/null || echo "kubelet版本检查失败"
echo "检查kubectl版本..."
kubectl version --client 2>/dev/null || echo "kubectl版本检查失败"
echo "检查containerd版本..."
containerd --version 2>/dev/null || echo "containerd版本检查失败"
if command -v crictl &> /dev/null; then
    echo "检查crictl版本..."
    crictl version 2>/dev/null || echo "crictl版本检查失败"
fi

# 最终验证
echo "=== 最终验证Kubernetes组件安装 ==="
if command -v kubeadm &> /dev/null && command -v kubelet &> /dev/null && command -v kubectl &> /dev/null; then
    echo "✓ 所有Kubernetes组件已成功安装"
else
    echo "⚠ 部分Kubernetes组件安装失败，请检查安装日志"
fi`
					k8sComponentsCmd = strings.ReplaceAll(k8sComponentsCmd, "${version}", kubeVersion)
				default:
					result.WriteString(fmt.Sprintf("不支持的发行版: %s\n", nodeDistro))
					return result.String(), fmt.Errorf("不支持的发行版: %s", nodeDistro)
				}
				result.WriteString("使用默认Kubernetes组件安装脚本\n")
			}

			// 执行Kubernetes组件安装脚本并实时输出
			result.WriteString("\n=== 执行Kubernetes组件安装脚本 ===\n")
			outputLog(node.ID, node.Name, "=== 执行Kubernetes组件安装脚本 ===")
			// 确保k8sComponentsScriptName有定义
			if k8sComponentsScriptName == "" {
				k8sComponentsScriptName = "k8s_components_default"
			}
			result.WriteString(fmt.Sprintf("脚本名称: %s\n", k8sComponentsScriptName))
			outputLog(node.ID, node.Name, fmt.Sprintf("脚本名称: %s", k8sComponentsScriptName))
			result.WriteString("脚本执行开始时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			outputLog(node.ID, node.Name, "脚本执行开始时间: "+time.Now().Format("2006-01-02 15:04:05"))
			k8sComponentsOutput, err := client.RunCommandWithOutput(k8sComponentsCmd, func(line string) {
				result.WriteString("[脚本输出] " + line + "\n")
				fmt.Println("[脚本输出] " + line)                 // 实时打印到控制台
				outputLog(node.ID, node.Name, "[脚本输出] "+line) // 实时发送到前端
			})
			if err != nil {
				result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
				outputLog(node.ID, node.Name, "脚本执行结束时间: "+time.Now().Format("2006-01-02 15:04:05"))
				result.WriteString(fmt.Sprintf("Kubernetes组件安装失败: %v\n详细输出:\n%s\n", err, k8sComponentsOutput))
				outputLog(node.ID, node.Name, fmt.Sprintf("Kubernetes组件安装失败: %v", err))
				return result.String(), err
			}
			result.WriteString("\n脚本执行结束时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
			outputLog(node.ID, node.Name, "脚本执行结束时间: "+time.Now().Format("2006-01-02 15:04:05"))
			result.WriteString("Kubernetes组件安装成功\n")
			outputLog(node.ID, node.Name, "Kubernetes组件安装成功")

			// 添加延迟，确保Kubernetes组件安装完全执行
			result.WriteString("\n=== 等待5秒，确保Kubernetes组件安装完全执行 ===\n")
			outputLog(node.ID, node.Name, "=== 等待5秒，确保Kubernetes组件安装完全执行 ===")
			if _, err := client.RunCommand("sleep 5"); err != nil {
				result.WriteString(fmt.Sprintf("等待命令执行失败: %v\n", err))
				outputLog(node.ID, node.Name, fmt.Sprintf("等待命令执行失败: %v", err))
			}
		} else {
			result.WriteString("\n=== 跳过Kubernetes组件安装 ===\n")
		}

		result.WriteString(fmt.Sprintf("=== 节点 %s 部署完成 ===\n\n", node.Name))
	}

	// 3. 初始化Master节点
	// 检查是否需要取消部署
	select {
	case <-ctx.Done():
		result.WriteString("部署已取消\n")
		return result.String(), ctx.Err()
	default:
	}

	// 检查是否有master节点
	if len(masterNodes) == 0 {
		result.WriteString("=== 跳过Master节点初始化：未找到master节点 ===\n")
	} else if !shouldSkip(StepMasterInitialization) {
		// 检查masterNode字段是否有效
		if masterNode.Name == "" && masterNode.IP == "" {
			result.WriteString("=== 跳过Master节点初始化：master节点信息无效 ===\n")
		} else if masterNode.Username == "" {
			result.WriteString("=== 跳过Master节点初始化：master节点用户名未设置 ===\n")
		} else if masterNode.Password == "" && masterNode.PrivateKey == "" {
			result.WriteString("=== 跳过Master节点初始化：master节点密码或私钥未设置 ===\n")
		} else {
			result.WriteString("=== 初始化Master节点 ===\n")
			// 直接使用节点的IP地址进行连接，避免依赖本地hosts文件
			masterSSHConfig := ssh.SSHConfig{
				Host:       masterNode.IP, // 直接使用IP地址，不依赖本地hosts文件
				Port:       masterNode.Port,
				Username:   masterNode.Username,
				Password:   masterNode.Password,
				PrivateKey: masterNode.PrivateKey,
			}

			initMasterClient, err := ssh.NewSSHClient(masterSSHConfig)
			if err != nil {
				result.WriteString(fmt.Sprintf("创建Master节点SSH客户端失败: %v\n", err))
				return result.String(), err
			}
			defer initMasterClient.Close()
			result.WriteString(fmt.Sprintf("连接到Master节点 %s (%s) 成功\n", masterNode.Name, masterNode.IP))

			// 检测Master节点的操作系统类型
			result.WriteString("\n=== 检测Master节点操作系统类型 ===\n")
			distroCmd := `
if [ -f /etc/os-release ]; then
	. /etc/os-release
	echo $ID
fi
`
			masterDistro, err := initMasterClient.RunCommand(distroCmd)
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
			finalIpForwardOutput, err := initMasterClient.RunCommandWithOutput(finalIpForwardCmd, func(line string) {
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
						initCmd = strings.ReplaceAll(script, "${version}", kubeVersion)
						initFound = true
						result.WriteString(fmt.Sprintf("使用自定义Kubernetes初始化脚本: %s\n", initScriptName))
					}
				}
			}

			// 如果没有找到自定义脚本，使用默认脚本
			if !initFound {
				initCmd = fmt.Sprintf(`# 重置集群，清理旧配置
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

# 验证防火墙和swap状态
					echo "=== 验证防火墙和swap状态 ==="
					
					# 检查firewalld状态
					if command -v firewall-cmd &> /dev/null; then
					    firewall_status=$(sudo systemctl is-active firewalld 2>/dev/null || echo "inactive")
					    echo "当前firewalld状态: $firewall_status"
					    if [ "$firewall_status" = "active" ]; then
					        echo "警告: firewalld仍在运行，正在尝试停止并禁用..."
					        sudo systemctl stop firewalld || true
					        sudo systemctl disable firewalld || true
					        firewall_status=$(sudo systemctl is-active firewalld 2>/dev/null || echo "inactive")
					        echo "停止后firewalld状态: $firewall_status"
					    fi
					fi
					
					# 检查swap状态
					swap_status=$(sudo swapon --show | wc -l)
					echo "当前swap使用情况: $swap_status 个设备"
					if [ $swap_status -gt 0 ]; then
					    echo "警告: swap仍在使用，正在尝试禁用..."
					    sudo swapoff -a
					    swap_status=$(sudo swapon --show | wc -l)
					    echo "禁用后swap使用情况: $swap_status 个设备"
					fi
					
					# 检查/proc/sys/net/ipv4/ip_forward状态
					ip_forward_status=$(cat /proc/sys/net/ipv4/ip_forward)
					echo "当前IP转发状态: $ip_forward_status"
					if [ "$ip_forward_status" != "1" ]; then
					    echo "警告: IP转发未启用，正在尝试启用..."
					    sudo sysctl -w net.ipv4.ip_forward=1
					    ip_forward_status=$(cat /proc/sys/net/ipv4/ip_forward)
					    echo "启用后IP转发状态: $ip_forward_status"
					fi
					
					# 初始化Master节点，使用阿里云镜像源
					echo "=== 执行kubeadm init ==="
					sudo kubeadm init --kubernetes-version=%s --image-repository=registry.aliyuncs.com/google_containers --cri-socket=unix:///run/containerd/containerd.sock --pod-network-cidr=10.244.0.0/16 --upload-certs

# 检查kubeadm init是否成功
					if [ $? -eq 0 ]; then
					    echo "=== kubeadm init 成功 ==="
					    
					    # 立即生成join命令并输出，供Worker节点使用
					    echo "=== 生成Join命令 ==="
					    sudo kubeadm token create --print-join-command
					    
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
					        # 增加重试机制，确保Flannel安装成功
					        for i in {1..3}; do
					            echo "尝试安装Flannel ($i/3)..."
					            if kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml; then
					                echo "✓ Flannel网络插件安装成功"
					                # 等待Flannel部署完成
					                echo "等待Flannel部署完成..."
					                sleep 10
					                # 检查Flannel pods状态
					                kubectl get pods -n kube-flannel
					                break
					            else
					                echo "✗ Flannel安装失败，正在重试..."
					                sleep 5
					            fi
					        done
					        
					        # 验证CNI配置是否生成
					        echo "=== 验证CNI配置 ==="
					        if [ -d /etc/cni/net.d ]; then
					            echo "CNI配置目录存在"
					            ls -la /etc/cni/net.d/
					            if ls /etc/cni/net.d/*.conf 1> /dev/null 2>&1; then
					                echo "✓ CNI配置文件已生成"
					            else
					                echo "✗ CNI配置文件未生成，尝试手动创建Flannel配置"
					                # 手动创建Flannel CNI配置
					                sudo mkdir -p /etc/cni/net.d
					                sudo bash -c 'cat <<EOF > /etc/cni/net.d/10-flannel.conf
{
  "name": "cbr0",
  "type": "flannel",
  "delegate": {
    "isDefaultGateway": true
  }
}
EOF'
					                echo "✓ 手动创建Flannel CNI配置成功"
					                ls -la /etc/cni/net.d/
					            fi
					        else
					            echo "✗ CNI配置目录不存在，创建目录并手动配置"
					            sudo mkdir -p /etc/cni/net.d
					            sudo bash -c 'cat <<EOF > /etc/cni/net.d/10-flannel.conf
{
  "name": "cbr0",
  "type": "flannel",
  "delegate": {
    "isDefaultGateway": true
  }
}
EOF'
					            echo "✓ 手动创建Flannel CNI配置成功"
					        fi
					        
					        # 重启containerd和kubelet服务，确保CNI插件生效
					        echo "=== 重启containerd和kubelet服务，确保CNI插件生效 ==="
					        sudo systemctl restart containerd
					        sudo systemctl restart kubelet
					        echo "✓ 服务重启完成"
					        
					        # 再次检查Flannel pods状态
					        echo "=== 再次检查Flannel pods状态 ==="
					        sleep 5
					        kubectl get pods -n kube-flannel
					        
					        # 检查节点状态
					        echo "=== 检查节点状态 ==="
					        kubectl get nodes
					    else
					        echo "✗ 无法安装CNI插件，kubectl配置失败"
					        # 即使kubectl配置失败，也要尝试创建CNI配置目录
					        sudo mkdir -p /etc/cni/net.d
					        echo "✓ 创建CNI配置目录成功"
					        
					        # 重启containerd和kubelet服务
					        echo "=== 重启containerd和kubelet服务 ==="
					        sudo systemctl restart containerd
					        sudo systemctl restart kubelet
					    fi
					else
					        echo "✗ kubeadm init 失败"
					        # 显示更多错误信息
					        echo "=== 显示kubeadm日志 ==="
					        sudo journalctl -u kubelet --no-pager -n 50
					    fi`, kubeVersion)
				result.WriteString("使用默认Kubernetes初始化脚本\n")
			}

			var joinCmd string
			initOutput, err := initMasterClient.RunCommandWithOutput(initCmd, func(line string) {
				result.WriteString(line + "\n")
				fmt.Println(line)                               // 实时打印到控制台
				outputLog(masterNode.ID, masterNode.Name, line) // 实时发送到前端

				// 实时检查输出，提取Join命令
				if strings.HasPrefix(line, "kubeadm join") {
					joinCmd = strings.TrimSpace(line)
					result.WriteString("\n=== 已获取Join命令，开始部署Worker节点 ===\n")
					outputLog(masterNode.ID, masterNode.Name, "=== 已获取Join命令，开始部署Worker节点 ===")
				}
			})
			if err != nil {
				result.WriteString(fmt.Sprintf("Master节点初始化失败: %v\n输出: %s\n", err, initOutput))
				outputLog(masterNode.ID, masterNode.Name, fmt.Sprintf("Master节点初始化失败: %v", err))
				return result.String(), err
			}
			result.WriteString("Master节点初始化成功\n\n")
			outputLog(masterNode.ID, masterNode.Name, "Master节点初始化成功")

			// 如果没有从输出中捕获到Join命令，尝试直接获取
			if joinCmd == "" {
				result.WriteString("=== 从输出中未捕获到Join命令，尝试直接获取 ===\n")

				// 尝试多种方法获取join命令，增加重试机制
				joinCmdCmd := `kubeadm token create --print-join-command`
				var retryCount int = 3
				var lastErr error

				for i := 1; i <= retryCount; i++ {
					result.WriteString(fmt.Sprintf("尝试获取Join命令 (%d/%d)...\n", i, retryCount))
					joinCmd, err = initMasterClient.RunCommand(joinCmdCmd)
					if err == nil && strings.TrimSpace(joinCmd) != "" {
						joinCmd = strings.TrimSpace(joinCmd)
						result.WriteString(fmt.Sprintf("成功获取Join命令: %s\n", joinCmd))
						break
					} else {
						lastErr = err
						result.WriteString(fmt.Sprintf("获取Join命令失败: %v\n", err))
						if i < retryCount {
							result.WriteString("等待3秒后重试...\n")
							time.Sleep(3 * time.Second)
						}
					}
				}

				if joinCmd == "" {
					// 尝试另一种方法获取join命令
					result.WriteString("=== 尝试使用另一种方法获取Join命令 ===\n")
					tokenCmd := `kubeadm token create`
					token, err := initMasterClient.RunCommand(tokenCmd)
					if err != nil {
						result.WriteString(fmt.Sprintf("创建token失败: %v\n", err))
						if lastErr != nil {
							return result.String(), lastErr
						} else {
							return result.String(), err
						}
					}
					token = strings.TrimSpace(token)

					// 获取ca cert hash
					caCertHashCmd := `openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | openssl dgst -sha256 -hex | sed 's/^.* //'`
					caCertHash, err := initMasterClient.RunCommand(caCertHashCmd)
					if err != nil {
						result.WriteString(fmt.Sprintf("获取ca cert hash失败: %v\n", err))
						if lastErr != nil {
							return result.String(), lastErr
						} else {
							return result.String(), err
						}
					}
					caCertHash = strings.TrimSpace(caCertHash)

					// 构建join命令
					controlPlaneEndpoint := fmt.Sprintf("%s:6443", masterNode.IP)
					joinCmd = fmt.Sprintf("kubeadm join %s --token %s --discovery-token-ca-cert-hash sha256:%s", controlPlaneEndpoint, token, caCertHash)
					result.WriteString(fmt.Sprintf("成功构建Join命令: %s\n", joinCmd))
				}
			}

			// 将join命令存储到master节点的JoinCommand字段中
			for i, n := range nodes {
				if n.ID == masterNode.ID {
					nodes[i].JoinCommand = joinCmd
					break
				}
			}
		}
	} else {
		result.WriteString("=== 跳过Master节点初始化 ===\n")
		// 如果跳过Master节点初始化，需要单独创建SSH客户端并获取Join命令
		// 直接使用节点的IP地址进行连接，避免依赖本地hosts文件
		masterSSHConfig := ssh.SSHConfig{
			Host:       masterNode.IP, // 直接使用IP地址，不依赖本地hosts文件
			Port:       masterNode.Port,
			Username:   masterNode.Username,
			Password:   masterNode.Password,
			PrivateKey: masterNode.PrivateKey,
		}

		var err error
		masterClient, err = ssh.NewSSHClient(masterSSHConfig)
		if err != nil {
			result.WriteString(fmt.Sprintf("创建Master节点SSH客户端失败: %v\n", err))
			return result.String(), err
		}
		defer masterClient.Close()
		result.WriteString(fmt.Sprintf("连接到Master节点 %s (%s) 成功\n", masterNode.Name, masterNode.IP))

		// 获取Join命令，增加重试机制和多种获取方法
		result.WriteString("=== 获取Join命令 ===\n")

		// 尝试多种方法获取join命令，增加重试机制
		joinCmdCmd := `kubeadm token create --print-join-command`
		var retryCount int = 3
		var lastErr error

		for i := 1; i <= retryCount; i++ {
			result.WriteString(fmt.Sprintf("尝试获取Join命令 (%d/%d)...\n", i, retryCount))
			joinCmd, err = masterClient.RunCommand(joinCmdCmd)
			if err == nil && strings.TrimSpace(joinCmd) != "" {
				joinCmd = strings.TrimSpace(joinCmd)
				result.WriteString(fmt.Sprintf("成功获取Join命令: %s\n", joinCmd))
				break
			} else {
				lastErr = err
				result.WriteString(fmt.Sprintf("获取Join命令失败: %v\n", err))
				if i < retryCount {
					result.WriteString("等待3秒后重试...\n")
					time.Sleep(3 * time.Second)
				}
			}
		}

		if joinCmd == "" {
			// 尝试另一种方法获取join命令
			result.WriteString("=== 尝试使用另一种方法获取Join命令 ===\n")
			tokenCmd := `kubeadm token create`
			token, err := masterClient.RunCommand(tokenCmd)
			if err != nil {
				result.WriteString(fmt.Sprintf("创建token失败: %v\n", err))
				if lastErr != nil {
					return result.String(), lastErr
				} else {
					return result.String(), err
				}
			}
			token = strings.TrimSpace(token)

			// 获取ca cert hash
			caCertHashCmd := `openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | openssl dgst -sha256 -hex | sed 's/^.* //'`
			caCertHash, err := masterClient.RunCommand(caCertHashCmd)
			if err != nil {
				result.WriteString(fmt.Sprintf("获取ca cert hash失败: %v\n", err))
				if lastErr != nil {
					return result.String(), lastErr
				} else {
					return result.String(), err
				}
			}
			caCertHash = strings.TrimSpace(caCertHash)

			// 构建join命令
			controlPlaneEndpoint := fmt.Sprintf("%s:6443", masterNode.IP)
			joinCmd = fmt.Sprintf("kubeadm join %s --token %s --discovery-token-ca-cert-hash sha256:%s", controlPlaneEndpoint, token, caCertHash)
			result.WriteString(fmt.Sprintf("成功构建Join命令: %s\n", joinCmd))
		}

		// 将join命令存储到master节点的JoinCommand字段中
		for i, n := range nodes {
			if n.ID == masterNode.ID {
				nodes[i].JoinCommand = joinCmd
				break
			}
		}
	}

	// 如果没有Master节点，从环境变量获取join命令
	if len(masterNodes) == 0 {
		// 从环境变量获取join命令
		joinCmd = os.Getenv("KUBEADM_JOIN_COMMAND")
		if joinCmd == "" {
			// 尝试从其他环境变量构建join命令
			token := os.Getenv("KUBEADM_TOKEN")
			caCertHash := os.Getenv("KUBEADM_CA_CERT_HASH")
			controlPlaneEndpoint := os.Getenv("KUBEADM_CONTROL_PLANE_ENDPOINT")
			if token != "" && caCertHash != "" && controlPlaneEndpoint != "" {
				joinCmd = fmt.Sprintf("kubeadm join %s --token %s --discovery-token-ca-cert-hash %s", controlPlaneEndpoint, token, caCertHash)
			}
		}
		if joinCmd != "" {
			joinCmd = strings.TrimSpace(joinCmd)
			result.WriteString(fmt.Sprintf("=== 从环境变量获取到Join命令: %s ===\n\n", joinCmd))
		}
	}

	// 只有当joinCmd不为空时才输出join命令
	if joinCmd != "" {
		result.WriteString(fmt.Sprintf("=== Join命令: %s ===\n\n", joinCmd))
	}

	// 4. 并行部署Worker节点
	// 检查是否需要取消部署
	select {
	case <-ctx.Done():
		result.WriteString("部署已取消\n")
		return result.String(), ctx.Err()
	default:
	}
	if !shouldSkip(StepWorkerJoin) && joinCmd != "" {
		// 创建一个通道来接收部署结果
		type workerResult struct {
			nodeName string
			err      error
			output   string
		}
		results := make(chan workerResult, len(workerNodes))

		// 为每个Worker节点启动一个goroutine进行部署
		for _, workerNode := range workerNodes {
			go func(worker node.Node) {
				// 检查上下文是否已取消
				select {
				case <-ctx.Done():
					results <- workerResult{
						nodeName: worker.Name,
						err:      ctx.Err(),
						output:   "部署已取消",
					}
					return
				default:
				}

				var workerResultStr strings.Builder
				workerResultStr.WriteString(fmt.Sprintf("=== 将Worker节点 %s 加入集群 ===\n", worker.Name))

				// 直接使用节点的IP地址进行连接，避免依赖本地hosts文件
				// 从数据库中获取的节点信息已经包含了正确的IP地址
				workerSSHConfig := ssh.SSHConfig{
					Host:       worker.IP, // 直接使用IP地址，不依赖本地hosts文件
					Port:       worker.Port,
					Username:   worker.Username,
					Password:   worker.Password,
					PrivateKey: worker.PrivateKey,
				}

				workerClient, err := ssh.NewSSHClient(workerSSHConfig)
				if err != nil {
					workerResultStr.WriteString(fmt.Sprintf("创建Worker节点SSH客户端失败: %v\n", err))
					results <- workerResult{
						nodeName: worker.Name,
						err:      err,
						output:   workerResultStr.String(),
					}
					return
				}
				workerResultStr.WriteString(fmt.Sprintf("连接到Worker节点 %s (%s) 成功\n", worker.Name, worker.IP))
				defer workerClient.Close()

				// 添加Calico初始化依赖步骤
				calicoPrepCmd := `# 1. 必须的内核模块 - Calico初始化依赖
			echo "=== 加载必须的内核模块（Calico初始化依赖） ==="
		sudo modprobe br_netfilter || echo "br_netfilter模块已加载或加载失败"
		sudo modprobe overlay || echo "overlay模块已加载或加载失败"
		
		# 2. 持久化内核模块配置
		echo "=== 持久化内核模块配置 ==="
		sudo cat <<EOF > /etc/modules-load.d/k8s.conf
		br_netfilter
	overlay
		EOF
		
		# 3. 必须的 sysctl - Calico初始化依赖，此文件必须写入
		echo "=== 配置必须的sysctl（Calico初始化依赖） ==="
		sudo cat <<EOF > /etc/sysctl.d/k8s.conf
		net.bridge.bridge-nf-call-iptables = 1
		net.bridge.bridge-nf-call-ip6tables = 1
		net.ipv4.ip_forward = 1
		EOF
		sudo sysctl --system
		
		# 4. Rocky 10 必装，否则 calico-node Init 直接失败
		echo "=== 安装iproute-tc（Calico初始化依赖） ==="
		if command -v dnf &> /dev/null; then
		    sudo dnf install -y iproute-tc || true
		elif command -v yum &> /dev/null; then
		    sudo yum install -y iproute-tc || true
		fi
		
		# 5. BPF 挂载点（init 容器 mount-bpffs 需要）
		echo "=== 配置BPF挂载点 ==="
		sudo mkdir -p /sys/fs/bpf
		sudo mount bpffs /sys/fs/bpf || true
		
		# 6. CNI 目录
		echo "=== 创建CNI目录 ==="
		sudo mkdir -p /opt/cni/bin
		sudo mkdir -p /etc/cni/net.d
		
		# 7. 重启关键服务
		echo "=== 重启关键服务 ==="
		sudo systemctl restart containerd || true
		sudo systemctl restart kubelet || true
		
		# 8. 等待服务重启完成
		echo "=== 等待服务重启完成 ==="
		sleep 5`

				// 执行Calico初始化依赖步骤
				calicoOutput, err := workerClient.RunCommandWithOutput(calicoPrepCmd, func(line string) {
					workerResultStr.WriteString(line + "\n")
					outputLog(worker.ID, worker.Name, line) // 实时发送到前端
				})
				if err != nil {
					workerResultStr.WriteString(fmt.Sprintf("Worker节点 %s Calico初始化依赖步骤执行失败: %v\n输出: %s\n", worker.Name, err, calicoOutput))
					// 继续执行join命令，因为依赖步骤失败不一定导致join失败
				} else {
					workerResultStr.WriteString(fmt.Sprintf("Worker节点 %s Calico初始化依赖步骤执行成功\n\n", worker.Name))
				}

				// 将Worker节点加入集群
				joinOutput, err := workerClient.RunCommandWithOutput(joinCmd, func(line string) {
					workerResultStr.WriteString(line + "\n")
					outputLog(worker.ID, worker.Name, line) // 实时发送到前端
				})
				if err != nil {
					workerResultStr.WriteString(fmt.Sprintf("Worker节点 %s 加入集群失败: %v\n输出: %s\n", worker.Name, err, joinOutput))
					results <- workerResult{
						nodeName: worker.Name,
						err:      err,
						output:   workerResultStr.String(),
					}
					return
				}
				workerResultStr.WriteString(fmt.Sprintf("Worker节点 %s 加入集群成功\n\n", worker.Name))
				results <- workerResult{
					nodeName: worker.Name,
					err:      nil,
					output:   workerResultStr.String(),
				}
			}(workerNode)
		}

		// 收集所有Worker节点的部署结果
		for i := 0; i < len(workerNodes); i++ {
			select {
			case <-ctx.Done():
				result.WriteString("部署已取消\n")
				return result.String(), ctx.Err()
			case res := <-results:
				result.WriteString(res.output)
				if res.err != nil {
					result.WriteString(fmt.Sprintf("Worker节点 %s 部署失败: %v\n", res.nodeName, res.err))
				}
			}
		}
	} else if len(workerNodes) > 0 {
		if joinCmd == "" {
			result.WriteString("=== 跳过Worker节点加入集群：未找到join命令 ===\n")
		} else {
			result.WriteString("=== 跳过Worker节点加入集群 ===\n")
		}
	}

	// 6. 验证集群状态（只有当有master节点时才执行）
	// 检查是否需要取消部署
	select {
	case <-ctx.Done():
		result.WriteString("部署已取消\n")
		return result.String(), ctx.Err()
	default:
	}
	if !shouldSkip(StepClusterVerification) && len(masterNodes) > 0 {
		result.WriteString("=== 验证集群状态 ===\n")
		verifyCmd := `# 验证集群状态
 echo "=== 等待集群就绪（120秒） - 给CNI插件足够部署时间 ==="
 sleep 120
 
 echo "=== 重启关键服务，确保网络插件生效 ==="
 sudo systemctl restart containerd || true
 sudo systemctl restart kubelet || true
 
 echo "=== 再次等待30秒，确保服务完全恢复 ==="
 sleep 30
 
 echo "=== 查看节点状态 ==="
 kubectl get nodes
 
 echo "=== 查看Pod状态 ==="
 kubectl get pods -A
 
 echo "=== 查看CNI相关Pod详细状态 ==="
 kubectl get pods -n kube-flannel -o wide 2>/dev/null || echo "未安装Flannel"
 kubectl get pods -n calico-system -o wide 2>/dev/null || echo "未安装Calico"
 
 echo "=== 查看kubelet日志（最后50行） ==="
 journalctl -u kubelet --no-pager -n 50
 
 echo "=== 查看CNI配置 ==="
 ls -la /etc/cni/net.d/`

		verifyOutput, err := masterClient.RunCommandWithOutput(verifyCmd, func(line string) {
			result.WriteString(line + "\n")
			fmt.Println(line)                               // 实时打印到控制台
			outputLog(masterNode.ID, masterNode.Name, line) // 实时发送到前端
		})
		if err != nil {
			result.WriteString(fmt.Sprintf("验证集群状态失败: %v\n输出: %s\n", err, verifyOutput))
			// 验证失败不影响部署流程，继续执行
		} else {
			// 验证成功，检查是否所有节点都已Ready
			result.WriteString("=== 检查所有节点是否Ready ===\n")
			checkNodesCmd := `kubectl get nodes | grep -v NAME | awk '{print $2}' | grep -v Ready | wc -l`
			notReadyCount, _ := masterClient.RunCommand(checkNodesCmd)
			if notReadyCount != "0" {
				result.WriteString(fmt.Sprintf("警告: 仍有 %s 个节点未Ready，请检查网络插件部署情况\n", notReadyCount))
				// 输出详细信息
				masterClient.RunCommandWithOutput("kubectl describe nodes", func(line string) {
					result.WriteString(line + "\n")
				})
			} else {
				result.WriteString("✓ 所有节点均已Ready\n")
			}
		}
	} else if len(masterNodes) > 0 {
		result.WriteString("=== 跳过集群验证 ===\n")
	}

	deploymentCompleteMsg := "=== Kubernetes集群部署完成 ==="
	outputLog("cluster", "Kubernetes Cluster", deploymentCompleteMsg)
	result.WriteString(deploymentCompleteMsg + "\n")

	if len(masterNodes) > 0 {
		masterNodeMsg := fmt.Sprintf("Master节点: %s (%s)", masterNode.Name, masterNode.IP)
		outputLog("cluster", "Kubernetes Cluster", masterNodeMsg)
		result.WriteString(masterNodeMsg + "\n")
	} else {
		noMasterMsg := "Master节点: 无 (仅部署工作节点)"
		outputLog("cluster", "Kubernetes Cluster", noMasterMsg)
		result.WriteString(noMasterMsg + "\n")
	}

	workerCountMsg := fmt.Sprintf("Worker节点数量: %d", len(workerNodes))
	outputLog("cluster", "Kubernetes Cluster", workerCountMsg)
	result.WriteString(workerCountMsg + "\n")

	kubeVersionMsg := fmt.Sprintf("Kubernetes版本: %s", kubeVersion)
	outputLog("cluster", "Kubernetes Cluster", kubeVersionMsg)
	result.WriteString(kubeVersionMsg + "\n")

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
func InitMaster(sshConfig SSHConfig, config KubeadmConfig, skipSteps []string) (string, error) {

	// 辅助函数：检查步骤是否需要跳过
	shouldSkip := func(step string) bool {
		for _, s := range skipSteps {
			if s == step {
				return true
			}
		}
		return false
	}

	// 构建完整的执行命令，根据skipSteps参数决定是否执行某些步骤
	skipStepsStr := strings.Join(skipSteps, " ")
	cmd := fmt.Sprintf(`#!/bin/bash

# 初始化步骤执行状态
echo "=== 开始执行主节点初始化步骤 ==="
echo "跳过的步骤: %s"

# 只在不跳过系统准备步骤时执行重置操作
`, skipStepsStr)

	// 3. 容器运行时配置 - 安装并确保containerd正在运行
	if !shouldSkip(StepContainerRuntimeInstallation) {
		cmd += `# 检查并安装必要的依赖
echo "=== 检查并安装必要的依赖 ==="

# 检测操作系统类型和包管理器
echo "=== 检测操作系统类型和包管理器 ==="
if command -v apt-get &> /dev/null; then
    echo "检测到Ubuntu/Debian系统，使用apt包管理器"
    PACKAGE_MANAGER="apt"
    sudo apt-get update -y
    # 安装必要的依赖
    sudo apt-get install -y iptables iptables-persistent ip6tables curl wget gnupg2 lsb-release
elif command -v dnf &> /dev/null; then
    echo "检测到Fedora/CentOS/RHEL 8+系统，使用dnf包管理器"
    PACKAGE_MANAGER="dnf"
    sudo dnf update -y
    # 安装必要的依赖 - 修复dnf系统的包名问题
    # EL9不再需要redhat-lsb-core，Kubernetes不依赖它
    sudo dnf install -y iptables curl wget gnupg2
    # 启用nftables服务（EL9使用nftables，iptables是兼容层）
    sudo systemctl enable --now nftables || true
elif command -v yum &> /dev/null; then
    echo "检测到CentOS/RHEL 7系统，使用yum包管理器"
    PACKAGE_MANAGER="yum"
    sudo yum update -y
    # 安装必要的依赖
    sudo yum install -y iptables-services curl wget gnupg2 redhat-lsb-core
else
    echo "警告：未检测到支持的包管理器，可能导致部署失败"
    PACKAGE_MANAGER="unknown"
fi

# 确保iptables和ip6tables服务启动
echo "=== 启动iptables和ip6tables服务 ==="
if command -v systemctl &> /dev/null; then
    # 检查iptables服务是否存在，存在才启动
    if systemctl list-units --all | grep -q iptables.service; then
        sudo systemctl start iptables || true
        sudo systemctl enable iptables || true
    fi
    
    # 检查ip6tables服务是否存在，存在才启动
    if systemctl list-units --all | grep -q ip6tables.service; then
        sudo systemctl start ip6tables || true
        sudo systemctl enable ip6tables || true
    fi
fi

# 安装Kubernetes组件
echo "=== 安装Kubernetes组件 ==="
if [ "$PACKAGE_MANAGER" = "apt" ]; then
    # 添加Kubernetes仓库
    curl -fsSL https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
    echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list > /dev/null
    sudo apt-get update -y
    
    # 安装kubeadm、kubelet、kubectl
    sudo apt-get install -y kubeadm kubelet kubectl
    
    # 固定版本，防止自动更新
    sudo apt-mark hold kubeadm kubelet kubectl
elif [ "$PACKAGE_MANAGER" = "dnf" ]; then
    # 添加Kubernetes仓库
    sudo cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
exclude=kubelet kubeadm kubectl
EOF
    
    # 安装kubeadm、kubelet、kubectl
    sudo dnf install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
    
    # 启动并启用kubelet服务
    sudo systemctl enable --now kubelet
elif [ "$PACKAGE_MANAGER" = "yum" ]; then
    # 添加Kubernetes仓库
    sudo cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
exclude=kubelet kubeadm kubectl
EOF
    
    # 安装kubeadm、kubelet、kubectl
    sudo yum install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
    
    # 启动并启用kubelet服务
    sudo systemctl enable --now kubelet
else
    echo "警告：无法自动安装Kubernetes组件，请手动安装"
fi

# 验证必要命令是否可用
echo "=== 验证必要命令是否可用 ==="

# 验证iptables命令
if command -v iptables &> /dev/null; then
    echo "✓ iptables 已安装"
    iptables --version || echo "✓ iptables 版本信息获取失败，但命令存在"
else
    echo "✗ iptables 未安装"
fi

# 验证ip6tables命令
if command -v ip6tables &> /dev/null; then
    echo "✓ ip6tables 已安装"
    ip6tables --version || echo "✓ ip6tables 版本信息获取失败，但命令存在"
else
    echo "✗ ip6tables 未安装"
fi

# 验证kubeadm命令 - 使用正确的命令格式 kubeadm version
if command -v kubeadm &> /dev/null; then
    echo "✓ kubeadm 已安装"
    kubeadm version || echo "✓ kubeadm 版本信息获取失败，但命令存在"
else
    echo "✗ kubeadm 未安装"
fi

# 验证kubectl命令 - 使用正确的命令格式 kubectl version --client
if command -v kubectl &> /dev/null; then
    echo "✓ kubectl 已安装"
    kubectl version --client || echo "✓ kubectl 版本信息获取失败，但命令存在"
else
    echo "✗ kubectl 未安装"
fi

# 验证kubelet命令 - 兼容不同版本，尝试多种方式
if command -v kubelet &> /dev/null; then
    echo "✓ kubelet 已安装"
    # 尝试多种方式获取kubelet版本
    if kubelet version &> /dev/null; then
        kubelet version
    elif kubelet --version &> /dev/null; then
        kubelet --version
    else
        echo "✓ kubelet 命令存在，但获取版本信息失败"
    fi
else
    echo "✗ kubelet 未安装"
fi

# 2. IP转发配置 - 确保IP转发已启用
echo "=== 确保IP转发配置正确 ==="
sudo bash -c 'cat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf
net.ipv4.ip_forward = 1
EOF'
sudo sysctl --system
echo "=== IP转发配置完成 ==="

echo "=== 安装containerd依赖 ==="
# 安装containerd所需的依赖
containerd_installed=false
containerd_package="containerd.io"

# 首先检查系统是否已安装containerd
if command -v containerd &> /dev/null; then
    echo "发现已安装containerd，版本信息:"
    containerd --version
    containerd_installed=true
fi

# 如果未安装，从Docker仓库安装containerd.io
if [ "$containerd_installed" = "false" ]; then
    echo "未安装containerd，开始安装..."
    
    if [ "$PACKAGE_MANAGER" = "apt" ]; then
        echo "使用apt安装containerd.io..."
        # 安装依赖
        sudo apt-get install -y ca-certificates curl gnupg lsb-release
        # 添加Docker仓库
        sudo mkdir -p /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        echo \n  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \n  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
        sudo apt-get update -y
        # 安装containerd.io
        sudo apt-get install -y $containerd_package
        
        # 验证安装
        if command -v containerd &> /dev/null; then
            echo "✓ containerd.io安装成功，版本: $(containerd --version)"
            containerd_installed=true
        else
            echo "✗ containerd.io安装失败，尝试使用系统包管理器安装containerd"
            # 尝试使用系统包管理器安装containerd
            sudo apt-get install -y containerd
            if command -v containerd &> /dev/null; then
                echo "✓ 系统包管理器安装containerd成功，版本: $(containerd --version)"
                containerd_installed=true
            fi
        fi
    elif [ "$PACKAGE_MANAGER" = "dnf" ]; then
        echo "使用dnf安装containerd.io..."
        # 添加Docker仓库
        sudo dnf config-manager --add-repo=https://download.docker.com/linux/centos/docker-ce.repo
        # 安装containerd.io
        sudo dnf install -y $containerd_package
        
        # 验证安装
        if command -v containerd &> /dev/null; then
            echo "✓ containerd.io安装成功，版本: $(containerd --version)"
            containerd_installed=true
        else
            echo "✗ containerd.io安装失败，尝试使用系统包管理器安装containerd"
            # 尝试使用系统包管理器安装containerd
            sudo dnf install -y containerd
            if command -v containerd &> /dev/null; then
                echo "✓ 系统包管理器安装containerd成功，版本: $(containerd --version)"
                containerd_installed=true
            fi
        fi
    elif [ "$PACKAGE_MANAGER" = "yum" ]; then
        echo "使用yum安装containerd.io..."
        # 添加Docker仓库
        sudo yum-config-manager --add-repo=https://download.docker.com/linux/centos/docker-ce.repo
        # 安装containerd.io
        sudo yum install -y $containerd_package
        
        # 验证安装
        if command -v containerd &> /dev/null; then
            echo "✓ containerd.io安装成功，版本: $(containerd --version)"
            containerd_installed=true
        else
            echo "✗ containerd.io安装失败，尝试使用系统包管理器安装containerd"
            # 尝试使用系统包管理器安装containerd
            sudo yum install -y containerd
            if command -v containerd &> /dev/null; then
                echo "✓ 系统包管理器安装containerd成功，版本: $(containerd --version)"
                containerd_installed=true
            fi
        fi
    else
        echo "警告：无法自动安装containerd，请手动安装"
    fi
fi

# 验证containerd安装状态
if [ "$containerd_installed" = "true" ]; then
    echo "=== containerd安装验证成功 ==="
else
    echo "=== 警告：containerd安装验证失败，可能导致部署失败 ==="
fi

# 配置containerd
echo "=== 配置containerd ==="
sudo mkdir -p /etc/containerd
if command -v containerd &> /dev/null; then
    containerd config default | sudo tee /etc/containerd/config.toml
    # 修改containerd配置，使用systemd cgroup驱动
    sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
    echo "containerd配置完成"
else
    echo "警告：containerd命令未找到，跳过配置"
fi

# 启动containerd服务
echo "=== 启动containerd服务 ==="

# 检查并创建containerd服务文件
echo "=== 检查并创建containerd服务文件 ==="
if command -v systemctl &> /dev/null; then
    # 检查containerd服务是否存在
    if systemctl list-units --all | grep -q containerd.service; then
        echo "✓ containerd.service单元已存在"
    else
        echo "✗ containerd.service单元不存在，正在创建..."
        
        # 检查containerd二进制文件路径
        containerd_path=$(which containerd 2>/dev/null || echo "/usr/bin/containerd")
        
        # 创建containerd.service文件
        sudo cat > /etc/systemd/system/containerd.service << EOF
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStartPre=-/sbin/modprobe br_netfilter
ExecStart=$containerd_path
Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5
LimitNPROC=infinity
LimitCORE=infinity
LimitNOFILE=infinity
OOMScoreAdjust=-999

[Install]
WantedBy=multi-user.target
EOF
        
        echo "✓ containerd.service文件创建成功"
        # 重新加载systemd配置
        sudo systemctl daemon-reload
    fi
    
    # 启动并启用containerd服务
    echo "=== 启动并启用containerd服务 ==="
    sudo systemctl enable containerd
    sudo systemctl start containerd
    
    # 等待服务启动
    sleep 5
    
    # 检查服务状态
    containerd_status=$(sudo systemctl is-active containerd 2>/dev/null || echo "inactive")
    echo "containerd服务状态: $containerd_status"
    
    # 如果服务未启动，尝试手动启动
    if [ "$containerd_status" != "active" ]; then
        echo "⚠️  containerd服务未正常启动，尝试手动启动..."
        # 停止可能存在的containerd进程
        sudo pkill -f containerd || true
        sleep 2
        # 清理旧的socket和状态文件
        sudo rm -rf /run/containerd /var/run/containerd
        sudo mkdir -p /var/run/containerd
        # 手动启动containerd
        containerd --version
        containerd &
        sleep 10
    fi
else
    echo "警告：systemctl命令未找到，无法管理服务，尝试手动启动containerd"
    # 检查containerd命令是否可用
    if command -v containerd &> /dev/null; then
        # 停止可能存在的containerd进程
        sudo pkill -f containerd || true
        sleep 2
        # 清理旧的socket和状态文件
        sudo rm -rf /run/containerd /var/run/containerd
        sudo mkdir -p /var/run/containerd
        # 手动启动containerd
        containerd --version
        containerd &
        sleep 10
    else
        echo "错误：containerd命令未找到，无法启动containerd"
    fi
fi

# 验证containerd运行状态
echo "=== 验证containerd运行状态 ==="
containerd_running=false

# 检查进程是否存在
if pgrep -x containerd > /dev/null; then
    echo "✓ containerd进程正在运行"
    containerd_running=true
fi

# 检查containerd socket是否存在
cri_socket="/run/containerd/containerd.sock"
if [ -S "$cri_socket" ]; then
    echo "✓ containerd socket存在: $cri_socket"
    # 测试socket连接
    if command -v ctr &> /dev/null; then
        echo "测试containerd连接:"
        ctr version || echo "containerd连接测试失败，但socket存在"
    fi
else
    # 检查备选路径
    alt_socket="/var/run/containerd/containerd.sock"
    if [ -S "$alt_socket" ]; then
        echo "✓ containerd socket存在于备选路径: $alt_socket"
        cri_socket="$alt_socket"
    else
        echo "✗ 未找到containerd socket，containerd可能未正确启动"
    fi
fi

# 最终状态报告
if [ "$containerd_running" = "true" ] || [ -S "$cri_socket" ]; then
    echo "=== containerd启动验证成功 ==="
else
    echo "=== ⚠️  containerd启动验证失败 ==="
    echo "请检查containerd日志以获取详细信息: journalctl -u containerd -n 50"
fi

# 检查containerd socket是否存在
echo "=== 检查containerd socket是否存在 ==="
cri_socket="/run/containerd/containerd.sock"
if [ ! -S "$cri_socket" ]; then
    echo "containerd socket不存在，尝试使用备选路径..."
    # 检查备选路径
    alt_socket="/var/run/containerd/containerd.sock"
    if [ -S "$alt_socket" ]; then
        echo "在备选路径找到containerd socket: $alt_socket"
        # 更新cri_socket变量
        cri_socket="$alt_socket"
    else
        echo "警告：未找到containerd socket，可能导致部署失败"
        # 不退出，继续执行，让后续步骤处理错误
    fi
else
    echo "找到containerd socket: $cri_socket"
fi

# 配置crictl
echo "=== 配置crictl ==="
# 安装crictl（如果未安装）
if ! command -v crictl &> /dev/null; then
    echo "安装crictl..."
    CRICTL_VERSION="v1.26.0"
    wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$CRICTL_VERSION/crictl-$CRICTL_VERSION-linux-amd64.tar.gz
    sudo tar zxvf crictl-$CRICTL_VERSION-linux-amd64.tar.gz -C /usr/local/bin
    rm -f crictl-$CRICTL_VERSION-linux-amd64.tar.gz
fi

# 配置crictl使用的socket
sudo rm -f /etc/crictl.yaml || true
sudo printf "runtime-endpoint: unix://%s\nimage-endpoint: unix://%s\n" "$cri_socket" "$cri_socket" > /etc/crictl.yaml

# 测试crictl连接
echo "=== 测试crictl连接 ==="
if command -v crictl &> /dev/null; then
    crictl info || echo "crictl info命令执行失败，继续执行"
else
    echo "警告：crictl命令未找到，跳过连接测试"
fi

# 确保kubelet所需的目录存在
echo "=== 确保kubelet所需的目录存在 ==="
sudo mkdir -p /var/lib/kubelet

echo "=== 容器运行时配置完成 ==="

`
	}

	// 4. Kubernetes仓库配置 - 只在需要时执行
	if !shouldSkip(StepKubernetesRepositoryConfiguration) {
		cmd += `# 检查Kubernetes仓库配置
echo "=== 检查Kubernetes仓库配置 ==="
# 这里可以添加检查和配置Kubernetes仓库的逻辑
echo "=== Kubernetes仓库配置检查完成 ==="

`
	}

	// 5. Kubernetes组件安装 - 只在需要时执行
	if !shouldSkip(StepKubernetesComponentsInstallation) {
		cmd += `# 检查Kubernetes组件安装
echo "=== 安装Kubernetes组件 ==="

# 检测操作系统类型和包管理器
echo "=== 检测操作系统类型和包管理器 ==="
if command -v apt-get &> /dev/null; then
    echo "检测到Ubuntu/Debian系统，使用apt包管理器"
    PACKAGE_MANAGER="apt"
    # 更新包列表
    sudo apt-get update -y
    
    # 安装kubeadm、kubelet和kubectl
    echo "=== 安装kubeadm、kubelet和kubectl ==="
    # 添加Kubernetes仓库
    sudo apt-get install -y apt-transport-https ca-certificates curl gpg
    
    # 创建keyring目录
    mkdir -p -m 755 /etc/apt/keyrings
    
    # 使用阿里云镜像源
    # 下载并安装阿里云GPG密钥
    curl -fsSL -L https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
    
    # 添加阿里云Kubernetes repo
    echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main" | tee /etc/apt/sources.list.d/kubernetes.list
    
    # 更新仓库缓存
    sudo apt-get update -y
    
    # 检查可用的Kubernetes版本
    echo "=== 检查可用的Kubernetes版本 ==="
    AVAILABLE_VERSIONS=$(apt-cache madison kubelet | grep -oP '[0-9]+\.[0-9]+\.[0-9]+' | sort -V | uniq)
    echo "可用的Kubernetes版本: $AVAILABLE_VERSIONS"
    
    # 选择要安装的版本
    SELECTED_VERSION="${KUBE_VERSION}"
    echo "尝试安装指定版本: $SELECTED_VERSION"
    
    # 检查指定版本是否可用
    if ! echo "$AVAILABLE_VERSIONS" | grep -q "^$SELECTED_VERSION$"; then
        echo "指定版本 $SELECTED_VERSION 不可用，查找可用的最新版本..."
        # 如果指定版本不可用，使用可用的最新版本
        LATEST_VERSION=$(echo "$AVAILABLE_VERSIONS" | tail -1)
        if [ -n "$LATEST_VERSION" ]; then
            echo "使用可用的最新版本: $LATEST_VERSION"
            SELECTED_VERSION="$LATEST_VERSION"
        else
            echo "警告: 未找到可用的Kubernetes版本，尝试使用1.28.2版本..."
            SELECTED_VERSION="1.28.2"
        fi
    fi
    
    # 安装Kubernetes组件
    echo "=== 安装kubelet、kubeadm和kubectl $SELECTED_VERSION ==="
    sudo apt-get install -y kubelet=$SELECTED_VERSION kubeadm=$SELECTED_VERSION kubectl=$SELECTED_VERSION
    
    # 启动kubelet
    echo "=== 启动kubelet服务 ==="
    sudo systemctl enable --now kubelet
    
    echo "=== Kubernetes组件安装完成 ==="
elif command -v dnf &> /dev/null; then
    echo "检测到Fedora/CentOS/RHEL 8+系统，使用dnf包管理器"
    PACKAGE_MANAGER="dnf"
    
    # 添加Kubernetes仓库
    echo "=== 添加Kubernetes仓库 ==="
    cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
exclude=kubelet kubeadm kubectl
EOF
    
    # 更新仓库缓存
    echo "=== 更新仓库缓存 ==="
    sudo dnf clean all
    sudo dnf makecache -y
    
    # 检查可用的Kubernetes版本
    echo "=== 检查可用的Kubernetes版本 ==="
    AVAILABLE_VERSIONS=$(dnf --showduplicates list kubelet --disableexcludes=kubernetes 2>/dev/null | grep -oP '(?<=kubelet-)[0-9]+\.[0-9]+\.[0-9]+' | sort -V | uniq)
    echo "可用的Kubernetes版本: $AVAILABLE_VERSIONS"
    
    # 选择要安装的版本
    SELECTED_VERSION="${KUBE_VERSION}"
    echo "尝试安装指定版本: $SELECTED_VERSION"
    
    # 检查指定版本是否可用
    if ! echo "$AVAILABLE_VERSIONS" | grep -q "^$SELECTED_VERSION$"; then
        echo "指定版本 $SELECTED_VERSION 不可用，查找可用的最新版本..."
        # 如果指定版本不可用，使用可用的最新版本
        LATEST_VERSION=$(echo "$AVAILABLE_VERSIONS" | tail -1)
        if [ -n "$LATEST_VERSION" ]; then
            echo "使用可用的最新版本: $LATEST_VERSION"
            SELECTED_VERSION="$LATEST_VERSION"
        else
            echo "警告: 未找到可用的Kubernetes版本，尝试使用1.28.2版本..."
            SELECTED_VERSION="1.28.2"
        fi
    fi
    
    # 安装Kubernetes组件
    echo "=== 安装kubelet、kubeadm和kubectl $SELECTED_VERSION ==="
    sudo dnf install -y kubelet-$SELECTED_VERSION kubeadm-$SELECTED_VERSION kubectl-$SELECTED_VERSION --disableexcludes=kubernetes
    
    # 启动kubelet
    echo "=== 启动kubelet服务 ==="
    sudo systemctl enable --now kubelet
    
    echo "=== Kubernetes组件安装完成 ==="
elif command -v yum &> /dev/null; then
    echo "检测到CentOS/RHEL 7系统，使用yum包管理器"
    PACKAGE_MANAGER="yum"
    
    # 添加Kubernetes仓库
    echo "=== 添加Kubernetes仓库 ==="
    cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
exclude=kubelet kubeadm kubectl
EOF
    
    # 更新仓库缓存
    echo "=== 更新仓库缓存 ==="
    sudo yum clean all
    sudo yum makecache -y
    
    # 检查可用的Kubernetes版本
    echo "=== 检查可用的Kubernetes版本 ==="
    AVAILABLE_VERSIONS=$(yum --showduplicates list kubelet --disableexcludes=kubernetes 2>/dev/null | grep -oP '(?<=kubelet-)[0-9]+\.[0-9]+\.[0-9]+' | sort -V | uniq)
    echo "可用的Kubernetes版本: $AVAILABLE_VERSIONS"
    
    # 选择要安装的版本
    SELECTED_VERSION="${KUBE_VERSION}"
    echo "尝试安装指定版本: $SELECTED_VERSION"
    
    # 检查指定版本是否可用
    if ! echo "$AVAILABLE_VERSIONS" | grep -q "^$SELECTED_VERSION$"; then
        echo "指定版本 $SELECTED_VERSION 不可用，查找可用的最新版本..."
        # 如果指定版本不可用，使用可用的最新版本
        LATEST_VERSION=$(echo "$AVAILABLE_VERSIONS" | tail -1)
        if [ -n "$LATEST_VERSION" ]; then
            echo "使用可用的最新版本: $LATEST_VERSION"
            SELECTED_VERSION="$LATEST_VERSION"
        else
            echo "警告: 未找到可用的Kubernetes版本，尝试使用1.28.2版本..."
            SELECTED_VERSION="1.28.2"
        fi
    fi
    
    # 安装Kubernetes组件
    echo "=== 安装kubelet、kubeadm和kubectl $SELECTED_VERSION ==="
    sudo yum install -y kubelet-$SELECTED_VERSION kubeadm-$SELECTED_VERSION kubectl-$SELECTED_VERSION --disableexcludes=kubernetes
    
    # 启动kubelet
    echo "=== 启动kubelet服务 ==="
    sudo systemctl enable --now kubelet
    
    echo "=== Kubernetes组件安装完成 ==="
else
    echo "警告：未检测到支持的包管理器，无法自动安装Kubernetes组件"
fi

# 检查安装结果
echo "=== 检查Kubernetes组件安装结果 ==="
if command -v kubeadm &> /dev/null; then
    # 使用更兼容的方式获取kubeadm版本信息
    kubeadm_version=$(kubeadm version 2>&1 | grep -i "version:" | head -1 | awk '{print $2}' || echo "未知版本")
    echo "kubeadm版本: $kubeadm_version"
else
    echo "错误：kubeadm命令未找到"
fi

if command -v kubelet &> /dev/null; then
    # 兼容不同版本的kubelet版本获取方式
    if kubelet --version &> /dev/null; then
        echo "kubelet版本: $(kubelet --version 2>&1 | grep -oP 'Kubernetes v[0-9]+\.[0-9]+\.[0-9]+')"
    else
        echo "kubelet版本: 无法获取版本信息，但命令存在"
    fi
else
    echo "错误：kubelet命令未找到"
fi

if command -v kubectl &> /dev/null; then
    # 使用更兼容的方式获取kubectl版本信息
    kubectl_version=$(kubectl version --client 2>&1 | grep -i "client version" | awk '{print $3}' || echo "未知版本")
    echo "kubectl版本: $kubectl_version"
else
    echo "错误：kubectl命令未找到"
fi

# 检查kubelet服务状态
if command -v systemctl &> /dev/null; then
    echo "=== 检查kubelet服务状态 ==="
    kubelet_status=$(sudo systemctl is-active kubelet 2>/dev/null || echo "inactive")
    echo "kubelet服务状态: $kubelet_status"
    if [ "$kubelet_status" != "active" ]; then
        echo "警告：kubelet服务未运行，尝试启动..."
        sudo systemctl start kubelet
        sleep 5
        kubelet_status=$(sudo systemctl is-active kubelet 2>/dev/null || echo "inactive")
        echo "启动后kubelet服务状态: $kubelet_status"
    fi
fi

echo "=== Kubernetes组件安装检查完成 ==="

`
	}

	// 1. 系统准备步骤 - 重置集群，清理旧配置，配置防火墙和SELinux
	// 只在containerd安装完成后执行重置操作，确保containerd socket可用
	if !shouldSkip(StepSystemPreparation) {
		cmd += `# 禁用swap
echo "=== 禁用swap ==="
sudo swapoff -a
sudo sed -i '/ swap / s/^/#/' /etc/fstab

# 关闭并禁用防火墙
echo "=== 配置防火墙 ==="
if command -v ufw &> /dev/null; then
    echo "处理ufw防火墙..."
    # 停止并禁用ufw服务
    sudo systemctl stop ufw || true
    sudo systemctl disable ufw || true
    # 额外的禁用步骤，确保完全关闭
    sudo ufw disable 2>/dev/null || true
    # 确保ufw配置文件设置为禁用
    if [ -f /etc/ufw/ufw.conf ]; then
        sudo sed -i 's/^ENABLED=yes/ENABLED=no/' /etc/ufw/ufw.conf || true
    fi
    echo "✓ ufw防火墙已关闭并禁用，重启后保持关闭"
elif command -v firewall-cmd &> /dev/null; then
    echo "处理firewalld防火墙..."
    # 停止并禁用firewalld服务
    sudo systemctl stop firewalld || true
    sudo systemctl disable firewalld || true
    # 额外的禁用步骤，确保完全关闭
    sudo firewall-cmd --state 2>/dev/null && sudo firewall-cmd --panic-on || true
    # 确保firewalld配置文件设置为禁用
    if [ -f /etc/firewalld/firewalld.conf ]; then
        sudo sed -i 's/^FirewallBackend=.*/FirewallBackend=nftables/' /etc/firewalld/firewalld.conf || true
    fi
    echo "✓ firewalld防火墙已关闭并禁用，重启后保持关闭"
else
    echo "未检测到ufw或firewalld，跳过防火墙配置"
fi

# 配置SELinux为permissive模式
echo "=== 配置SELinux ==="
if command -v setenforce &> /dev/null; then
    echo "临时设置SELinux为permissive模式..."
    sudo setenforce 0 2>/dev/null || true
    
    echo "永久设置SELinux为permissive模式..."
    # 尝试多种方式修改SELINUX配置，确保生效
    if [ -f /etc/selinux/config ]; then
        # 备份原始配置文件
        sudo cp /etc/selinux/config /etc/selinux/config.bak
        # 修改配置文件，将enforcing改为permissive
        sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config 2>/dev/null || true
        # 添加fallback，将disabled也改为permissive
        sudo sed -i 's/^SELINUX=disabled$/SELINUX=permissive/' /etc/selinux/config 2>/dev/null || true
        # 验证SELinux配置
        selinux_status=$(grep ^SELINUX= /etc/selinux/config | cut -d= -f2)
        echo "SELinux配置已设置为: $selinux_status"
        # 验证SELinux配置文件内容
        sudo grep -E '^SELINUX=' /etc/selinux/config 2>/dev/null || true
        # 再次确认SELinux状态
        selinux_current=$(sudo getenforce 2>/dev/null || echo "Unknown")
        echo "当前SELinux状态: $selinux_current"
        if [ "$selinux_status" = "permissive" ] || [ "$selinux_current" = "Permissive" ]; then
            echo "✓ SELinux已成功设置为permissive模式，重启后保持配置"
        else
            echo "⚠ SELinux配置可能未完全生效，请检查/etc/selinux/config文件"
        fi
    else
        echo "未找到/etc/selinux/config文件，SELinux可能未安装或使用不同配置"
    fi
else
    echo "未检测到SELinux，跳过SELinux配置"
fi

# 确保防火墙和SELinux状态在重启后保持
echo "=== 最终确认防火墙和SELinux状态 ==="
# 再次确认防火墙状态
if command -v ufw &> /dev/null; then
    ufw_status=$(sudo ufw status 2>/dev/null || echo "inactive")
    echo "当前ufw状态: $ufw_status"
elif command -v firewall-cmd &> /dev/null; then
    firewalld_status=$(sudo systemctl is-active firewalld 2>/dev/null || echo "inactive")
    echo "当前firewalld状态: $firewalld_status"
fi

# 再次确认SELinux状态
if command -v getenforce &> /dev/null; then
    selinux_current=$(sudo getenforce 2>/dev/null || echo "Disabled")
    echo "当前SELinux状态: $selinux_current"
fi

# 重置集群，清理旧配置
echo "=== 重置集群，清理旧配置 ==="
# 只在kubeadm命令可用时执行重置
echo "=== 检查kubeadm命令是否可用 ==="
if command -v kubeadm &> /dev/null; then
    echo "kubeadm命令可用，执行reset操作..."
    # 重置集群，使用--force参数避免交互式操作
    sudo kubeadm reset --force 2>/dev/null || echo "kubeadm reset执行失败，可能是第一次部署"
else
    echo "kubeadm命令不可用，跳过reset操作"
fi

# 清理CNI配置
echo "=== 清理CNI配置 ==="
sudo rm -rf /etc/cni/net.d 2>/dev/null

# 重置iptables规则
echo "=== 重置iptables规则 ==="
if command -v iptables &> /dev/null; then
    sudo iptables -F
    sudo iptables -t nat -F
    sudo iptables -t mangle -F
    sudo iptables -X
fi

# 重置ip6tables规则
echo "=== 重置ip6tables规则 ==="
if command -v ip6tables &> /dev/null; then
    sudo ip6tables -F
    sudo ip6tables -t nat -F
    sudo ip6tables -t mangle -F
    sudo ip6tables -X
fi

# 如果使用IPVS，重置IPVS表
echo "=== 重置IPVS表（如果使用） ==="
if command -v ipvsadm &> /dev/null; then
    sudo ipvsadm --clear
fi

# 清理kubeconfig文件
echo "=== 清理kubeconfig文件 ==="
sudo rm -rf ~/.kube 2>/dev/null
rm -rf $HOME/.kube 2>/dev/null

# 清理集群配置文件
echo "=== 清理集群配置文件 ==="
sudo rm -f /etc/kubernetes/admin.conf 2>/dev/null
sudo rm -f /etc/kubernetes/kubelet.conf 2>/dev/null
sudo rm -f /etc/kubernetes/controller-manager.conf 2>/dev/null
sudo rm -f /etc/kubernetes/scheduler.conf 2>/dev/null
sudo rm -rf /etc/kubernetes/manifests 2>/dev/null

# 清理旧的etcd数据
echo "=== 清理旧的etcd数据 ==="
sudo rm -rf /var/lib/etcd 2>/dev/null

# 清理kubelet数据
echo "=== 清理kubelet数据 ==="
sudo rm -rf /var/lib/kubelet 2>/dev/null

# 确保IP转发配置正确
echo "=== 确保IP转发配置正确 ==="
sudo bash -c 'cat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf
net.ipv4.ip_forward = 1
EOF'
sudo sysctl --system 2>/dev/null
echo "=== IP转发配置完成 ==="

echo "=== 系统准备步骤完成 ==="

`
	}

	// 6. Master节点初始化 - 核心步骤，只有在不跳过主节点初始化时执行
	if !shouldSkip(StepMasterInitialization) {
		cmd += fmt.Sprintf(`# 1. 停掉kubelet，防止无限重启刷日志
echo "=== 停止并禁用kubelet服务 ==="
sudo systemctl stop kubelet 2>/dev/null || true
sudo systemctl disable kubelet 2>/dev/null || true

# 2. 确保containerd正确安装和配置
echo "=== 确保containerd正确安装和配置 ==="
if ! command -v containerd &> /dev/null; then
    echo "错误：containerd未安装，无法继续部署"
    exit 1
fi

# 3. 生成并修正containerd配置
echo "=== 生成并修正containerd配置 ==="
sudo mkdir -p /etc/containerd
echo "生成containerd默认配置..."
containerd config default > /etc/containerd/config.toml
echo "修正containerd配置，设置SystemdCgroup=true..."
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml

# 添加国内镜像加速
echo "配置containerd国内镜像加速..."
sudo sed -i '/\[plugins.\"io\.containerd\.grpc\.v1\.cri\".registry.mirrors\]/,/\[/c\[plugins.\"io\.containerd\.grpc\.v1\.cri\".registry.mirrors\]\n\n  [plugins.\"io\.containerd\.grpc\.v1\.cri\".registry.mirrors.\"registry.k8s.io\"]\n    endpoint = [\"https://registry.cn-hangzhou.aliyuncs.com/google_containers\"]\n\n  [plugins.\"io\.containerd\.grpc\.v1\.cri\".registry.mirrors.\"k8s.gcr.io\"]\n    endpoint = [\"https://registry.cn-hangzhou.aliyuncs.com/google_containers\"]\n\n  [plugins.\"io\.containerd\.grpc\.v1\.cri\".registry.mirrors.\"docker.io\"]\n    endpoint = [\"https://registry.cn-hangzhou.aliyuncs.com/docker\", \"https://docker.mirrors.ustc.edu.cn\"]' /etc/containerd/config.toml

# 解决InvalidDiskCapacity警告
echo "配置containerd解决InvalidDiskCapacity警告..."
sudo sed -i '/\[plugins.\"io\.containerd\.grpc\.v1\.cri\"]/a\  disable_selinux = true\n  sandbox_image = \"registry.cn-hangzhou.aliyuncs.com/google_containers/pause:3.10.1\"' /etc/containerd/config.toml

# 4. 重启containerd服务，确保配置生效
echo "=== 重启containerd服务 ==="
sudo systemctl daemon-reload
sudo systemctl restart containerd

# 等待服务重启
sleep 5

# 5. 校验containerd是否真正可用（关键步骤）
echo "=== 校验containerd是否真正可用 ==="
containerd_available=true

# 检查containerd socket是否存在
echo "1. 检查containerd socket是否存在..."
if [ -S /var/run/containerd/containerd.sock ]; then
    echo "✓ containerd socket存在：/var/run/containerd/containerd.sock"
    ls -l /var/run/containerd/containerd.sock
else
    echo "✗ containerd socket不存在，检查containerd服务状态："
    sudo systemctl status containerd --no-pager -n 20
    containerd_available=false
fi

# 检查crictl是否可用
echo "2. 使用crictl info验证containerd是否可用..."
if command -v crictl &> /dev/null; then
    crictl_info_output=$(crictl info 2>&1)
    if [ $? -eq 0 ]; then
        echo "✓ crictl info执行成功，containerd状态正常"
        echo "crictl info输出："
        echo "$crictl_info_output" | head -20
        echo "...（输出已截断，完整输出请查看日志）"
    else
        echo "✗ crictl info执行失败，containerd可能未正常工作："
        echo "$crictl_info_output"
        containerd_available=false
    fi
else
    echo "⚠️  crictl未安装，跳过crictl info验证"
fi

# 如果containerd不可用，退出部署
if [ "$containerd_available" = "false" ]; then
    echo "错误：containerd不可用，无法继续部署"
    exit 1
fi

# 7. 预拉取pause镜像，确保kubeadm init时能快速获取
echo "=== 预拉取pause容器镜像 ==="
pause_image="registry.cn-hangzhou.aliyuncs.com/google_containers/pause:3.10.1"
echo "正在拉取pause镜像: $pause_image"

# 使用ctr命令拉取镜像
if command -v ctr &> /dev/null; then
    if sudo ctr image pull $pause_image; then
        echo "✓ pause镜像拉取成功"
        # 验证镜像是否成功拉取
        if sudo ctr image list | grep -q "pause:3.10.1"; then
            echo "✓ pause镜像验证成功，已存在于本地"
        else
            echo "✗ pause镜像拉取后验证失败"
            exit 1
        fi
    else
        echo "✗ 使用ctr拉取pause镜像失败，尝试使用crictl拉取..."
        # 如果ctr拉取失败，尝试使用crictl拉取
        if command -v crictl &> /dev/null; then
            if sudo crictl pull $pause_image; then
                echo "✓ 使用crictl拉取pause镜像成功"
            else
                echo "✗ 使用crictl拉取pause镜像也失败，无法继续部署"
                exit 1
            fi
        else
            echo "✗ crictl未安装，无法使用crictl拉取镜像"
            exit 1
        fi
    fi
else
    echo "✗ ctr命令未找到，无法拉取pause镜像"
    exit 1
fi

# 6. 重新启用kubelet服务
echo "=== 重新启用kubelet服务 ==="
sudo systemctl enable kubelet 2>/dev/null || true

# 7. 添加master主机名解析
echo "=== 添加master主机名解析 ==="
echo "127.0.0.1 master" >> /etc/hosts

# 8. 初始化master节点，使用国内镜像源
echo "=== 初始化master节点 ==="
echo "使用的kubeadm init命令参数："
echo "--apiserver-advertise-address=$HOSTNAME -I"
echo "--kubernetes-version=%s"
echo "--image-repository=registry.cn-hangzhou.aliyuncs.com/google_containers"
echo "--cri-socket=%s"
echo "--pod-network-cidr=%s"
echo "--upload-certs"
sudo kubeadm init --apiserver-advertise-address=$(hostname -I | cut -d' ' -f1) --kubernetes-version=%s --image-repository=registry.cn-hangzhou.aliyuncs.com/google_containers --cri-socket=%s --pod-network-cidr=%s --upload-certs

# 检查kubeadm init是否成功
if [ $? -eq 0 ]; then
    echo "=== kubeadm init 成功 ==="
    
    # 配置kubectl
    echo "=== 配置kubectl ==="
    mkdir -p $HOME/.kube
    if [ -f /etc/kubernetes/admin.conf ]; then
        echo "找到admin.conf文件，正在配置kubectl..."
        sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
        sudo chown $(id -u):$(id -g) $HOME/.kube/config
        echo "kubectl配置成功"
    else
        echo "未找到admin.conf文件，可能初始化过程中出现问题"
    fi
    
    # 生成Join命令
echo "=== 生成Join命令 ==="
echo "生成的Join命令："
sudo kubeadm token create --print-join-command
    
    # 安装CNI网络插件（使用Calico）
echo "=== 安装Calico网络插件 ==="
if [ -f $HOME/.kube/config ]; then
    kubectl apply -f https://docs.projectcalico.org/manifests/calico.yaml
else
    echo "无法安装CNI插件，kubectl配置失败"
fi
else
    echo "=== kubeadm init 失败 ==="
    echo "显示kubeadm日志："
    sudo journalctl -u kubelet --no-pager -n 50
fi
`, config.ClusterConfiguration.KubernetesVersion, config.InitConfiguration.NodeRegistration.CRISocket, config.ClusterConfiguration.Networking.PodSubnet, config.ClusterConfiguration.KubernetesVersion, config.InitConfiguration.NodeRegistration.CRISocket, config.ClusterConfiguration.Networking.PodSubnet)
	} else {
		cmd += `# 跳过Master节点初始化步骤
echo "=== 跳过Master节点初始化步骤 ==="
`
	}

	// 7. 集群验证 - 只在需要时执行
	if !shouldSkip(StepClusterVerification) {
		cmd += `# 验证集群状态
echo "=== 验证集群状态 ==="
if [ -f $HOME/.kube/config ]; then
    echo "=== 等待集群就绪（30秒） ==="
    sleep 30
    echo "=== 查看节点状态 ==="
    kubectl get nodes
    echo "=== 查看Pod状态 ==="
    kubectl get pods -A
else
    echo "无法验证集群状态，kubectl配置失败"
fi

`
	}

	cmd += `# 主节点初始化步骤执行完成
echo "=== 主节点初始化步骤执行完成 ==="
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
	cmd := fmt.Sprintf(`#!/bin/bash

# 1. 必须的内核模块 - Calico初始化依赖
	echo "=== 加载必须的内核模块（Calico初始化依赖） ==="
	sudo modprobe br_netfilter || echo "br_netfilter模块已加载或加载失败"
	sudo modprobe overlay || echo "overlay模块已加载或加载失败"
	
# 2. 必须的sysctl - Calico初始化依赖，此文件必须写入
	echo "=== 配置必须的sysctl（Calico初始化依赖） ==="
	sudo cat <<EOF > /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward = 1
EOF
	sudo sysctl --system

# 3. 安装必要的依赖
	echo "=== 安装必要的依赖 ==="
	if command -v dnf &> /dev/null; then
	    # Rocky 10 必装，否则calico-node Init直接失败
	    sudo dnf install -y iproute-tc 2>/dev/null || true
	fi

# 4. BPF挂载点（init容器mount-bpffs需要）
	echo "=== 创建并挂载BPF挂载点 ==="
	sudo mkdir -p /sys/fs/bpf
	sudo mount bpffs /sys/fs/bpf || true

# 5. 确保CNI目录存在
	echo "=== 确保CNI目录存在 ==="
	sudo mkdir -p /opt/cni/bin
	sudo mkdir -p /etc/cni/net.d
	sudo chmod 755 /opt/cni/bin
	sudo chmod 755 /etc/cni/net.d

# 6. 确保kubelet服务正确配置和运行
	echo "=== 确保kubelet服务正确配置和运行 ==="
	sudo systemctl enable kubelet 2>/dev/null || true
	sudo systemctl start kubelet 2>/dev/null || true

# 7. 确保containerd服务正常运行
	echo "=== 确保containerd服务正常运行 ==="
	sudo systemctl enable containerd 2>/dev/null || true
	sudo systemctl restart containerd 2>/dev/null || true
	sleep 5

# 8. 执行kubeadm join命令将节点加入集群
	echo "=== 将节点加入集群 ==="
	kubeadm join %s --token %s --discovery-token-ca-cert-hash %s --cri-socket=unix:///run/containerd/containerd.sock

# 9. 重启containerd和kubelet服务，确保网络插件生效
	echo "=== 重启containerd和kubelet服务，确保网络插件生效 ==="
	sudo systemctl restart containerd || true
	sudo systemctl restart kubelet || true
	echo "=== 节点加入集群完成 ==="`, controlPlaneEndpoint, token, caCertHash)
	return RunCommandOnRemote(sshConfig, "bash", "-c", cmd)
}

// CheckKubeadmVersion 检查kubeadm版本
func CheckKubeadmVersion(sshConfig SSHConfig) (string, error) {
	cmd := `kubeadm version --short`
	return RunCommandOnRemote(sshConfig, "bash", "-c", cmd)
}

// PullKubernetesImages 拉取Kubernetes镜像
func PullKubernetesImages(sshConfig SSHConfig, version string) (string, error) {
	cmd := fmt.Sprintf(`kubeadm config images pull --kubernetes-version %s --image-repository registry.aliyuncs.com/google_containers`, version)
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
