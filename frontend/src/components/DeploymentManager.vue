<template>
  <div class="deployment-manager">
    <h2>部署流程管理</h2>
    
    <div class="source-switcher">
      <div class="section-header">
        <h3>部署源管理</h3>
        <div class="source-distro-selector">
          <label for="source-distro">选择发行版本:</label>
          <select id="source-distro" v-model="activeDistro" class="form-input">
            <option v-for="system in systems" :key="system" :value="system">{{ system }}</option>
          </select>
        </div>
      </div>
      <div class="source-options">
        <label v-for="source in (deploymentSources[activeDistro] || [])" :key="source.id" class="source-option">
          <input 
            type="radio" 
            v-model="selectedSources[activeDistro]" 
            :value="source.id" 
            @change="switchSource(source, activeDistro)"
          >
          <span class="source-label">{{ source.name }}</span>
          <span class="source-url">{{ source.url }}</span>
        </label>
      </div>
      <div class="source-actions-bottom">
        <button class="btn btn-primary" @click="applySource">应用当前源</button>
        <button class="btn btn-secondary" @click="applySourceToAll">应用到所有版本</button>
      </div>
    </div>
    

    
    <div class="process-list">
      <h3>部署流程列表</h3>
      <div class="system-tabs">
        <button 
          v-for="system in systems" 
          :key="system" 
          class="tab-btn" 
          :class="{ active: activeSystem === system }"
          @click="activeSystem = system"
        >
          {{ system }}
        </button>
      </div>
      
      <div class="process-steps" v-if="currentProcess">
        <div 
          v-for="(step, index) in currentProcess.steps" 
          :key="index" 
          class="process-step"
        >
          <div class="step-header">
            <div class="step-number">{{ index + 1 }}</div>
            <div class="step-info">
              <div class="step-title-row">
                <h4>{{ step.name }}</h4>
                <button class="btn btn-small" @click="editScript(index)">编辑脚本</button>
              </div>
              <p class="step-description">{{ step.description }}</p>
            </div>
          </div>
          <div class="step-content">
            <div class="step-script">
              <div class="script-header">
                <h5>使用的脚本</h5>
              </div>
              <pre>{{ step.script }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
    
    <!-- 脚本编辑对话框 -->
    <div v-if="showEditScriptDialog" class="dialog-overlay" @click="closeEditScriptDialog">
      <div class="dialog-content dialog-large" @click.stop>
        <div class="dialog-header">
          <h4>编辑脚本 - {{ currentEditingStep?.name }}</h4>
          <button class="dialog-close" @click="closeEditScriptDialog">&times;</button>
        </div>
        <div class="dialog-body">
          <div class="form-group">
            <label for="script-content">脚本内容</label>
            <textarea 
              id="script-content" 
              v-model="editingScript" 
              placeholder="请输入脚本内容..."
              class="form-textarea"
              rows="20"
            ></textarea>
          </div>
        </div>
        <div class="dialog-footer">
          <button class="btn" @click="closeEditScriptDialog">取消</button>
          <button class="btn btn-primary" @click="saveScript">保存脚本</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import axios from 'axios'

// localStorage辅助函数
const loadFromLocalStorage = (key, defaultValue) => {
  try {
    const stored = localStorage.getItem(key)
    return stored ? JSON.parse(stored) : defaultValue
  } catch (error) {
    // 静默处理localStorage错误
    return defaultValue
  }
}

const saveToLocalStorage = (key, value) => {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch (error) {
    // 静默处理localStorage错误
  }
}

// API基础URL
const API_BASE_URL = 'http://localhost:8080'

// 部署源管理相关状态
const defaultDeploymentSources = {
  centos: [
    {
      id: 'centos-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'centos-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ],
  ubuntu: [
    {
      id: 'ubuntu-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'ubuntu-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ],
  debian: [
    {
      id: 'debian-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'debian-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ],
  rocky: [
    {
      id: 'rocky-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'rocky-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ],
  almalinux: [
    {
      id: 'almalinux-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'almalinux-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ]
}

const deploymentSources = ref(loadFromLocalStorage('deploymentSources', defaultDeploymentSources))

// 按发行版本存储选中的源
const defaultSelectedSources = {
  centos: 'centos-1',
  ubuntu: 'ubuntu-1',
  debian: 'debian-1',
  rocky: 'rocky-1',
  almalinux: 'almalinux-1'
}

const selectedSources = ref(loadFromLocalStorage('selectedSources', defaultSelectedSources))

// 脚本编辑相关状态
const showEditScriptDialog = ref(false)
const currentEditingStepIndex = ref(-1)
const currentEditingStep = ref(null)
const editingScript = ref('')

// 支持的系统类型
const systems = ref(['centos', 'ubuntu', 'debian', 'rocky', 'almalinux'])
const activeDistro = ref('centos')

// 确保activeSystem的初始值是有效的
const activeSystem = ref(systems.value[0] || 'centos')

// 支持的Kubernetes版本
const kubernetesVersions = ref(['v1.28', 'v1.29', 'v1.30'])
const selectedKubernetesVersion = ref(loadFromLocalStorage('selectedKubernetesVersion', 'v1.28'))

// 部署流程默认数据
const defaultProcessData = {
  centos: {
    name: 'CentOS/RHEL 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等',
        script: '# 禁用swap\nsudo swapoff -a\nsudo sed -i \'/ swap / s/^#//\' /etc/fstab\n\n# 安装并启动时间同步服务\nsudo yum install -y chrony\nsudo systemctl enable --now chronyd\nsudo timedatectl set-timezone Asia/Shanghai\n\n# 关闭防火墙\nsudo systemctl stop firewalld || true\nsudo systemctl disable firewalld || true\n\n# 禁用SELinux\nsudo setenforce 0\nsudo sed -i \'s/^SELINUX=enforcing$/SELINUX=permissive/\' /etc/selinux/config\n\n# 加载K8s所需内核模块\ncat <<EOF > /etc/modules-load.d/k8s.conf\noverlay\nbr_netfilter\nEOF\nsudo modprobe overlay\nsudo modprobe br_netfilter\n\n# 设置内核参数\n# 使用EOF方式写入IP转发配置文件\ncat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf\nnet.ipv4.ip_forward = 1\nEOF\n\n# 设置其他Kubernetes所需内核参数\ncat <<EOF > /etc/sysctl.d/k8s.conf\nnet.bridge.bridge-nf-call-iptables = 1\nnet.bridge.bridge-nf-call-ip6tables = 1\nEOF\n\n# 应用内核参数\nsudo sysctl --system'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时',
        script: '# 安装containerd\nsudo yum install -y containerd.io'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务',
        script: '# 配置containerd\nsudo mkdir -p /etc/containerd\nsudo containerd config default > /etc/containerd/config.toml\nsudo sed -i \'s/SystemdCgroup = false/SystemdCgroup = true/g\' /etc/containerd/config.toml\n\n# 启动前先停止可能运行的containerd进程\necho "停止可能运行的containerd进程..."\nsudo pkill -f containerd || true\nsleep 2\n\n# 清理旧的containerd socket和状态文件\necho "清理旧的containerd socket和状态文件..."\nsudo rm -f /run/containerd/containerd.sock\nsudo rm -rf /var/run/containerd\nsudo mkdir -p /var/run/containerd\n\n# 启动并启用containerd服务\necho "启动containerd服务..."\nsudo systemctl daemon-reload\nsudo systemctl restart containerd\nsudo systemctl enable containerd\n\n# 等待containerd启动，增加等待时间\necho "等待containerd启动..."\nsleep 10\n\n# 检查containerd状态\necho "=== 检查containerd状态 ==="\nif command -v systemctl &> /dev/null; then\n    systemctl_status=$(sudo systemctl is-active containerd)\n    echo "containerd服务状态: $systemctl_status"\n    \n    # 显示containerd服务详细状态\n    echo "containerd服务详细状态:"\n    sudo systemctl status containerd --no-pager\nfi\n\n# 检查containerd socket是否存在\necho "=== 检查containerd socket ==="\ncri_socket="/run/containerd/containerd.sock"\nif [ -S "$cri_socket" ]; then\n    echo "CRI socket $cri_socket 存在"\n    # 测试socket连接\n    echo "测试containerd连接..."\n    if command -v ctr &> /dev/null; then\n        ctr version\n    fi\nelse\n    echo "警告: CRI socket $cri_socket 不存在，检查containerd日志..."\n    sudo journalctl -u containerd --no-pager -n 30\n    \n    # 尝试手动启动containerd\n    echo "尝试手动启动containerd..."\n    containerd --version\n    containerd &\n    sleep 5\n    \n    # 再次检查socket\n    if [ -S "$cri_socket" ]; then\n        echo "手动启动成功，CRI socket $cri_socket 现在存在"\n    else\n        echo "手动启动失败，CRI socket $cri_socket 仍然不存在"\n    fi\nfi'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库',
        script: '# 添加Kubernetes仓库\n# 清理旧的Kubernetes仓库配置\nsudo rm -f /etc/yum.repos.d/kubernetes.repo\nsudo rm -f /etc/yum.repos.d/packages.cloud.google.com_yum_repos_kubernetes-el7-x86_64.repo\n\n# 添加新的Kubernetes仓库\nsudo cat <<EOF > /etc/yum.repos.d/kubernetes.repo\n[kubernetes]\nname=Kubernetes\nbaseurl=https://pkgs.k8s.io/core:/stable:/v1.28/rpm/\nenabled=1\ngpgcheck=1\ngpgkey=https://pkgs.k8s.io/core:/stable:/v1.28/rpm/repodata/repomd.xml.key\nexclude=kubelet kubeadm kubectl\nEOF\n\n# 更新仓库缓存\nsudo yum makecache'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl',
        script: '# 安装Kubernetes组件\nsudo yum install -y kubelet${version} kubeadm${version} kubectl${version} --disableexcludes=kubernetes\n\n# 启动kubelet\nsudo systemctl enable --now kubelet'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点',
        script: '# 初始化Kubernetes集群
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

# 执行kubeadm init
echo "=== 执行kubeadm init ==="\nsudo kubeadm init --pod-network-cidr=10.244.0.0/16 --upload-certs\n\n# 检查kubeadm init是否成功\nif [ $? -eq 0 ]; then\n    echo "=== kubeadm init 成功 ==="\n    \n    # 配置kubectl\necho "=== 配置kubectl ==="\nmkdir -p $HOME/.kube\n    \n    # 检查admin.conf是否存在\n    if [ -f /etc/kubernetes/admin.conf ]; then\n        echo "✓ 找到admin.conf文件，正在配置kubectl..."\n        sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config\n        sudo chown $(id -u):$(id -g) $HOME/.kube/config\n        echo "✓ kubectl配置成功"\n    else\n        echo "✗ 未找到admin.conf文件，可能初始化过程中出现问题"\n    fi\n    \n    # 安装CNI网络插件（使用Flannel）\n    if [ -f $HOME/.kube/config ]; then\n        echo "=== 安装Flannel网络插件 ==="\n        kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml\n    else\n        echo "✗ 无法安装CNI插件，kubectl配置失败"\n    fi\nelse\n    echo "✗ kubeadm init 失败"\n    # 显示更多错误信息\n    echo "=== 显示kubeadm日志 ==="\n    sudo journalctl -u kubelet --no-pager -n 50\nfi'
      }
    ]
  },
  ubuntu: {
    name: 'Ubuntu 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等',
        script: '# 禁用swap\nsudo swapoff -a\nsudo sed -i \'/ swap / s/^#//\' /etc/fstab\n\n# 安装并启动时间同步服务\nsudo apt update\nsudo apt install -y chrony\nsudo systemctl enable --now chronyd\nsudo timedatectl set-timezone Asia/Shanghai\n\n# 关闭防火墙\nsudo systemctl stop ufw || true\nsudo systemctl disable ufw || true\n\n# 加载K8s所需内核模块\ncat <<EOF > /etc/modules-load.d/k8s.conf\noverlay\nbr_netfilter\nEOF\nsudo modprobe overlay\nsudo modprobe br_netfilter\n\n# 设置内核参数\n# 使用EOF方式写入IP转发配置文件\ncat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf\nnet.ipv4.ip_forward = 1\nEOF\n\n# 设置其他Kubernetes所需内核参数\ncat <<EOF > /etc/sysctl.d/k8s.conf\nnet.bridge.bridge-nf-call-iptables = 1\nnet.bridge.bridge-nf-call-ip6tables = 1\nEOF\n\n# 应用内核参数\nsudo sysctl --system'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时',
        script: '# 安装containerd\nsudo apt update\nsudo apt install -y containerd.io'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务',
        script: '# 配置containerd\nsudo mkdir -p /etc/containerd\nsudo containerd config default > /etc/containerd/config.toml\nsudo sed -i \'s/SystemdCgroup = false/SystemdCgroup = true/g\' /etc/containerd/config.toml\n\n# 启动前先停止可能运行的containerd进程\necho "停止可能运行的containerd进程..."\nsudo pkill -f containerd || true\nsleep 2\n\n# 清理旧的containerd socket和状态文件\necho "清理旧的containerd socket和状态文件..."\nsudo rm -f /run/containerd/containerd.sock\nsudo rm -rf /var/run/containerd\nsudo mkdir -p /var/run/containerd\n\n# 启动并启用containerd服务\necho "启动containerd服务..."\nsudo systemctl daemon-reload\nsudo systemctl restart containerd\nsudo systemctl enable containerd\n\n# 等待containerd启动，增加等待时间\necho "等待containerd启动..."\nsleep 10\n\n# 检查containerd状态\necho "=== 检查containerd状态 ==="\nif command -v systemctl &> /dev/null; then\n    systemctl_status=$(sudo systemctl is-active containerd)\n    echo "containerd服务状态: $systemctl_status"\n    \n    # 显示containerd服务详细状态\n    echo "containerd服务详细状态:"\n    sudo systemctl status containerd --no-pager\nfi\n\n# 检查containerd socket是否存在\necho "=== 检查containerd socket ==="\ncri_socket="/run/containerd/containerd.sock"\nif [ -S "$cri_socket" ]; then\n    echo "CRI socket $cri_socket 存在"\n    # 测试socket连接\n    echo "测试containerd连接..."\n    if command -v ctr &> /dev/null; then\n        ctr version\n    fi\nelse\n    echo "警告: CRI socket $cri_socket 不存在，检查containerd日志..."\n    sudo journalctl -u containerd --no-pager -n 30\n    \n    # 尝试手动启动containerd\n    echo "尝试手动启动containerd..."\n    containerd --version\n    containerd &\n    sleep 5\n    \n    # 再次检查socket\n    if [ -S "$cri_socket" ]; then\n        echo "手动启动成功，CRI socket $cri_socket 现在存在"\n    else\n        echo "手动启动失败，CRI socket $cri_socket 仍然不存在"\n    fi\nfi'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库',
        script: '# 添加Kubernetes仓库\nsudo apt update\nsudo apt install -y apt-transport-https ca-certificates curl gpg\n\n# 创建keyring目录\nsudo mkdir -p -m 755 /etc/apt/keyrings\n\n# 下载并安装GPG密钥\ncurl -fsSL -L https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg\n\n# 添加Kubernetes repo\necho "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list\n\n# 更新仓库缓存\nsudo apt update'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl',
        script: '# 安装Kubernetes组件\nsudo apt install -y kubelet${version} kubeadm${version} kubectl${version}\n\n# 启动kubelet\nsudo systemctl enable --now kubelet'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点',
        script: '# 初始化Kubernetes集群\n# 执行kubeadm init\necho "=== 执行kubeadm init ==="\nsudo kubeadm init --pod-network-cidr=10.244.0.0/16 --upload-certs\n\n# 检查kubeadm init是否成功\nif [ $? -eq 0 ]; then\n    echo "=== kubeadm init 成功 ==="\n    \n    # 配置kubectl\necho "=== 配置kubectl ==="\nmkdir -p $HOME/.kube\n    \n    # 检查admin.conf是否存在\n    if [ -f /etc/kubernetes/admin.conf ]; then\n        echo "✓ 找到admin.conf文件，正在配置kubectl..."\n        sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config\n        sudo chown $(id -u):$(id -g) $HOME/.kube/config\n        echo "✓ kubectl配置成功"\n    else\n        echo "✗ 未找到admin.conf文件，可能初始化过程中出现问题"\n    fi\n    \n    # 安装CNI网络插件（使用Flannel）\n    if [ -f $HOME/.kube/config ]; then\n        echo "=== 安装Flannel网络插件 ==="\n        kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml\n    else\n        echo "✗ 无法安装CNI插件，kubectl配置失败"\n    fi\nelse\n    echo "✗ kubeadm init 失败"\n    # 显示更多错误信息\n    echo "=== 显示kubeadm日志 ==="\n    sudo journalctl -u kubelet --no-pager -n 50\nfi'
      }
    ]
  },
  debian: {
    name: 'Debian 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等',
        script: '# 禁用swap\nsudo swapoff -a\nsudo sed -i \'/ swap / s/^#//\' /etc/fstab\n\n# 安装并启动时间同步服务\nsudo apt update\nsudo apt install -y chrony\nsudo systemctl enable --now chronyd\nsudo timedatectl set-timezone Asia/Shanghai\n\n# 关闭防火墙\nsudo systemctl stop ufw || true\nsudo systemctl disable ufw || true\n\n# 加载K8s所需内核模块\ncat <<EOF > /etc/modules-load.d/k8s.conf\noverlay\nbr_netfilter\nEOF\nsudo modprobe overlay\nsudo modprobe br_netfilter\n\n# 设置内核参数\n# 使用EOF方式写入IP转发配置文件\ncat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf\nnet.ipv4.ip_forward = 1\nEOF\n\n# 设置其他Kubernetes所需内核参数\ncat <<EOF > /etc/sysctl.d/k8s.conf\nnet.bridge.bridge-nf-call-iptables = 1\nnet.bridge.bridge-nf-call-ip6tables = 1\nEOF\n\n# 应用内核参数\nsudo sysctl --system'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时',
        script: '# 安装containerd\nsudo apt update\nsudo apt install -y containerd.io'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务',
        script: '# 配置containerd\nsudo mkdir -p /etc/containerd\nsudo containerd config default > /etc/containerd/config.toml\nsudo sed -i \'s/SystemdCgroup = false/SystemdCgroup = true/g\' /etc/containerd/config.toml\n\n# 启动前先停止可能运行的containerd进程\necho "停止可能运行的containerd进程..."\nsudo pkill -f containerd || true\nsleep 2\n\n# 清理旧的containerd socket和状态文件\necho "清理旧的containerd socket和状态文件..."\nsudo rm -f /run/containerd/containerd.sock\nsudo rm -rf /var/run/containerd\nsudo mkdir -p /var/run/containerd\n\n# 启动并启用containerd服务\necho "启动containerd服务..."\nsudo systemctl daemon-reload\nsudo systemctl restart containerd\nsudo systemctl enable containerd\n\n# 等待containerd启动，增加等待时间\necho "等待containerd启动..."\nsleep 10\n\n# 检查containerd状态\necho "=== 检查containerd状态 ==="\nif command -v systemctl &> /dev/null; then\n    systemctl_status=$(sudo systemctl is-active containerd)\n    echo "containerd服务状态: $systemctl_status"\n    \n    # 显示containerd服务详细状态\n    echo "containerd服务详细状态:"\n    sudo systemctl status containerd --no-pager\nfi\n\n# 检查containerd socket是否存在\necho "=== 检查containerd socket ==="\ncri_socket="/run/containerd/containerd.sock"\nif [ -S "$cri_socket" ]; then\n    echo "CRI socket $cri_socket 存在"\n    # 测试socket连接\n    echo "测试containerd连接..."\n    if command -v ctr &> /dev/null; then\n        ctr version\n    fi\nelse\n    echo "警告: CRI socket $cri_socket 不存在，检查containerd日志..."\n    sudo journalctl -u containerd --no-pager -n 30\n    \n    # 尝试手动启动containerd\n    echo "尝试手动启动containerd..."\n    containerd --version\n    containerd &\n    sleep 5\n    \n    # 再次检查socket\n    if [ -S "$cri_socket" ]; then\n        echo "手动启动成功，CRI socket $cri_socket 现在存在"\n    else\n        echo "手动启动失败，CRI socket $cri_socket 仍然不存在"\n    fi\nfi'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库',
        script: '# 添加Kubernetes仓库\nsudo apt update\nsudo apt install -y apt-transport-https ca-certificates curl gpg\n\n# 创建keyring目录\nsudo mkdir -p -m 755 /etc/apt/keyrings\n\n# 下载并安装GPG密钥\ncurl -fsSL -L https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg\n\n# 添加Kubernetes repo\necho "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list\n\n# 更新仓库缓存\nsudo apt update'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl',
        script: '# 安装Kubernetes组件\nsudo apt install -y kubelet${version} kubeadm${version} kubectl${version}\n\n# 启动kubelet\nsudo systemctl enable --now kubelet'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点',
        script: '# 初始化Kubernetes集群\n# 执行kubeadm init\necho "=== 执行kubeadm init ==="\nsudo kubeadm init --pod-network-cidr=10.244.0.0/16 --upload-certs\n\n# 检查kubeadm init是否成功\nif [ $? -eq 0 ]; then\n    echo "=== kubeadm init 成功 ==="\n    \n    # 配置kubectl\necho "=== 配置kubectl ==="\nmkdir -p $HOME/.kube\n    \n    # 检查admin.conf是否存在\n    if [ -f /etc/kubernetes/admin.conf ]; then\n        echo "✓ 找到admin.conf文件，正在配置kubectl..."\n        sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config\n        sudo chown $(id -u):$(id -g) $HOME/.kube/config\n        echo "✓ kubectl配置成功"\n    else\n        echo "✗ 未找到admin.conf文件，可能初始化过程中出现问题"\n    fi\n    \n    # 安装CNI网络插件（使用Flannel）\n    if [ -f $HOME/.kube/config ]; then\n        echo "=== 安装Flannel网络插件 ==="\n        kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml\n    else\n        echo "✗ 无法安装CNI插件，kubectl配置失败"\n    fi\nelse\n    echo "✗ kubeadm init 失败"\n    # 显示更多错误信息\n    echo "=== 显示kubeadm日志 ==="\n    sudo journalctl -u kubelet --no-pager -n 50\nfi'
      }
    ]
  },
  rocky: {
    name: 'Rocky Linux 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等',
        script: '# 禁用swap\nsudo swapoff -a\nsudo sed -i \'/ swap / s/^#//\' /etc/fstab\n\n# 安装并启动时间同步服务\nsudo dnf install -y chrony\nsudo systemctl enable --now chronyd\nsudo timedatectl set-timezone Asia/Shanghai\n\n# 关闭防火墙\nsudo systemctl stop firewalld || true\nsudo systemctl disable firewalld || true\n\n# 禁用SELinux\nsudo setenforce 0\nsudo sed -i \'s/^SELINUX=enforcing$/SELINUX=permissive/\' /etc/selinux/config\n\n# 加载K8s所需内核模块\ncat <<EOF > /etc/modules-load.d/k8s.conf\noverlay\nbr_netfilter\nEOF\nsudo modprobe overlay\nsudo modprobe br_netfilter\n\n# 设置内核参数\n# 使用EOF方式写入IP转发配置文件\ncat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf\nnet.ipv4.ip_forward = 1\nEOF\n\n# 设置其他Kubernetes所需内核参数\ncat <<EOF > /etc/sysctl.d/k8s.conf\nnet.bridge.bridge-nf-call-iptables = 1\nnet.bridge.bridge-nf-call-ip6tables = 1\nEOF\n\n# 应用内核参数\nsudo sysctl --system'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时',
        script: '# 安装containerd\nsudo dnf install -y containerd.io'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务',
        script: '# 配置containerd\nsudo mkdir -p /etc/containerd\nsudo containerd config default > /etc/containerd/config.toml\nsudo sed -i \'s/SystemdCgroup = false/SystemdCgroup = true/g\' /etc/containerd/config.toml\n\n# 启动前先停止可能运行的containerd进程\necho "停止可能运行的containerd进程..."\nsudo pkill -f containerd || true\nsleep 2\n\n# 清理旧的containerd socket和状态文件\necho "清理旧的containerd socket和状态文件..."\nsudo rm -f /run/containerd/containerd.sock\nsudo rm -rf /var/run/containerd\nsudo mkdir -p /var/run/containerd\n\n# 启动并启用containerd服务\necho "启动containerd服务..."\nsudo systemctl daemon-reload\nsudo systemctl restart containerd\nsudo systemctl enable containerd\n\n# 等待containerd启动，增加等待时间\necho "等待containerd启动..."\nsleep 10\n\n# 检查containerd状态\necho "=== 检查containerd状态 ==="\nif command -v systemctl &> /dev/null; then\n    systemctl_status=$(sudo systemctl is-active containerd)\n    echo "containerd服务状态: $systemctl_status"\n    \n    # 显示containerd服务详细状态\n    echo "containerd服务详细状态:"\n    sudo systemctl status containerd --no-pager\nfi\n\n# 检查containerd socket是否存在\necho "=== 检查containerd socket ==="\ncri_socket="/run/containerd/containerd.sock"\nif [ -S "$cri_socket" ]; then\n    echo "CRI socket $cri_socket 存在"\n    # 测试socket连接\n    echo "测试containerd连接..."\n    if command -v ctr &> /dev/null; then\n        ctr version\n    fi\nelse\n    echo "警告: CRI socket $cri_socket 不存在，检查containerd日志..."\n    sudo journalctl -u containerd --no-pager -n 30\n    \n    # 尝试手动启动containerd\n    echo "尝试手动启动containerd..."\n    containerd --version\n    containerd &\n    sleep 5\n    \n    # 再次检查socket\n    if [ -S "$cri_socket" ]; then\n        echo "手动启动成功，CRI socket $cri_socket 现在存在"\n    else\n        echo "手动启动失败，CRI socket $cri_socket 仍然不存在"\n    fi\nfi'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库',
        script: '# 添加Kubernetes仓库\n# 清理旧的Kubernetes仓库配置\nsudo rm -f /etc/yum.repos.d/kubernetes.repo\nsudo rm -f /etc/yum.repos.d/packages.cloud.google.com_yum_repos_kubernetes-el7-x86_64.repo\n\n# 添加新的Kubernetes仓库\nsudo cat <<EOF > /etc/yum.repos.d/kubernetes.repo\n[kubernetes]\nname=Kubernetes\nbaseurl=https://mirrors.aliyun.com/kubernetes-new/core/stable/v1.28/rpm/\nenabled=1\ngpgcheck=1\ngpgkey=https://mirrors.aliyun.com/kubernetes-new/core/stable/v1.28/rpm/repodata/repomd.xml.key\nexclude=kubelet kubeadm kubectl\nEOF\n\n# 更新仓库缓存\nsudo dnf makecache'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl',
        script: '# 安装Kubernetes组件\nsudo dnf install -y kubelet${version} kubeadm${version} kubectl${version} --disableexcludes=kubernetes\n\n# 启动kubelet\nsudo systemctl enable --now kubelet'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点',
        script: '# 初始化Kubernetes集群\n# 执行kubeadm init\necho "=== 执行kubeadm init ==="\nsudo kubeadm init --pod-network-cidr=10.244.0.0/16 --upload-certs\n\n# 检查kubeadm init是否成功\nif [ $? -eq 0 ]; then\n    echo "=== kubeadm init 成功 ==="\n    \n    # 配置kubectl\necho "=== 配置kubectl ==="\nmkdir -p $HOME/.kube\n    \n    # 检查admin.conf是否存在\n    if [ -f /etc/kubernetes/admin.conf ]; then\n        echo "✓ 找到admin.conf文件，正在配置kubectl..."\n        sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config\n        sudo chown $(id -u):$(id -g) $HOME/.kube/config\n        echo "✓ kubectl配置成功"\n    else\n        echo "✗ 未找到admin.conf文件，可能初始化过程中出现问题"\n    fi\n    \n    # 安装CNI网络插件（使用Flannel）\n    if [ -f $HOME/.kube/config ]; then\n        echo "=== 安装Flannel网络插件 ==="\n        kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml\n    else\n        echo "✗ 无法安装CNI插件，kubectl配置失败"\n    fi\nelse\n    echo "✗ kubeadm init 失败"\n    # 显示更多错误信息\n    echo "=== 显示kubeadm日志 ==="\n    sudo journalctl -u kubelet --no-pager -n 50\nfi'
      }
    ]
  },
  almalinux: {
    name: 'AlmaLinux 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等',
        script: '# 禁用swap\nsudo swapoff -a\nsudo sed -i \'/ swap / s/^#//\' /etc/fstab\n\n# 安装并启动时间同步服务\nsudo dnf install -y chrony\nsudo systemctl enable --now chronyd\nsudo timedatectl set-timezone Asia/Shanghai\n\n# 关闭防火墙\nsudo systemctl stop firewalld || true\nsudo systemctl disable firewalld || true\n\n# 禁用SELinux\nsudo setenforce 0\nsudo sed -i \'s/^SELINUX=enforcing$/SELINUX=permissive/\' /etc/selinux/config\n\n# 加载K8s所需内核模块\ncat <<EOF > /etc/modules-load.d/k8s.conf\noverlay\nbr_netfilter\nEOF\nsudo modprobe overlay\nsudo modprobe br_netfilter\n\n# 设置内核参数\n# 使用EOF方式写入IP转发配置文件\ncat <<EOF > /etc/sysctl.d/99-kubernetes-ipforward.conf\nnet.ipv4.ip_forward = 1\nEOF\n\n# 设置其他Kubernetes所需内核参数\ncat <<EOF > /etc/sysctl.d/k8s.conf\nnet.bridge.bridge-nf-call-iptables = 1\nnet.bridge.bridge-nf-call-ip6tables = 1\nEOF\n\n# 应用内核参数\nsudo sysctl --system'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时',
        script: '# 安装containerd\nsudo dnf install -y containerd.io'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务',
        script: '# 配置containerd\nsudo mkdir -p /etc/containerd\nsudo containerd config default > /etc/containerd/config.toml\nsudo sed -i \'s/SystemdCgroup = false/SystemdCgroup = true/g\' /etc/containerd/config.toml\n\n# 启动前先停止可能运行的containerd进程\necho "停止可能运行的containerd进程..."\nsudo pkill -f containerd || true\nsleep 2\n\n# 清理旧的containerd socket和状态文件\necho "清理旧的containerd socket和状态文件..."\nsudo rm -f /run/containerd/containerd.sock\nsudo rm -rf /var/run/containerd\nsudo mkdir -p /var/run/containerd\n\n# 启动并启用containerd服务\necho "启动containerd服务..."\nsudo systemctl daemon-reload\nsudo systemctl restart containerd\nsudo systemctl enable containerd\n\n# 等待containerd启动，增加等待时间\necho "等待containerd启动..."\nsleep 10\n\n# 检查containerd状态\necho "=== 检查containerd状态 ==="\nif command -v systemctl &> /dev/null; then\n    systemctl_status=$(sudo systemctl is-active containerd)\n    echo "containerd服务状态: $systemctl_status"\n    \n    # 显示containerd服务详细状态\n    echo "containerd服务详细状态:"\n    sudo systemctl status containerd --no-pager\nfi\n\n# 检查containerd socket是否存在\necho "=== 检查containerd socket ==="\ncri_socket="/run/containerd/containerd.sock"\nif [ -S "$cri_socket" ]; then\n    echo "CRI socket $cri_socket 存在"\n    # 测试socket连接\n    echo "测试containerd连接..."\n    if command -v ctr &> /dev/null; then\n        ctr version\n    fi\nelse\n    echo "警告: CRI socket $cri_socket 不存在，检查containerd日志..."\n    sudo journalctl -u containerd --no-pager -n 30\n    \n    # 尝试手动启动containerd\n    echo "尝试手动启动containerd..."\n    containerd --version\n    containerd &\n    sleep 5\n    \n    # 再次检查socket\n    if [ -S "$cri_socket" ]; then\n        echo "手动启动成功，CRI socket $cri_socket 现在存在"\n    else\n        echo "手动启动失败，CRI socket $cri_socket 仍然不存在"\n    fi\nfi'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库',
        script: '# 添加Kubernetes仓库\n# 清理旧的Kubernetes仓库配置\nsudo rm -f /etc/yum.repos.d/kubernetes.repo\nsudo rm -f /etc/yum.repos.d/packages.cloud.google.com_yum_repos_kubernetes-el7-x86_64.repo\n\n# 添加新的Kubernetes仓库\nsudo cat <<EOF > /etc/yum.repos.d/kubernetes.repo\n[kubernetes]\nname=Kubernetes\nbaseurl=https://pkgs.k8s.io/core:/stable:/v1.28/rpm/\nenabled=1\ngpgcheck=1\ngpgkey=https://pkgs.k8s.io/core:/stable:/v1.28/rpm/repodata/repomd.xml.key\nexclude=kubelet kubeadm kubectl\nEOF\n\n# 更新仓库缓存\nsudo dnf makecache'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl',
        script: '# 安装Kubernetes组件\nsudo dnf install -y kubelet${version} kubeadm${version} kubectl${version} --disableexcludes=kubernetes\n\n# 启动kubelet\nsudo systemctl enable --now kubelet'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点',
        script: '# 初始化Kubernetes集群\n# 执行kubeadm init\necho "=== 执行kubeadm init ==="\nsudo kubeadm init --pod-network-cidr=10.244.0.0/16 --upload-certs\n\n# 检查kubeadm init是否成功\nif [ $? -eq 0 ]; then\n    echo "=== kubeadm init 成功 ==="\n    \n    # 配置kubectl\necho "=== 配置kubectl ==="\nmkdir -p $HOME/.kube\n    \n    # 检查admin.conf是否存在\n    if [ -f /etc/kubernetes/admin.conf ]; then\n        echo "✓ 找到admin.conf文件，正在配置kubectl..."\n        sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config\n        sudo chown $(id -u):$(id -g) $HOME/.kube/config\n        echo "✓ kubectl配置成功"\n    else\n        echo "✗ 未找到admin.conf文件，可能初始化过程中出现问题"\n    fi\n    \n    # 安装CNI网络插件（使用Flannel）\n    if [ -f $HOME/.kube/config ]; then\n        echo "=== 安装Flannel网络插件 ==="\n        kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml\n    else\n        echo "✗ 无法安装CNI插件，kubectl配置失败"\n    fi\nelse\n    echo "✗ kubeadm init 失败"\n    # 显示更多错误信息\n    echo "=== 显示kubeadm日志 ==="\n    sudo journalctl -u kubelet --no-pager -n 50\nfi'
      }
    ]
  }
}

// 部署流程数据
const processData = ref(defaultProcessData)

// 监听Kubernetes版本变化，自动更新所有脚本中的仓库URL
watch(selectedKubernetesVersion, (newVersion) => {
  // 为所有发行版本更新脚本中的仓库URL
  systems.value.forEach(distro => {
    // 获取当前选中的源
    const selectedSourceId = selectedSources.value[distro]
    const sources = deploymentSources.value[distro]
    if (sources) {
      const source = sources.find(s => s.id === selectedSourceId)
      if (source) {
        updateScriptRepositoryURL(distro, source.url)
      }
    }
  })
  
  // 保存选中的Kubernetes版本到本地存储
  saveToLocalStorage('selectedKubernetesVersion', newVersion)
})

// 计算属性：当前激活的系统流程
const currentProcess = computed(() => {
  // 添加默认值，确保始终返回一个有效的对象
  return processData.value[activeSystem.value] || {
    name: '默认部署流程',
    steps: []
  }
})

// 部署源管理方法

const switchSource = (source, distro) => {
  // 保存选中的源到本地存储
  saveToLocalStorage('selectedSources', selectedSources.value)
}

const applySource = () => {
  const sources = deploymentSources.value[activeDistro]
  if (!sources) {
    console.error(`No deployment sources found for distro: ${activeDistro}`)
    return
  }
  
  const source = sources.find(s => s.id === selectedSources.value[activeDistro])
  if (!source) {
    console.error(`No source found with id: ${selectedSources.value[activeDistro]} for distro: ${activeDistro}`)
    return
  }
  
  // 更新对应发行版本的部署脚本中的仓库URL
  updateScriptRepositoryURL(activeDistro, source.url)
  
  alert(`已应用${activeDistro.value}部署源: ${source.name}，脚本中的仓库URL已自动更新`)
}

// 更新脚本中的仓库URL
const updateScriptRepositoryURL = (distro, sourceUrl) => {
  const scripts = processData.value[distro].steps
  const version = selectedKubernetesVersion.value
  
  for (let i = 0; i < scripts.length; i++) {
    let script = scripts[i].script
    
    // 检测是否是阿里云源
    const isAliyunSource = sourceUrl.includes('aliyun.com')
    
    // 输出调试信息
    console.log(`Updating script for ${distro}, isAliyunSource: ${isAliyunSource}, sourceUrl: ${sourceUrl}`)
    console.log(`Script name: ${scripts[i].name}`)
    
    // 直接替换脚本内容，不使用复杂的正则表达式
    if (distro === 'centos' || distro === 'rocky' || distro === 'almalinux') {
      // 更新RPM格式的仓库URL (CentOS/RHEL/Rocky/AlmaLinux)
      if (isAliyunSource) {
        // 阿里云新版格式: https://mirrors.aliyun.com/kubernetes-new/core/stable/v1.28/rpm/
        const aliyunBaseUrl = `${sourceUrl}core/stable/${version}/rpm/`
        console.log(`Aliyun base URL: ${aliyunBaseUrl}`)
        
        // 直接替换整个仓库配置块，确保所有URL都正确
        const aliyunRepoConfig = `[kubernetes]\nname=Kubernetes\nbaseurl=${aliyunBaseUrl}\nenabled=1\ngpgcheck=1\ngpgkey=${aliyunBaseUrl}repodata/repomd.xml.key\nexclude=kubelet kubeadm kubectl`
        
        // 使用简单的正则表达式匹配仓库配置块
        script = script.replace(/\[kubernetes\][\s\S]*?exclude=kubelet kubeadm kubectl/g, aliyunRepoConfig)
      } else {
        // 官方源格式: https://pkgs.k8s.io/core:/stable:/v1.28/rpm/
        const officialBaseUrl = `${sourceUrl}core:/stable:/${version}/rpm/`
        console.log(`Official base URL: ${officialBaseUrl}`)
        
        // 直接替换整个仓库配置块
        const officialRepoConfig = `[kubernetes]\nname=Kubernetes\nbaseurl=${officialBaseUrl}\nenabled=1\ngpgcheck=1\ngpgkey=${officialBaseUrl}repodata/repomd.xml.key\nexclude=kubelet kubeadm kubectl`
        
        script = script.replace(/\[kubernetes\][\s\S]*?exclude=kubelet kubeadm kubectl/g, officialRepoConfig)
      }
    } else if (distro === 'ubuntu' || distro === 'debian') {
      // 更新Debian格式的仓库URL (Ubuntu/Debian)
      if (isAliyunSource) {
        // 阿里云新版格式: https://mirrors.aliyun.com/kubernetes-new/core/stable/v1.28/deb/
        const aliyunDebUrl = `${sourceUrl}core/stable/${version}/deb/`
        console.log(`Aliyun deb URL: ${aliyunDebUrl}`)
        
        // 替换Release.key URL
        script = script.replace(/https:\/\/[^\/]+\/[^\/]+\/deb\/Release.key/g, `${aliyunDebUrl}Release.key`)
        // 替换deb仓库URL
        script = script.replace(/https:\/\/[^\/]+\/[^\/]+\/deb\//g, aliyunDebUrl)
      } else {
        // 官方源格式: https://pkgs.k8s.io/core:/stable:/v1.28/deb/
        const officialDebUrl = `${sourceUrl}core:/stable:/${version}/deb/`
        console.log(`Official deb URL: ${officialDebUrl}`)
        
        // 替换Release.key URL
        script = script.replace(/https:\/\/[^\/]+\/[^\/]+\/deb\/Release.key/g, `${officialDebUrl}Release.key`)
        // 替换deb仓库URL
        script = script.replace(/https:\/\/[^\/]+\/[^\/]+\/deb\//g, officialDebUrl)
      }
    }
    
    // 替换脚本中的版本占位符为实际选中的Kubernetes版本
    script = script.replace(/\${version}/g, version)
    
    console.log(`Updated script: ${script.substring(0, 300)}...`)
    
    scripts[i].script = script
  }
  
  // 保存到本地存储
  saveToLocalStorage('processData', processData.value)
  
  // 保存到后端
  saveScriptsToBackend()
};

const applySourceToAll = () => {
  const sources = deploymentSources.value[activeDistro]
  if (!sources) {
    console.error(`No deployment sources found for distro: ${activeDistro}`)
    return
  }
  
  const source = sources.find(s => s.id === selectedSources.value[activeDistro])
  if (!source) {
    console.error(`No source found with id: ${selectedSources.value[activeDistro]} for distro: ${activeDistro}`)
    return
  }
  
  // 为所有发行版本应用相同的源配置
  systems.value.forEach(distro => {
    const distroSources = deploymentSources.value[distro]
    if (distroSources) {
      // 检查该发行版本是否已有同名源
      const existingSourceIndex = distroSources.findIndex(s => s.name === source.name)
      if (existingSourceIndex !== -1) {
        // 更新现有源
        distroSources[existingSourceIndex].url = source.url
        selectedSources.value[distro] = distroSources[existingSourceIndex].id
      } else {
        // 添加新源
        const newId = `${distro}-${distroSources.length + 1}`
        distroSources.push({
          id: newId,
          name: source.name,
          url: source.url
        })
        selectedSources.value[distro] = newId
      }
      
      // 更新该发行版本的部署脚本中的仓库URL
      updateScriptRepositoryURL(distro, source.url)
    }
  })
  
  alert(`已将部署源: ${source.name} 应用到所有发行版本，所有脚本中的仓库URL已自动更新`)
}

// 从后端加载部署流程脚本
const loadScriptsFromBackend = async () => {
  try {
    const response = await axios.get(`${API_BASE_URL}/deployment-process/scripts`)
    const scripts = response.data.scripts
    
    // 将后端脚本映射到processData
    for (const system in scripts) {
      if (processData.value[system]) {
        processData.value[system].steps.forEach((step, index) => {
          // 根据步骤名称和系统类型查找对应的脚本
          const scriptKey = `${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}`
          if (scripts[scriptKey]) {
            step.script = scripts[scriptKey]
          }
        })
      }
    }
  } catch (error) {
    // 后端不可用，使用本地默认脚本
  }
}

// 保存脚本到后端（修改为可选，后端不可用时跳过）
const saveScriptsToBackend = async () => {
  try {
    // 将processData转换为后端需要的格式
    const scriptsToSave = {}
    
    for (const system in processData.value) {
      processData.value[system].steps.forEach(step => {
        const scriptKey = `${system}_${step.name.toLowerCase().replace(/\s+/g, '_')}`
        scriptsToSave[scriptKey] = step.script
      })
    }
    
    await axios.post(`${API_BASE_URL}/deployment-process/scripts`, scriptsToSave)
    return true
  } catch (error) {
    // 后端不可用，跳过保存
    return false
  }
}

// 脚本编辑方法
const editScript = (index) => {
  if (currentProcess.value) {
    currentEditingStepIndex.value = index
    currentEditingStep.value = currentProcess.value.steps[index]
    editingScript.value = currentProcess.value.steps[index].script
    showEditScriptDialog.value = true
  }
}

const closeEditScriptDialog = () => {
  showEditScriptDialog.value = false
  currentEditingStepIndex.value = -1
  currentEditingStep.value = null
  editingScript.value = ''
}

const saveScript = async () => {
  if (activeSystem.value && currentEditingStepIndex.value !== -1) {
    // 直接修改processData对象，确保Vue能检测到变化
    processData.value[activeSystem.value].steps[currentEditingStepIndex.value].script = editingScript.value
    
    // 保存到后端
    const success = await saveScriptsToBackend()
    
    closeEditScriptDialog()
    if (success) {
      alert('脚本已保存')
    }
  }
}

// 组件挂载时加载脚本
onMounted(() => {
  loadScriptsFromBackend()
})
</script>

<style scoped>
.deployment-manager {
  max-width: 1200px;
  margin: 0 auto;
}

h2 {
  font-size: 1.8rem;
  margin-bottom: 20px;
  color: var(--text-primary);
}

h3 {
  font-size: 1.4rem;
  margin: 20px 0 15px 0;
  color: var(--text-primary);
}

/* 部署源切换区域 */
.source-switcher {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  margin-bottom: 30px;
  box-shadow: var(--shadow-sm);
}

.source-options {
  display: flex;
  flex-direction: column;
  gap: 15px;
  margin-bottom: 20px;
}

.source-option {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background-color: var(--bg-secondary);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
  cursor: pointer;
  transition: all 0.3s ease;
}

.source-option:hover {
  border-color: var(--primary-color);
  background-color: rgba(52, 152, 219, 0.05);
}

.source-option input[type="radio"] {
  width: 18px;
  height: 18px;
  accent-color: var(--primary-color);
}

.source-label {
  font-weight: 600;
  color: var(--text-primary);
  min-width: 150px;
}

.source-url {
  font-size: 0.9rem;
  color: var(--text-secondary);
  flex: 1;
  word-break: break-all;
}

/* 系统标签页 */
.system-tabs {
  display: flex;
  gap: 10px;
  margin-bottom: 20px;
  overflow-x: auto;
  padding-bottom: 10px;
}

.tab-btn {
  padding: 10px 20px;
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.3s ease;
  white-space: nowrap;
}

.tab-btn:hover {
  border-color: var(--primary-color);
  color: var(--primary-color);
}

.tab-btn.active {
  background-color: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

/* 流程步骤 */
.process-steps {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
}

.process-step {
  margin-bottom: 25px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--border-color);
}

.process-step:last-child {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: none;
}

.step-header {
  display: flex;
  gap: 15px;
  margin-bottom: 15px;
  align-items: flex-start;
}

.step-number {
  width: 30px;
  height: 30px;
  background-color: var(--primary-color);
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  flex-shrink: 0;
  margin-top: 2px;
}

.step-info h4 {
  font-size: 1.2rem;
  margin: 0 0 5px 0;
  color: var(--text-primary);
}

.step-description {
  font-size: 0.95rem;
  color: var(--text-secondary);
  margin: 0;
}

.step-content {
  margin-left: 45px;
}

.step-script {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-sm);
  padding: 15px;
  border: 1px solid var(--border-color);
}

.step-script h5 {
  font-size: 1rem;
  margin: 0 0 10px 0;
  color: var(--text-primary);
}

.step-script pre {
  margin: 0;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.9rem;
  line-height: 1.5;
  overflow-x: auto;
  color: var(--text-primary);
  background-color: var(--bg-primary);
  padding: 10px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
}

/* 按钮样式 */
.btn {
  padding: 10px 20px;
  border: none;
  border-radius: var(--radius-md);
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 0.95rem;
}

.btn-primary {
  background-color: var(--primary-color);
  color: white;
}

.btn-primary:hover {
  background-color: var(--primary-dark);
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
}

.btn-small {
  padding: 6px 12px;
  font-size: 0.85rem;
}

.btn-danger {
  background-color: var(--error-color);
  color: white;
}

.btn-danger:hover {
  background-color: #c0392b;
}

/* 对话框样式 */
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog-content {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 25px;
  box-shadow: var(--shadow-lg);
  width: 90%;
  max-width: 600px;
  max-height: 90vh;
  overflow-y: auto;
}

.dialog-large {
  max-width: 800px;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
}

.dialog-header h4 {
  margin: 0;
  font-size: 1.3rem;
  color: var(--text-primary);
}

.dialog-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: var(--text-secondary);
  transition: color 0.3s ease;
}

.dialog-close:hover {
  color: var(--text-primary);
}

.dialog-body {
  margin-bottom: 25px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 20px;
  border-top: 1px solid var(--border-color);
}

/* 表单样式 */
.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 600;
  color: var(--text-primary);
  font-size: 0.95rem;
}

.form-input {
  width: 100%;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background-color: var(--bg-secondary);
  color: var(--text-primary);
  font-size: 0.95rem;
  transition: all 0.3s ease;
}

.form-input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

.form-textarea {
  width: 100%;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background-color: var(--bg-secondary);
  color: var(--text-primary);
  font-size: 0.95rem;
  font-family: 'Courier New', Courier, monospace;
  resize: vertical;
  transition: all 0.3s ease;
}

.form-textarea:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

/* 源管理样式 */
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.source-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

.source-actions-bottom {
  margin-top: 20px;
  text-align: right;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

.source-distro-selector {
  display: flex;
  align-items: center;
  gap: 10px;
}

.source-distro-selector label {
  margin: 0;
  font-weight: 600;
  color: var(--text-primary);
}

.source-distro-selector .form-input {
  min-width: 150px;
}

.btn-secondary {
  background-color: var(--bg-secondary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

.btn-secondary:hover {
  background-color: var(--bg-primary);
  border-color: var(--primary-color);
  color: var(--primary-color);
}

.source-option {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background-color: var(--bg-secondary);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
  cursor: pointer;
  transition: all 0.3s ease;
  flex-wrap: wrap;
}

.source-option:hover {
  border-color: var(--primary-color);
  background-color: rgba(52, 152, 219, 0.05);
}

.step-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  margin-bottom: 5px;
}

.step-title-row h4 {
  margin: 0;
}

.script-header {
  margin-bottom: 10px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>