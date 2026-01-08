<template>
  <div class="kubeadm-manager">
    <h2>Kubernetes集群部署</h2>
    
    <!-- 部署步骤指示器 -->
    <div class="steps-indicator">
      <div 
        v-for="(step, index) in steps" 
        :key="index" 
        class="step-item"
        :class="{
          'active': currentStep === index,
          'completed': currentStep > index,
          'failed': step.status === 'failed'
        }"
      >
        <div class="step-number">{{ index + 1 }}</div>
        <div class="step-title">{{ step.title }}</div>
        <div class="step-status" v-if="step.status">
          {{ step.status === 'completed' ? '✓' : step.status === 'failed' ? '✗' : '' }}
        </div>
      </div>
    </div>
    
    <!-- 步骤内容 -->
    <div class="step-content">
      <!-- 步骤1: 选择节点 -->
      <div v-if="currentStep === 0" class="step-node-selection">
        <h3>选择节点</h3>
        <div class="node-selection-container">
          <div class="node-filters">
            <div class="form-row">
              <div class="form-group">
                <label for="runtime-filter">容器运行时:</label>
                <select id="runtime-filter" v-model="selectedRuntimeFilter">
                  <option value="">所有</option>
                  <option value="containerd">Containerd</option>
                  <option value="cri-o">CRI-O</option>
                </select>
              </div>
              <div class="form-group">
                <label for="status-filter">状态:</label>
                <select id="status-filter" v-model="selectedStatusFilter">
                  <option value="">所有</option>
                  <option value="ready">就绪</option>
                  <option value="not-ready">未就绪</option>
                </select>
              </div>
            </div>
          </div>
          
          <div class="available-nodes">
            <h4>可用节点</h4>
            <div class="nodes-grid">
              <div 
                v-for="node in filteredNodes" 
                :key="node.id"
                class="node-card"
                :class="{
                  'selected': selectedNodes[node.id] !== undefined,
                  'master': selectedNodes[node.id] === 'master',
                  'worker': selectedNodes[node.id] === 'worker'
                }"
              >
                <div class="node-info">
                  <h5>{{ node.name }}</h5>
                  <div class="node-meta">
                    <span class="node-ip">{{ node.ip }}</span>
                    <span class="node-os">{{ node.os }}</span>
                    <span class="node-runtime">{{ node.containerRuntime }}</span>
                  </div>
                </div>
                <div class="node-selection-actions">
                  <div class="node-type-selector">
                    <button 
                      class="node-type-btn" 
                      :class="{ active: selectedNodes[node.id] === 'master' }"
                      @click="selectNodeType(node.id, 'master')"
                    >
                      主节点
                    </button>
                    <button 
                      class="node-type-btn" 
                      :class="{ active: selectedNodes[node.id] === 'worker' }"
                      @click="selectNodeType(node.id, 'worker')"
                    >
                      工作节点
                    </button>
                    <button 
                      class="node-type-btn" 
                      :class="{ active: selectedNodes[node.id] === undefined }"
                      @click="selectNodeType(node.id, undefined)"
                    >
                      取消
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
          
          <div class="selected-nodes-summary">
            <h4>已选择节点</h4>
            <div class="summary-info">
              <div class="summary-item">
                <span class="summary-label">主节点:</span>
                <span class="summary-value">{{ masterNodesCount }} 个</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">工作节点:</span>
                <span class="summary-value">{{ workerNodesCount }} 个</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">总节点数:</span>
                <span class="summary-value">{{ totalNodesCount }} 个</span>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- 步骤2: 部署配置 -->
      <div v-if="currentStep === 1" class="step-deploy-config">
        <h3>部署配置</h3>
        <div class="deploy-config-form">
          <div class="form-row">
            <div class="form-group">
              <label for="kube-version">Kubernetes版本:</label>
              <select id="kube-version" v-model="deployConfig.kubeVersion" required>
                <option value="">-- 选择版本 --</option>
                <option v-for="version in availableVersions" :key="version" :value="version">{{ version }}</option>
              </select>
            </div>
            <div class="form-group">
              <label for="pod-network">Pod网络插件:</label>
              <select id="pod-network" v-model="deployConfig.podNetwork" required>
                <option value="calico">Calico</option>
                <option value="flannel">Flannel</option>
                <option value="cilium">Cilium</option>
              </select>
            </div>
          </div>
          
          <div class="form-row">
            <div class="form-group">
              <label for="container-runtime">容器运行时:</label>
              <select id="container-runtime" v-model="deployConfig.containerRuntime" required>
                <option value="containerd">Containerd</option>
                <option value="cri-o">CRI-O</option>
              </select>
            </div>
            <div class="form-group">
              <label for="service-cidr">Service CIDR:</label>
              <input 
                type="text" 
                id="service-cidr" 
                v-model="deployConfig.serviceCIDR" 
                placeholder="10.96.0.0/12" 
                required
              >
            </div>
          </div>
          
          <div class="form-row">
            <div class="form-group">
              <label for="pod-cidr">Pod CIDR:</label>
              <input 
                type="text" 
                id="pod-cidr" 
                v-model="deployConfig.podCIDR" 
                placeholder="192.168.0.0/16" 
                required
              >
            </div>
            <div class="form-group">
              <label for="api-server-port">API Server端口:</label>
              <input 
                type="number" 
                id="api-server-port" 
                v-model="deployConfig.apiServerPort" 
                placeholder="6443" 
                required
              >
            </div>
          </div>
          
          <div class="form-row">
            <div class="form-group">
              <label class="checkbox-label">
                <input type="checkbox" v-model="deployConfig.enableHA">
                启用高可用(HA)
              </label>
            </div>
            <div class="form-group">
              <label class="checkbox-label">
                <input type="checkbox" v-model="deployConfig.enableMetrics">
                启用Metrics Server
              </label>
            </div>
          </div>
        </div>
        
        <div class="node-configuration-summary">
          <h3>节点配置预览</h3>
          <div class="summary-grid">
            <div class="summary-section">
              <h5>主节点配置</h5>
              <div v-for="node in masterNodes" :key="node.id" class="preview-node">
                {{ node.name }} ({{ node.ip }}) - {{ node.containerRuntime }}
              </div>
            </div>
            <div class="summary-section">
              <h5>工作节点配置</h5>
              <div v-for="node in workerNodes" :key="node.id" class="preview-node">
                {{ node.name }} ({{ node.ip }}) - {{ node.containerRuntime }}
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- 步骤3: 部署主节点 -->
      <div v-if="currentStep === 2" class="step-master-deployment">
        <h3>部署主节点</h3>
        <div class="deployment-progress-container">
          <div class="master-node-list">
            <div 
              v-for="node in masterNodes" 
              :key="node.id" 
              class="deployment-node-item"
              :class="{ 'deployed': deploymentStatus.master[node.id] === 'completed', 'failed': deploymentStatus.master[node.id] === 'failed' }"
            >
              <div class="node-header">
                <span class="node-name">{{ node.name }} ({{ node.ip }})</span>
                <span class="deployment-status">{{ getDeploymentStatusText(deploymentStatus.master[node.id]) }}</span>
              </div>
              <div class="node-progress-bar">
                <div 
                  class="progress-bar" 
                  :style="{ width: `${deploymentProgress.master[node.id] || 0}%` }"
                  :class="deploymentStatus.master[node.id] === 'failed' ? 'failed' : ''"
                ></div>
              </div>
            </div>
          </div>
          
          <div class="deployment-logs">
            <h4>部署日志</h4>
            <div class="logs-container">
              <pre>{{ deployLogs }}</pre>
            </div>
          </div>
        </div>
      </div>
      
      <!-- 步骤4: 部署工作节点 -->
      <div v-if="currentStep === 3" class="step-worker-deployment">
        <h3>部署工作节点</h3>
        <div class="deployment-progress-container">
          <div class="worker-node-list">
            <div 
              v-for="node in workerNodes" 
              :key="node.id" 
              class="deployment-node-item"
              :class="{ 'deployed': deploymentStatus.worker[node.id] === 'completed', 'failed': deploymentStatus.worker[node.id] === 'failed' }"
            >
              <div class="node-header">
                <span class="node-name">{{ node.name }} ({{ node.ip }})</span>
                <span class="deployment-status">{{ getDeploymentStatusText(deploymentStatus.worker[node.id]) }}</span>
              </div>
              <div class="node-progress-bar">
                <div 
                  class="progress-bar" 
                  :style="{ width: `${deploymentProgress.worker[node.id] || 0}%` }"
                  :class="deploymentStatus.worker[node.id] === 'failed' ? 'failed' : ''"
                ></div>
              </div>
            </div>
          </div>
          
          <div class="deployment-logs">
            <h4>部署日志</h4>
            <div class="logs-container">
              <pre>{{ deployLogs }}</pre>
            </div>
          </div>
        </div>
      </div>
      
      <!-- 步骤5: 部署完成 -->
      <div v-if="currentStep === 4" class="step-completion">
        <h3>部署完成</h3>
        <div class="completion-summary">
          <div class="summary-card success">
            <h4>部署结果</h4>
            <div class="summary-stats">
              <div class="stat-item">
                <span class="stat-label">主节点:</span>
                <span class="stat-value">{{ masterNodes.length }} / {{ masterNodes.length }} 成功</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">工作节点:</span>
                <span class="stat-value">{{ workerNodes.length }} / {{ workerNodes.length }} 成功</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">集群状态:</span>
                <span class="stat-value success">正常运行</span>
              </div>
            </div>
          </div>
          
          <div class="summary-card info">
            <h4>集群信息</h4>
            <div class="cluster-info">
              <div class="info-item">
                <span class="info-label">Kubernetes版本:</span>
                <span class="info-value">{{ deployConfig.kubeVersion }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">API Server地址:</span>
                <span class="info-value">{{ clusterInfo.apiServerAddress }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">Pod网络插件:</span>
                <span class="info-value">{{ deployConfig.podNetwork }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">容器运行时:</span>
                <span class="info-value">{{ deployConfig.containerRuntime }}</span>
              </div>
            </div>
          </div>
          
          <div class="summary-card warning">
            <h4>后续操作建议</h4>
            <ul class="next-steps">
              <li>安装Helm包管理器</li>
              <li>部署Ingress Controller</li>
              <li>配置监控系统(Prometheus + Grafana)</li>
              <li>设置日志收集系统(ELK或Loki)</li>
              <li>定期备份etcd数据</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
    
    <!-- 步骤导航按钮 -->
    <div class="step-navigation">
      <button 
        class="btn btn-secondary" 
        @click="goToPreviousStep" 
        :disabled="currentStep === 0 || isDeploying"
      >
        上一步
      </button>
      <button 
        v-if="currentStep < steps.length - 1" 
        class="btn btn-primary" 
        @click="goToNextStep" 
        :disabled="!canProceedToNextStep() || isDeploying"
      >
        <span v-if="isDeploying" class="loading-spinner"></span>
        {{ isDeploying ? '部署中...' : '下一步' }}
      </button>
      <button 
        v-else 
        class="btn btn-success" 
        @click="finishDeployment"
      >
        完成部署
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'

// 定义组件的属性和事件
const props = defineProps({
  availableVersions: {
    type: Array,
    default: () => []
  },
  nodes: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['showMessage', 'setKubeadmVersion'])

// API配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 300000 // 5分钟超时，适应Kubernetes组件安装的耗时过程
})

// 部署步骤
const steps = ref([
  { title: '选择节点', status: '' },
  { title: '部署配置', status: '' },
  { title: '部署主节点', status: '' },
  { title: '部署工作节点', status: '' },
  { title: '部署完成', status: '' }
])

// 当前步骤
const currentStep = ref(0)

// 节点相关状态
const selectedNodes = ref({})
const selectedRuntimeFilter = ref('')
const selectedStatusFilter = ref('')

// 部署配置
const deployConfig = ref({
  kubeVersion: '',
  podNetwork: 'calico',
  containerRuntime: 'containerd',
  serviceCIDR: '10.96.0.0/12',
  podCIDR: '192.168.0.0/16',
  apiServerPort: 6443,
  enableHA: false,
  enableMetrics: true
})

// 部署状态
const isDeploying = ref(false)
const deployLogs = ref('Kubernetes集群部署日志\n=====================\n')
const deploymentStatus = ref({
  master: {},
  worker: {}
})
const deploymentProgress = ref({
  master: {},
  worker: {}
})

// 集群信息
const clusterInfo = ref({
  apiServerAddress: '',
  clusterName: '',
  clusterId: ''
})

// 计算属性：过滤后的节点
const filteredNodes = computed(() => {
  return props.nodes.filter(node => {
    const matchesRuntime = !selectedRuntimeFilter.value || node.containerRuntime === selectedRuntimeFilter.value
    const matchesStatus = !selectedStatusFilter.value || node.status === selectedStatusFilter.value
    return matchesRuntime && matchesStatus
  })
})

// 计算属性：主节点数量
const masterNodesCount = computed(() => {
  return Object.values(selectedNodes.value).filter(type => type === 'master').length
})

// 计算属性：工作节点数量
const workerNodesCount = computed(() => {
  return Object.values(selectedNodes.value).filter(type => type === 'worker').length
})

// 计算属性：总节点数量
const totalNodesCount = computed(() => {
  return Object.keys(selectedNodes.value).length
})

// 计算属性：主节点列表
const masterNodes = computed(() => {
  return props.nodes.filter(node => selectedNodes.value[node.id] === 'master')
})

// 计算属性：工作节点列表
const workerNodes = computed(() => {
  return props.nodes.filter(node => selectedNodes.value[node.id] === 'worker')
})

// 选择节点类型
const selectNodeType = (nodeId, type) => {
  const node = props.nodes.find(n => n.id === nodeId)
  if (node) {
    if (type === undefined) {
      // 取消选择
      deployLogs.value += `[${new Date().toLocaleString()}] 取消选择节点: ${node.name} (${node.ip})\n`
      delete selectedNodes.value[nodeId]
    } else {
      // 选择节点类型
      const oldType = selectedNodes.value[nodeId]
      if (oldType) {
        deployLogs.value += `[${new Date().toLocaleString()}] 将节点 ${node.name} (${node.ip}) 从 ${oldType} 改为 ${type}\n`
      } else {
        deployLogs.value += `[${new Date().toLocaleString()}] 选择节点 ${node.name} (${node.ip}) 作为 ${type}\n`
      }
      selectedNodes.value[nodeId] = type
    }
  }
}

// 判断是否可以进入下一步
const canProceedToNextStep = () => {
  switch (currentStep.value) {
    case 0: // 选择节点
      return masterNodesCount.value > 0 && totalNodesCount.value > 0
    case 1: // 部署配置
      return deployConfig.value.kubeVersion && deployConfig.value.podNetwork && deployConfig.value.containerRuntime
    case 2: // 部署主节点
      return Object.values(deploymentStatus.value.master).every(status => status === 'completed')
    case 3: // 部署工作节点
      return Object.values(deploymentStatus.value.worker).every(status => status === 'completed')
    default:
      return true
  }
}

// 检查节点容器运行时状态
const checkContainerRuntime = () => {
  const selectedNodeIds = Object.keys(selectedNodes.value)
  const nodesWithoutRuntime = selectedNodeIds.filter(nodeId => {
    const node = props.nodes.find(n => n.id === nodeId)
    return !node.containerRuntime || node.containerRuntime === ''
  })
  
  return {
    hasNodesWithoutRuntime: nodesWithoutRuntime.length > 0,
    nodesWithoutRuntime: nodesWithoutRuntime
  }
}

// 安装容器运行时
const installContainerRuntime = async () => {
  isDeploying.value = true
  deployLogs.value = '开始安装容器运行时...\n'
  
  // 获取没有容器运行时的节点
  const selectedNodeIds = Object.keys(selectedNodes.value)
  const nodesWithoutRuntime = selectedNodeIds.filter(nodeId => {
    const node = props.nodes.find(n => n.id === nodeId)
    return !node.containerRuntime || node.containerRuntime === ''
  })
  
  try {
    // 调用后端API安装容器运行时
    const response = await apiClient.post('/nodes/runtime/batch-install', {
      nodeIds: nodesWithoutRuntime,
      runtimeType: deployConfig.value.containerRuntime,
      version: '' // 使用默认版本
    })
    
    deployLogs.value += response.data.result + '\n'
    
    // 重新获取节点信息
    deployLogs.value += '更新节点信息...\n'
    emit('update:nodes') // 触发更新节点列表
    
    // 给系统一点时间更新节点信息
    await new Promise(resolve => setTimeout(resolve, 1000))
    
    emit('showMessage', { text: '容器运行时安装成功!', type: 'success' })
    
    // 继续部署流程
    await goToNextStep()
  } catch (error) {
    deployLogs.value += '安装容器运行时失败: ' + (error.response?.data?.error || error.message) + '\n'
    emit('showMessage', { text: '安装容器运行时失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 进入下一步
const goToNextStep = async (skipRuntimeCheck = false) => {
  deployLogs.value += `[${new Date().toLocaleString()}] 准备进入下一步：从步骤 ${currentStep.value + 1} 到步骤 ${currentStep.value + 2}\n`
  
  // 如果是从步骤1进入步骤2，检查节点容器运行时
  if (currentStep.value === 0 && currentStep.value + 1 === 1 && !skipRuntimeCheck) {
    deployLogs.value += `[${new Date().toLocaleString()}] 检查节点容器运行时...\n`
    const { hasNodesWithoutRuntime, nodesWithoutRuntime } = checkContainerRuntime()
    
    if (hasNodesWithoutRuntime) {
      const nodeNames = nodesWithoutRuntime.map(nodeId => {
        const node = props.nodes.find(n => n.id === nodeId)
        return node ? node.name : nodeId
      })
      deployLogs.value += `[${new Date().toLocaleString()}] 发现 ${nodeNames.length} 个节点没有安装容器运行时：${nodeNames.join(', ')}\n`
      
      if (confirm(`以下节点没有安装容器运行时: ${nodeNames.join(', ')}\n是否自动安装${deployConfig.value.containerRuntime}?`)) {
        await installContainerRuntime()
        return
      } else {
        // 用户取消安装，不允许继续
        deployLogs.value += `[${new Date().toLocaleString()}] 用户取消安装容器运行时，部署流程终止\n`
        emit('showMessage', { text: '请先为所有节点安装容器运行时', type: 'warning' })
        return
      }
    } else {
      deployLogs.value += `[${new Date().toLocaleString()}] 所有节点已安装容器运行时，继续部署...\n`
    }
  }
  
  if (!canProceedToNextStep()) {
    deployLogs.value += `[${new Date().toLocaleString()}] 无法进入下一步，检查是否满足条件\n`
    return
  }
  
  // 更新当前步骤状态
  steps.value[currentStep.value].status = 'completed'
  
  // 进入下一步
  currentStep.value++
  deployLogs.value += `[${new Date().toLocaleString()}] 进入步骤 ${currentStep.value + 1}: ${steps.value[currentStep.value].title}\n`
  
  // 如果是进入主节点部署步骤，开始部署
  if (currentStep.value === 2) {
    await deployMasterNodes()
  }
  
  // 如果是进入工作节点部署步骤，开始部署
  if (currentStep.value === 3) {
    await deployWorkerNodes()
  }
}

// 回到上一步
const goToPreviousStep = () => {
  if (currentStep.value > 0) {
    deployLogs.value += `[${new Date().toLocaleString()}] 回到上一步：从步骤 ${currentStep.value + 1} 到步骤 ${currentStep.value}\n`
    currentStep.value--
  }
}

// 部署主节点
const deployMasterNodes = async () => {
  isDeploying.value = true
  deployLogs.value += `[${new Date().toLocaleString()}] 开始部署主节点...\n`

  // 如果没有主节点，直接返回
  if (masterNodes.value.length === 0) {
    deployLogs.value += '没有选择主节点，无法部署\n'
    isDeploying.value = false
    return
  }

  // 初始化部署状态
  masterNodes.value.forEach(node => {
    deploymentStatus.value.master[node.id] = 'deploying'
    deploymentProgress.value.master[node.id] = 0
  })

  try {
    // 逐个部署主节点
    for (const node of masterNodes.value) {
      deployLogs.value += `开始部署主节点: ${node.name} (${node.ip})\n`

      // 更新进度为10%
      deploymentProgress.value.master[node.id] = 10

      // 安装对应版本的kubeadm
      deployLogs.value += `安装Kubernetes组件 (版本: ${deployConfig.value.kubeVersion})...\n`
      deployLogs.value += `远程执行详细过程：\n`
      deployLogs.value += `1. 清理旧的Kubernetes repo配置\n`
      deployLogs.value += `2. 添加新的Kubernetes repo (pkgs.k8s.io)\n`
      deployLogs.value += `3. 更新repo缓存\n`
      deployLogs.value += `4. 安装kubelet、kubeadm、kubectl组件\n`
      deployLogs.value += `5. 配置kubelet使用systemd cgroup驱动\n`
      deployLogs.value += `6. 启动并启用kubelet服务\n`
      deploymentProgress.value.master[node.id] = 20
      
      // 调用后端API安装kubeadm
      try {
        const response = await apiClient.post(`/nodes/${node.id}/kubernetes/install`, {
          kubeadmVersion: deployConfig.value.kubeVersion
        })
        
        deploymentProgress.value.master[node.id] = 40
        deployLogs.value += `=== 开始安装Kubernetes组件 ===\n`
        deployLogs.value += response.data.result + `\n`
        deployLogs.value += `=== Kubernetes组件安装完成 ===\n\n`
      } catch (error) {
        deployLogs.value += `=== Kubernetes组件安装失败 ===\n`
        deployLogs.value += `${error.response?.data?.error || error.message}\n\n`
        throw error
      }

      // 调用后端API初始化主节点
      deployLogs.value += `=== 开始初始化主节点 ===\n`
      
      const initResponse = await apiClient.post('/kubeadm/init', {
        masterNodeId: node.id,
        config: {
          initConfiguration: {
            localAPIEndpoint: {
              advertiseAddress: node.ip,
              bindPort: deployConfig.value.apiServerPort
            },
            nodeRegistration: {
              name: node.name,
              criSocket: '/run/containerd/containerd.sock',
              taints: []
            }
          },
          clusterConfiguration: {
            kubernetesVersion: deployConfig.value.kubeVersion,
            controlPlaneEndpoint: `${node.ip}:${deployConfig.value.apiServerPort}`,
            networking: {
              podSubnet: deployConfig.value.podCIDR,
              serviceSubnet: deployConfig.value.serviceCIDR,
              dnsDomain: 'cluster.local',
              serviceNodePortRange: '30000-32767'
            },
            api: {
              timeoutForControlPlane: 300
            },
            controllerManager: {
              extraArgs: {}
            },
            scheduler: {
              extraArgs: {}
            }
          },
          etcd: {
            local: {
              dataDir: '/var/lib/etcd',
              extraArgs: {}
            }
          }
        }
      })

      deploymentProgress.value.master[node.id] = 50
      deployLogs.value += initResponse.data.result + `\n`
      deployLogs.value += `=== 主节点初始化完成 ===\n\n`

      // 获取join命令供工作节点使用
      try {
        const joinResponse = await apiClient.get('/kubeadm/join-command', {
          params: { masterNodeId: node.id }
        })
        deployLogs.value += `获取到Join命令\n`
      } catch (joinError) {
        deployLogs.value += `获取Join命令失败: ${joinError.message}\n`
      }

      // 拉取Kubernetes镜像
      deploymentProgress.value.master[node.id] = 70
      try {
        await apiClient.post('/kubeadm/images/pull', {
          masterNodeId: node.id,
          version: deployConfig.value.kubeVersion
        })
        deployLogs.value += `Kubernetes镜像已拉取\n`
      } catch (pullError) {
        deployLogs.value += `拉取镜像失败: ${pullError.message}，尝试继续部署...\n`
      }

      // 更新进度为90%
      deploymentProgress.value.master[node.id] = 90

      deploymentStatus.value.master[node.id] = 'completed'
      deploymentProgress.value.master[node.id] = 100
      deployLogs.value += `主节点 ${node.name} 部署成功!\n`
    }

    steps.value[2].status = 'completed'
    deployLogs.value += '所有主节点部署完成!\n'
  } catch (error) {
    deployLogs.value += '部署失败: ' + (error.response?.data?.error || error.message) + '\n'
    steps.value[2].status = 'failed'
    // 设置所有主节点为失败状态
    masterNodes.value.forEach(node => {
      deploymentStatus.value.master[node.id] = 'failed'
    })
  } finally {
    isDeploying.value = false
  }
}

// 部署工作节点
const deployWorkerNodes = async () => {
  isDeploying.value = true
  deployLogs.value += '\n开始部署工作节点...\n'
  
  // 初始化部署状态
  workerNodes.value.forEach(node => {
    deploymentStatus.value.worker[node.id] = 'deploying'
    deploymentProgress.value.worker[node.id] = 0
  })
  
  try {
    // 逐个部署工作节点
    for (const node of workerNodes.value) {
      deployLogs.value += `开始部署工作节点: ${node.name} (${node.ip})\n`
      
      // 更新进度为10%
      deploymentProgress.value.worker[node.id] = 10

      // 安装对应版本的kubeadm
      deployLogs.value += `安装Kubernetes组件 (版本: ${deployConfig.value.kubeVersion})...\n`
      deployLogs.value += `远程执行详细过程：\n`
      deployLogs.value += `1. 清理旧的Kubernetes repo配置\n`
      deployLogs.value += `2. 添加新的Kubernetes repo (pkgs.k8s.io)\n`
      deployLogs.value += `3. 更新repo缓存\n`
      deployLogs.value += `4. 安装kubelet、kubeadm、kubectl组件\n`
      deployLogs.value += `5. 配置kubelet使用systemd cgroup驱动\n`
      deployLogs.value += `6. 启动并启用kubelet服务\n`
      deploymentProgress.value.worker[node.id] = 20
      
      // 调用后端API安装kubeadm
      try {
        const response = await apiClient.post(`/nodes/${node.id}/kubernetes/install`, {
          kubeadmVersion: deployConfig.value.kubeVersion
        })
        
        deploymentProgress.value.worker[node.id] = 40
        deployLogs.value += `=== 开始安装Kubernetes组件 ===\n`
        deployLogs.value += response.data.result + `\n`
        deployLogs.value += `=== Kubernetes组件安装完成 ===\n\n`
      } catch (error) {
        deployLogs.value += `=== Kubernetes组件安装失败 ===\n`
        deployLogs.value += `${error.response?.data?.error || error.message}\n\n`
        throw error
      }

      // 获取join命令
      deployLogs.value += `获取Join命令...\n`
      deploymentProgress.value.worker[node.id] = 50
      
      const joinResponse = await apiClient.get('/kubeadm/join-command', {
        params: { masterNodeId: masterNodes.value[0].id }
      })
      
      const joinCommand = joinResponse.data.command
      deployLogs.value += `获取到Join命令\n`
      deploymentProgress.value.worker[node.id] = 60

      // 执行join命令加入集群
      deployLogs.value += `执行Join命令加入集群...\n`
      deploymentProgress.value.worker[node.id] = 70
      
      // 解析join命令，提取token、caCertHash和controlPlaneEndpoint
      // 命令格式: kubeadm join <control-plane-endpoint>:6443 --token <token> --discovery-token-ca-cert-hash <hash>
      // 或者: kubeadm join <control-plane-endpoint>:6443 --token <token> --discovery-token-ca-cert-hash sha256:<hash>
      
      // 更健壮的解析逻辑
      const cmdParts = joinCommand.split(' ')
      if (cmdParts.length < 7) {
        throw new Error(`Join命令格式错误: ${joinCommand}`)
      }
      
      // 查找各个参数的位置
      const controlPlaneEndpointIndex = cmdParts.indexOf('join') + 1
      const tokenIndex = cmdParts.indexOf('--token') + 1
      const caCertHashIndex = cmdParts.indexOf('--discovery-token-ca-cert-hash') + 1
      
      if (controlPlaneEndpointIndex === 0 || tokenIndex === 0 || caCertHashIndex === 0 || 
         controlPlaneEndpointIndex >= cmdParts.length || tokenIndex >= cmdParts.length || caCertHashIndex >= cmdParts.length) {
        throw new Error(`Join命令格式错误: ${joinCommand}`)
      }
      
      const controlPlaneEndpoint = cmdParts[controlPlaneEndpointIndex]
      const token = cmdParts[tokenIndex]
      const caCertHash = cmdParts[caCertHashIndex]
      
      // 调用join API执行加入集群
      const joinResult = await apiClient.post('/kubeadm/join', {
        workerNodeId: node.id,
        token: token,
        caCertHash: caCertHash,
        controlPlaneEndpoint: controlPlaneEndpoint
      })
      
      deploymentProgress.value.worker[node.id] = 90
      deployLogs.value += `工作节点 ${node.name} 加入集群成功\n`

      deploymentStatus.value.worker[node.id] = 'completed'
      deploymentProgress.value.worker[node.id] = 100
      deployLogs.value += `工作节点 ${node.name} 部署成功!\n`
    }
    
    steps.value[3].status = 'completed'
    deployLogs.value += '所有工作节点部署完成!\n'
    
    // 设置集群信息
    clusterInfo.value = {
      apiServerAddress: `https://${masterNodes.value[0].ip}:${deployConfig.value.apiServerPort}`,
      clusterName: 'k8s-cluster',
      clusterId: 'cluster-12345'
    }
  } catch (error) {
    deployLogs.value += '部署失败: ' + (error.response?.data?.error || error.message) + '\n'
    steps.value[3].status = 'failed'
    // 设置所有工作节点为失败状态
    workerNodes.value.forEach(node => {
      deploymentStatus.value.worker[node.id] = 'failed'
    })
  } finally {
    isDeploying.value = false
  }
}

// 完成部署
const finishDeployment = () => {
  deployLogs.value += `[${new Date().toLocaleString()}] Kubernetes集群部署完成!\n`
  deployLogs.value += `==========================================\n`
  deployLogs.value += `集群信息：\n`
  deployLogs.value += `- 版本：${deployConfig.value.kubeVersion}\n`
  deployLogs.value += `- 主节点数：${masterNodes.value.length}\n`
  deployLogs.value += `- 工作节点数：${workerNodes.value.length}\n`
  deployLogs.value += `- 容器运行时：${deployConfig.value.containerRuntime}\n`
  deployLogs.value += `- Pod网络插件：${deployConfig.value.podNetwork}\n`
  deployLogs.value += `- API Server地址：${clusterInfo.value.apiServerAddress}\n`
  deployLogs.value += `==========================================\n`
  emit('showMessage', { text: 'Kubernetes集群部署完成!', type: 'success' })
  // 可以添加跳转到集群管理页面的逻辑
}

// 获取部署状态文本
const getDeploymentStatusText = (status) => {
  const statusMap = {
    '': '未开始',
    'deploying': '部署中',
    'completed': '已完成',
    'failed': '部署失败'
  }
  return statusMap[status] || '未知状态'
}
</script>

<style scoped>
.kubeadm-manager {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

/* 步骤指示器 */
.steps-indicator {
  display: flex;
  gap: 20px;
  margin-bottom: 30px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--border-color);
  flex-wrap: wrap;
}

.step-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 15px;
  border-radius: var(--radius-md);
  background-color: var(--bg-card);
  border: 2px solid var(--border-color);
  transition: all 0.3s ease;
  position: relative;
}

.step-item::after {
  content: '';
  position: absolute;
  right: -25px;
  top: 50%;
  transform: translateY(-50%);
  width: 20px;
  height: 2px;
  background-color: var(--border-color);
}

.step-item:last-child::after {
  display: none;
}

.step-item.active {
  background-color: var(--primary-color);
  border-color: var(--primary-color);
  color: white;
}

.step-item.completed {
  background-color: var(--success-color);
  border-color: var(--success-color);
  color: white;
}

.step-item.failed {
  background-color: var(--error-color);
  border-color: var(--error-color);
  color: white;
}

.step-number {
  font-weight: 700;
  font-size: 1.1rem;
}

.step-title {
  font-weight: 600;
}

.step-status {
  font-size: 1.2rem;
  font-weight: bold;
}

/* 步骤内容 */
.step-content {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 25px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  margin-bottom: 25px;
}

/* 节点选择步骤 */
.step-node-selection .node-selection-container {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

.node-filters {
  background-color: var(--bg-secondary);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.available-nodes {
  margin-top: 20px;
}

.nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 15px;
  margin-top: 15px;
}

.node-card {
  background-color: var(--bg-card);
  border: 2px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 20px;
  transition: all 0.3s ease;
  cursor: pointer;
}

.node-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.node-card.selected {
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

.node-card.master {
  border-color: var(--success-color);
  background-color: rgba(39, 174, 96, 0.05);
}

.node-card.worker {
  border-color: var(--primary-color);
  background-color: rgba(52, 152, 219, 0.05);
}

.node-info h5 {
  margin: 0 0 10px 0;
  font-size: 1.1rem;
  font-weight: 600;
}

.node-meta {
  display: flex;
  flex-direction: column;
  gap: 5px;
  font-size: 0.85rem;
  color: var(--text-secondary);
}

.node-type-selector {
  display: flex;
  gap: 10px;
  margin-top: 15px;
}

.node-type-btn {
  flex: 1;
  padding: 8px 12px;
  border: 1px solid var(--border-color);
  background-color: var(--bg-secondary);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 0.85rem;
  font-weight: 500;
}

.node-type-btn:hover {
  background-color: var(--border-color);
}

.node-type-btn.active {
  background-color: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

/* 已选择节点摘要 */
.selected-nodes-summary {
  background-color: var(--bg-secondary);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  margin-top: 20px;
}

.selected-nodes-summary h4 {
  margin: 0 0 15px 0;
  font-size: 1rem;
  font-weight: 600;
}

.summary-info {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}

.summary-item {
  display: flex;
  gap: 8px;
  align-items: center;
}

.summary-label {
  font-weight: 500;
  color: var(--text-secondary);
}

.summary-value {
  font-weight: 600;
  color: var(--text-primary);
}

/* 部署配置步骤 */
.step-deploy-config .deploy-config-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* 部署进度步骤 */
.step-master-deployment, .step-worker-deployment {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

.deployment-progress-container {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 25px;
}

@media (max-width: 1024px) {
  .deployment-progress-container {
    grid-template-columns: 1fr;
  }
}

.master-node-list, .worker-node-list {
  background-color: var(--bg-secondary);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.deployment-node-item {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 15px;
  margin-bottom: 15px;
  transition: all 0.3s ease;
}

.deployment-node-item:last-child {
  margin-bottom: 0;
}

.deployment-node-item.deployed {
  border-color: var(--success-color);
  background-color: rgba(39, 174, 96, 0.05);
}

.deployment-node-item.failed {
  border-color: var(--error-color);
  background-color: rgba(231, 76, 60, 0.05);
}

.node-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.node-name {
  font-weight: 600;
  font-size: 0.95rem;
}

.deployment-status {
  font-size: 0.85rem;
  font-weight: 600;
  padding: 4px 12px;
  border-radius: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.deployment-status:empty {
  display: none;
}

.node-progress-bar {
  background-color: var(--bg-input);
  border-radius: var(--radius-sm);
  height: 8px;
  overflow: hidden;
  margin-top: 10px;
}

.progress-bar {
  height: 100%;
  background-color: var(--primary-color);
  border-radius: var(--radius-sm);
  transition: width 0.3s ease;
}

.progress-bar.failed {
  background-color: var(--error-color);
}

.deployment-logs {
  background-color: var(--bg-secondary);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.deployment-logs h4 {
  margin: 0 0 15px 0;
  font-size: 1rem;
  font-weight: 600;
}

.logs-container {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  max-height: 400px;
  overflow-y: auto;
  padding: 15px;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.9rem;
  line-height: 1.6;
}

.logs-container pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}

/* 部署完成步骤 */
.step-completion .completion-summary {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.summary-card {
  background-color: var(--bg-secondary);
  padding: 25px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.summary-card h4 {
  margin: 0 0 20px 0;
  font-size: 1.1rem;
  font-weight: 600;
}

.summary-card.success {
  border-color: var(--success-color);
  background-color: rgba(39, 174, 96, 0.05);
}

.summary-card.info {
  border-color: var(--primary-color);
  background-color: rgba(52, 152, 219, 0.05);
}

.summary-card.warning {
  border-color: var(--warning-color);
  background-color: rgba(241, 196, 15, 0.05);
}

.summary-stats, .cluster-info {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.stat-item, .info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 15px;
  background-color: var(--bg-card);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
}

.stat-label, .info-label {
  font-weight: 500;
  color: var(--text-secondary);
}

.stat-value, .info-value {
  font-weight: 600;
  color: var(--text-primary);
}

.stat-value.success {
  color: var(--success-color);
}

.next-steps {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.next-steps li {
  padding: 12px 15px;
  background-color: var(--bg-card);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
  font-size: 0.95rem;
  position: relative;
  padding-left: 30px;
}

.next-steps li::before {
  content: '→';
  position: absolute;
  left: 10px;
  color: var(--primary-color);
  font-weight: bold;
}

/* 步骤导航按钮 */
.step-navigation {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 20px;
  border-top: 1px solid var(--border-color);
}

/* 表单样式 */
.form-row {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}

.form-row .form-group {
  flex: 1;
  min-width: 250px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 15px;
}

.form-group label {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

.form-group input,
.form-group select {
  padding: 12px 15px;
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 0.95rem;
  transition: all 0.3s ease;
  font-family: inherit;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-weight: 500;
  color: var(--text-primary);
  user-select: none;
}

.checkbox-label input[type="checkbox"] {
  width: 18px;
  height: 18px;
  accent-color: var(--primary-color);
  cursor: pointer;
}

/* 按钮样式 */
.btn {
  padding: 12px 24px;
  border: none;
  border-radius: var(--radius-sm);
  font-size: 0.95rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s ease;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-family: inherit;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
}

.btn-primary {
  background-color: var(--primary-color);
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background-color: var(--primary-dark);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(52, 152, 219, 0.3);
}

.btn-secondary {
  background-color: var(--bg-input);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

.btn-secondary:hover:not(:disabled) {
  background-color: var(--border-color);
  border-color: var(--border-light);
  transform: translateY(-1px);
}

.btn-success {
  background-color: var(--success-color);
  color: white;
}

.btn-success:hover:not(:disabled) {
  background-color: var(--success-dark);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(39, 174, 96, 0.3);
}

/* 加载动画 */
.loading-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top: 2px solid white;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* 节点配置预览 */
.node-configuration-summary {
  margin-top: 25px;
  padding-top: 25px;
  border-top: 1px solid var(--border-color);
}

.node-configuration-summary h4 {
  margin: 0 0 20px 0;
  font-size: 1rem;
  font-weight: 600;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 20px;
}

.summary-section {
  background-color: var(--bg-secondary);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.summary-section h5 {
  margin: 0 0 15px 0;
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-primary);
}

.preview-node {
  background-color: var(--bg-card);
  padding: 12px 15px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
  margin-bottom: 10px;
  font-size: 0.9rem;
}

.preview-node:last-child {
  margin-bottom: 0;
}
</style>