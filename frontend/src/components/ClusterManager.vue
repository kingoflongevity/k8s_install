<template>
  <div class="cluster-manager">
    <!-- 集群管理区域 -->
    <div class="cluster-section">
      <h3>集群初始化</h3>
      <div class="cluster-form-container">
        <form class="cluster-form" @submit.prevent="initCluster">
          <!-- 基本配置 -->
          <div class="form-section">
            <h4>基本配置</h4>
            <div class="form-row">
              <div class="form-group">
                <label for="advertiseAddress">广告地址 (Advertise Address):</label>
                <input 
                  type="text" 
                  id="advertiseAddress" 
                  v-model="config.advertiseAddress" 
                  placeholder="192.168.1.100" 
                  required
                >
              </div>
              <div class="form-group">
                <label for="kubernetesVersion">Kubernetes 版本:</label>
                <input 
                  type="text" 
                  id="kubernetesVersion" 
                  v-model="config.kubernetesVersion" 
                  placeholder="v1.30.0" 
                  required
                >
              </div>
            </div>
            
            <!-- 生产环境：控制平面端点，用于高可用性 -->
            <div class="form-row">
              <div class="form-group">
                <label for="controlPlaneEndpoint">控制平面端点 (Control Plane Endpoint):</label>
                <input 
                  type="text" 
                  id="controlPlaneEndpoint" 
                  v-model="config.controlPlaneEndpoint" 
                  placeholder="kube-api.example.com:6443" 
                  title="用于高可用性集群的负载均衡器地址"
                >
              </div>
            </div>
          </div>
          
          <!-- 网络配置 -->
          <div class="form-section">
            <h4>网络配置</h4>
            <div class="form-row">
              <div class="form-group">
                <label for="podSubnet">Pod 子网:</label>
                <input 
                  type="text" 
                  id="podSubnet" 
                  v-model="config.podSubnet" 
                  placeholder="10.244.0.0/16" 
                  required
                >
              </div>
              <div class="form-group">
                <label for="serviceSubnet">Service 子网:</label>
                <input 
                  type="text" 
                  id="serviceSubnet" 
                  v-model="config.serviceSubnet" 
                  placeholder="10.96.0.0/12" 
                  required
                >
              </div>
            </div>
            
            <div class="form-row">
              <div class="form-group">
                <label for="dnsDomain">DNS 域名:</label>
                <input 
                  type="text" 
                  id="dnsDomain" 
                  v-model="config.dnsDomain" 
                  placeholder="cluster.local" 
                  required
                >
              </div>
              <div class="form-group">
                <label for="serviceNodePortRange">Service NodePort 范围:</label>
                <input 
                  type="text" 
                  id="serviceNodePortRange" 
                  v-model="config.serviceNodePortRange" 
                  placeholder="30000-32767"
                >
              </div>
            </div>
          </div>
          
          <!-- 生产环境：节点注册配置 -->
          <div class="form-section">
            <h4>节点注册配置</h4>
            <div class="form-row">
              <div class="form-group">
                <label for="nodeName">节点名称:</label>
                <input 
                  type="text" 
                  id="nodeName" 
                  v-model="config.nodeName" 
                  placeholder="自动生成"
                >
              </div>
              <div class="form-group">
                <label for="criSocket">CRI Socket:</label>
                <input 
                  type="text" 
                  id="criSocket" 
                  v-model="config.criSocket" 
                  placeholder="/run/containerd/containerd.sock"
                >
              </div>
            </div>
          </div>
          
          <!-- 生产环境：API服务器配置 -->
          <div class="form-section">
            <h4>API服务器配置</h4>
            <div class="form-row">
              <div class="form-group">
                <label for="timeoutForControlPlane">控制平面超时时间 (秒):</label>
                <input 
                  type="number" 
                  id="timeoutForControlPlane" 
                  v-model.number="config.timeoutForControlPlane" 
                  placeholder="300"
                  min="30"
                >
              </div>
            </div>
          </div>
          
          <!-- 生产环境：etcd配置 -->
          <div class="form-section">
            <h4>etcd配置</h4>
            <div class="form-row">
              <div class="form-group">
                <label for="etcdDataDir">etcd 数据目录:</label>
                <input 
                  type="text" 
                  id="etcdDataDir" 
                  v-model="config.etcdDataDir" 
                  placeholder="/var/lib/etcd"
                >
              </div>
            </div>
          </div>
          
          <div class="form-actions">
            <button type="submit" class="btn btn-primary" :disabled="isDeploying">
              <span v-if="isDeploying" class="loading-spinner"></span>
              {{ isDeploying ? '初始化中...' : '初始化集群' }}
            </button>
            <button 
              type="button" 
              class="btn btn-danger" 
              @click="resetCluster"
              :disabled="isDeploying"
            >
              <span v-if="isDeploying" class="loading-spinner"></span>
              {{ isDeploying ? '重置中...' : '重置集群' }}
            </button>
          </div>
        </form>
        
        <!-- 加入命令区域 -->
        <div class="join-command-section">
          <h3>加入集群命令</h3>
          <div v-if="joinCommand" class="join-command-box">
            <pre>{{ joinCommand }}</pre>
            <button class="btn btn-secondary" @click="copyJoinCommand">复制命令</button>
          </div>
          <div v-else class="no-command">
            <p>集群初始化后，加入命令将显示在此处</p>
          </div>
        </div>
      </div>
    </div>
    
    <!-- 部署日志区域 -->
    <div v-if="deployLogs" class="logs-section">
      <h3>部署日志</h3>
      <div class="logs-container">
        <pre>{{ deployLogs }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onActivated, onDeactivated } from 'vue'
import axios from 'axios'

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 300000 // 5分钟超时，适应Kubernetes组件安装的耗时过程
})

// 状态变量
const isDeploying = ref(false)
const deployLogs = ref('')
const joinCommand = ref('')
let logInterval = null

// 部署配置
const config = ref({
  advertiseAddress: '',
  kubernetesVersion: 'v1.30.0',
  controlPlaneEndpoint: '', // 生产环境：用于高可用性
  podSubnet: '10.244.0.0/16',
  serviceSubnet: '10.96.0.0/12',
  dnsDomain: 'cluster.local',
  serviceNodePortRange: '30000-32767', // 生产环境：Service NodePort范围
  nodeName: '', // 生产环境：节点名称
  criSocket: '/run/containerd/containerd.sock', // 生产环境：CRI Socket
  timeoutForControlPlane: 300, // 生产环境：API服务器超时时间
  etcdDataDir: '/var/lib/etcd' // 生产环境：etcd数据目录
})

// 定义组件的事件
const emit = defineEmits(['showMessage'])

// 初始化集群
const initCluster = async () => {
  isDeploying.value = true
  deployLogs.value = ''
  joinCommand.value = ''

  try {
    // 获取主节点ID（这里假设第一个节点是主节点）
    const nodesResponse = await apiClient.get('/nodes')
    const masterNodes = nodesResponse.data.filter(node => node.nodeType === 'master' || node.nodeType === 'Master')
    
    if (masterNodes.length === 0) {
      throw new Error('没有找到主节点，请先添加主节点并设置为主节点类型')
    }
    
    const masterNodeId = masterNodes[0].id
    
    const kubeadmConfig = {
      masterNodeId: masterNodeId,
      config: {
        apiVersion: 'kubeadm.k8s.io/v1beta3',
        kind: 'InitConfiguration',
        initConfiguration: {
          localAPIEndpoint: {
            advertiseAddress: config.value.advertiseAddress,
            bindPort: 6443
          },
          nodeRegistration: {
            name: config.value.nodeName,
            criSocket: config.value.criSocket
          }
        },
        clusterConfiguration: {
          kubernetesVersion: config.value.kubernetesVersion,
          controlPlaneEndpoint: config.value.controlPlaneEndpoint,
          networking: {
            podSubnet: config.value.podSubnet,
            serviceSubnet: config.value.serviceSubnet,
            dnsDomain: config.value.dnsDomain,
            serviceNodePortRange: config.value.serviceNodePortRange
          },
          api: {
            timeoutForControlPlane: config.value.timeoutForControlPlane
          },
          etcd: {
            local: {
              dataDir: config.value.etcdDataDir
            }
          }
        }
      }
    }

    const response = await apiClient.post('/kubeadm/init', kubeadmConfig)
    deployLogs.value = response.data.result
    emit('showMessage', { text: '集群初始化成功!', type: 'success' })

    // 获取加入命令
    await getJoinCommand(masterNodeId)
  } catch (error) {
    deployLogs.value = error.response?.data?.error || error.message
    emit('showMessage', { text: '集群初始化失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 重置集群
const resetCluster = async () => {
  if (!confirm('确定要重置集群吗? 此操作将删除所有 Kubernetes 资源!')) {
    return
  }

  isDeploying.value = true
  deployLogs.value = ''
  joinCommand.value = ''

  try {
    // 获取主节点ID（这里假设第一个节点是主节点）
    const nodesResponse = await apiClient.get('/nodes')
    const masterNodes = nodesResponse.data.filter(node => node.nodeType === 'master' || node.nodeType === 'Master')
    
    if (masterNodes.length === 0) {
      throw new Error('没有找到主节点，请先添加主节点并设置为主节点类型')
    }
    
    const masterNodeId = masterNodes[0].id
    
    const response = await apiClient.post('/kubeadm/reset', {
      masterNodeId: masterNodeId
    })
    deployLogs.value = response.data.result
    emit('showMessage', { text: '集群重置成功!', type: 'success' })
  } catch (error) {
    deployLogs.value = error.response?.data?.error || error.message
    emit('showMessage', { text: '集群重置失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 获取加入命令
const getJoinCommand = async (masterNodeId) => {
  try {
    const response = await apiClient.get('/kubeadm/join-command', {
      params: {
        masterNodeId: masterNodeId
      }
    })
    joinCommand.value = response.data.command
  } catch (error) {
    emit('showMessage', { text: '获取加入命令失败: ' + error.message, type: 'error' })
  }
}

// 复制加入命令
const copyJoinCommand = async () => {
  try {
    await navigator.clipboard.writeText(joinCommand.value)
    emit('showMessage', { text: '命令已复制到剪贴板!', type: 'success' })
  } catch (error) {
    emit('showMessage', { text: '复制失败: ' + error.message, type: 'error' })
  }
}

// 获取并格式化部署日志
const getDeployLogs = async () => {
  try {
    const response = await apiClient.get('/logs')
    if (response.data.logs && response.data.logs.length > 0) {
      // 只显示与Kubernetes部署相关的日志
      const k8sLogs = response.data.logs.filter(log => 
        log.operation.includes('Kubernetes') || 
        log.operation.includes('kubeadm') ||
        log.operation.includes('InstallKubernetesComponents')
      )
      
      if (k8sLogs.length > 0) {
        // 格式化日志
        let logsText = ''
        for (const log of k8sLogs.reverse()) {
          logsText += `=== ${log.nodeName} - ${log.operation} ===\n`
          logsText += `时间: ${new Date(log.createdAt).toLocaleString('zh-CN')}\n`
          logsText += `状态: ${log.status}\n`
          logsText += `命令: ${log.command}\n`
          logsText += `输出: ${log.output}\n\n`
        }
        deployLogs.value = logsText
      }
    }
  } catch (error) {
      // 静默处理获取日志失败
    }
}

// 组件激活时启动日志刷新定时器
onActivated(() => {
  getDeployLogs()
  
  // 每隔1秒刷新一次日志，实现实时效果
  logInterval = setInterval(() => {
    getDeployLogs()
  }, 1000)
})

// 组件停用时清除日志刷新定时器
onDeactivated(() => {
  if (logInterval) {
    clearInterval(logInterval)
    logInterval = null
  }
})
</script>

<style scoped>
/* 集群管理区域 */
.cluster-manager {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

.cluster-section {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

.cluster-section h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 20px 0;
}

/* 集群表单容器 */
.cluster-form-container {
  display: grid;
  grid-template-columns: 1fr 350px;
  gap: 25px;
}

@media (max-width: 1024px) {
  .cluster-form-container {
    grid-template-columns: 1fr;
  }
}

/* 集群表单 */
.cluster-form {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  gap: 15px;
}

/* 加入命令区域 */
.join-command-section {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.join-command-section h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
}

.join-command-box {
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 20px;
  margin-bottom: 15px;
  position: relative;
}

.join-command-box pre {
  margin: 0 0 15px 0;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.9rem;
  line-height: 1.6;
  max-height: 300px;
  overflow-y: auto;
}

.no-command {
  text-align: center;
  color: var(--text-muted);
  padding: 20px;
  font-style: italic;
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
}

/* 部署日志区域 */
.logs-section {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

.logs-section h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 20px 0;
}

.logs-container {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  max-height: 400px;
  overflow-y: auto;
  padding: 20px;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.9rem;
  line-height: 1.6;
}

.logs-container pre {
  margin: 0;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-all;
}

/* 表单章节样式 */
.form-section {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 20px;
  margin-bottom: 20px;
  transition: all 0.3s ease;
}

.form-section:hover {
  box-shadow: var(--shadow-md);
  border-color: var(--primary-light);
}

.form-section h4 {
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid var(--primary-color);
  display: inline-block;
}

/* 表单样式 */
.form-row {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
  margin-bottom: 15px;
}

.form-row:last-child {
  margin-bottom: 0;
}

.form-row .form-group {
  flex: 1;
  min-width: 250px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-group label {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

.form-group input {
  padding: 12px 15px;
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 0.95rem;
  transition: all 0.3s ease;
  font-family: inherit;
}

.form-group input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

/* 表单操作按钮 */
.form-actions {
  display: flex;
  gap: 12px;
  margin-top: 20px;
  flex-wrap: wrap;
  padding-top: 20px;
  border-top: 1px solid var(--border-color);
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

.btn-danger {
  background-color: var(--error-color);
  color: white;
}

.btn-danger:hover:not(:disabled) {
  background-color: #c0392b;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(231, 76, 60, 0.3);
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
</style>