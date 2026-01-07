<template>
  <div class="docker-manager">
    <!-- Docker管理区域 -->
    <div class="docker-section">
      <h3>Docker 容器管理</h3>
      <div class="docker-form-container">
        <!-- 节点选择 -->
        <div class="form-group">
          <label for="nodeSelect">选择节点:</label>
          <div class="node-selection">
            <!-- 单选选择器 -->
            <select 
              id="nodeSelect" 
              v-model="selectedNodeId" 
              class="node-select"
              :disabled="selectedNodes.length > 0"
            >
              <option value="">-- 选择单个节点 --</option>
              <option 
                v-for="node in nodes" 
                :key="node.id" 
                :value="node.id"
              >
                {{ node.name }} ({{ node.ip }})
              </option>
            </select>
            
            <!-- 批量选择区域 -->
            <div class="batch-selection">
              <div class="batch-header">
                <h4>批量操作</h4>
                <div class="batch-select-all">
                  <input 
                    type="checkbox" 
                    id="selectAll" 
                    v-model="selectAllNodes"
                  >
                  <label for="selectAll">全选</label>
                </div>
              </div>
              <div class="batch-nodes-list">
                <div 
                  v-for="node in nodes" 
                  :key="node.id" 
                  class="batch-node-item"
                >
                  <input 
                    type="checkbox" 
                    :id="`node-${node.id}`" 
                    v-model="selectedNodes"
                    :value="node.id"
                  >
                  <label :for="`node-${node.id}`">{{ node.name }} ({{ node.ip }})</label>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <!-- Docker版本选择 -->
        <div class="form-group">
          <label for="dockerVersion">选择Docker版本:</label>
          <select 
            id="dockerVersion" 
            v-model="selectedDockerVersion" 
            class="docker-version-select"
          >
            <option value="">-- 选择版本 (默认安装最新版) --</option>
            <option 
              v-for="version in availableDockerVersions" 
              :key="version" 
              :value="version"
            >
              {{ version }}
            </option>
          </select>
        </div>
        
        <!-- 操作按钮区域 -->
        <div class="docker-actions">
          <button 
            class="btn btn-primary" 
            @click="installDocker"
            :disabled="isDeploying || (selectedNodes.length === 0 && !selectedNodeId)"
          >
            <span v-if="isDeploying" class="loading-spinner"></span>
            {{ isDeploying ? '安装中...' : '安装 Docker' }}
          </button>
          <button 
            class="btn btn-secondary" 
            @click="startDocker"
            :disabled="isDeploying || (selectedNodes.length === 0 && !selectedNodeId)"
          >
            <span v-if="isDeploying" class="loading-spinner"></span>
            {{ isDeploying ? '启动中...' : '启动 Docker' }}
          </button>
          <button 
            class="btn btn-warning" 
            @click="stopDocker"
            :disabled="isDeploying || (selectedNodes.length === 0 && !selectedNodeId)"
          >
            <span v-if="isDeploying" class="loading-spinner"></span>
            {{ isDeploying ? '停止中...' : '停止 Docker' }}
          </button>
          <button 
            class="btn btn-success" 
            @click="enableDocker"
            :disabled="isDeploying || (selectedNodes.length === 0 && !selectedNodeId)"
          >
            <span v-if="isDeploying" class="loading-spinner"></span>
            {{ isDeploying ? '启用中...' : '启用自启' }}
          </button>
          <button 
            class="btn btn-danger" 
            @click="disableDocker"
            :disabled="isDeploying || (selectedNodes.length === 0 && !selectedNodeId)"
          >
            <span v-if="isDeploying" class="loading-spinner"></span>
            {{ isDeploying ? '禁用中...' : '禁用自启' }}
          </button>
          <button 
            class="btn btn-info" 
            @click="checkDockerStatus"
            :disabled="isDeploying || (selectedNodes.length === 0 && !selectedNodeId)"
          >
            <span v-if="isDeploying" class="loading-spinner"></span>
            {{ isDeploying ? '检查中...' : '检查状态' }}
          </button>
          <button 
            class="btn btn-danger" 
            @click="removeDocker"
            :disabled="isDeploying || (selectedNodes.length === 0 && !selectedNodeId)"
          >
            <span v-if="isDeploying" class="loading-spinner"></span>
            {{ isDeploying ? '删除中...' : '删除 Docker' }}
          </button>
        </div>
        
        <!-- Docker配置表单 -->
        <div class="docker-config-section">
          <h4>Docker 配置</h4>
          <form class="docker-config-form" @submit.prevent="configureDocker">
            <div class="form-row">
              <div class="form-group">
                <label for="registryMirrors">镜像加速地址:</label>
                <input 
                  type="text" 
                  id="registryMirrors" 
                  v-model="dockerConfig.registryMirrorsInput" 
                  placeholder="https://registry.docker-cn.com,https://mirror.aliyuncs.com"
                >
                <small>多个地址用逗号分隔</small>
              </div>
            </div>
            
            <div class="form-row">
              <div class="form-group">
                <label for="dataRoot">数据存储目录:</label>
                <input 
                  type="text" 
                  id="dataRoot" 
                  v-model="dockerConfig.dataRoot" 
                  placeholder="/var/lib/docker"
                >
              </div>
              <div class="form-group">
                <label for="storageDriver">存储驱动:</label>
                <select id="storageDriver" v-model="dockerConfig.storageDriver">
                  <option value="overlay2">overlay2</option>
                  <option value="aufs">aufs</option>
                  <option value="devicemapper">devicemapper</option>
                  <option value="btrfs">btrfs</option>
                  <option value="zfs">zfs</option>
                </select>
              </div>
            </div>
            
            <div class="form-row">
              <div class="form-group">
                <label for="logDriver">日志驱动:</label>
                <select id="logDriver" v-model="dockerConfig.logDriver">
                  <option value="json-file">json-file</option>
                  <option value="syslog">syslog</option>
                  <option value="journald">journald</option>
                  <option value="gelf">gelf</option>
                  <option value="fluentd">fluentd</option>
                </select>
              </div>
              <div class="form-group">
                <label for="logMaxSize">日志文件大小限制:</label>
                <input 
                  type="text" 
                  id="logMaxSize" 
                  v-model="dockerConfig.logMaxSize" 
                  placeholder="100m"
                >
              </div>
              <div class="form-group">
                <label for="logMaxFile">日志文件数量限制:</label>
                <input 
                  type="number" 
                  id="logMaxFile" 
                  v-model.number="dockerConfig.logMaxFile" 
                  placeholder="3"
                  min="1"
                >
              </div>
            </div>
            
            <div class="form-row">
              <div class="form-group">
                <label for="cgroupDriver">Cgroup驱动:</label>
                <select id="cgroupDriver" v-model="dockerConfig.cgroupDriver">
                  <option value="systemd">systemd (推荐)</option>
                  <option value="cgroupfs">cgroupfs</option>
                </select>
              </div>
            </div>
            
            <div class="form-actions">
              <button 
                type="submit" 
                class="btn btn-primary" 
                :disabled="isDeploying || (selectedNodes.length === 0 && !selectedNodeId)"
              >
                <span v-if="isDeploying" class="loading-spinner"></span>
                {{ isDeploying ? '配置中...' : '应用配置' }}
              </button>
            </div>
          </form>
        </div>
        
        <!-- Docker状态显示 -->
        <div v-if="dockerStatus" class="docker-status-section">
          <h4>Docker 状态</h4>
          <div class="status-box" :class="dockerStatusClass">
            <span class="status-icon">{{ dockerStatusIcon }}</span>
            <span class="status-text">{{ dockerStatus }}</span>
          </div>
        </div>
      </div>
    </div>
    
    <!-- 部署日志区域 -->
    <div v-if="deployLogs" class="logs-section">
      <h3>操作日志</h3>
      <div class="logs-container">
        <pre>{{ deployLogs }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import axios from 'axios'

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 60000 // 60秒超时
})

// 状态变量
const isDeploying = ref(false)
const deployLogs = ref('')
const dockerStatus = ref('')
const selectedNodeId = ref('')
const selectedNodes = ref([])
const selectAllNodes = ref(false)
const nodes = ref([])

// 监听全选状态变化
watch(() => selectAllNodes.value, (newVal) => {
  if (newVal) {
    selectedNodes.value = nodes.value.map(node => node.id)
  } else {
    selectedNodes.value = []
  }
})

// 监听选中节点变化，更新全选状态
watch(() => selectedNodes.value.length, (newVal) => {
  if (nodes.value.length > 0) {
    selectAllNodes.value = newVal === nodes.value.length
  }
})

// Docker配置
const dockerConfig = ref({
  registryMirrorsInput: '',
  registryMirrors: [],
  dataRoot: '/var/lib/docker',
  storageDriver: 'overlay2',
  logDriver: 'json-file',
  logMaxSize: '100m',
  logMaxFile: 3,
  cgroupDriver: 'systemd'
})

// Docker版本相关
const availableDockerVersions = ref([])
const selectedDockerVersion = ref('')

// 获取可用的Docker版本
const getDockerVersions = async () => {
  try {
    const response = await apiClient.get('/docker/packages')
    if (response.data.versions && Array.isArray(response.data.versions)) {
      availableDockerVersions.value = response.data.versions
    }
  } catch (error) {
    emit('showMessage', { text: '获取Docker版本列表失败: ' + error.message, type: 'error' })
  }
}

// 定义组件的事件
const emit = defineEmits(['showMessage'])

// 计算属性：Docker状态样式类
const dockerStatusClass = computed(() => {
  if (dockerStatus.value === 'running') return 'status-running'
  if (dockerStatus.value === 'enabled but not running') return 'status-enabled'
  return 'status-stopped'
})

// 计算属性：Docker状态图标
const dockerStatusIcon = computed(() => {
  if (dockerStatus.value === 'running') return '✅'
  if (dockerStatus.value === 'enabled but not running') return '⚠️'
  return '❌'
})

// 获取节点列表
const getNodes = async () => {
  try {
    const response = await apiClient.get('/nodes')
    nodes.value = response.data
  } catch (error) {
    emit('showMessage', { text: '获取节点列表失败: ' + error.message, type: 'error' })
    nodes.value = []
  }
}

// 安装Docker
const installDocker = async () => {
  // 确定要操作的节点列表
  const targetNodes = selectedNodes.value.length > 0 ? selectedNodes.value : (selectedNodeId.value ? [selectedNodeId.value] : [])
  
  if (targetNodes.length === 0) {
    emit('showMessage', { text: '请先选择节点', type: 'warning' })
    return
  }

  isDeploying.value = true
  deployLogs.value = ''
  
  // 启动轮询，获取最新日志
  const pollInterval = setInterval(async () => {
    try {
      const response = await apiClient.get('/logs')
      if (response.data.logs && response.data.logs.length > 0) {
        // 只显示与当前操作相关的日志
        const operationLogs = response.data.logs.filter(log => 
          log.operation === 'InstallDocker' && 
          targetNodes.includes(log.nodeId)
        )
        
        if (operationLogs.length > 0) {
          // 格式化日志
          let logsText = ''
          for (const log of operationLogs) {
            logsText += `=== ${log.nodeName} ===\n`
            logsText += `时间: ${formatDate(log.createdAt)}\n`
            logsText += `状态: ${log.status}\n`
            logsText += `命令: ${log.command}\n`
            logsText += `输出: ${log.output}\n\n`
          }
          deployLogs.value = logsText
        }
      }
    } catch (error) {
      console.error('获取日志失败:', error)
    }
  }, 1000) // 每秒轮询一次

  try {
    let results = ''
    
    // 如果是批量操作，调用批量安装API
      if (targetNodes.length > 1) {
        const response = await apiClient.post('/nodes/docker/batch-install', { 
          nodeIds: targetNodes, 
          version: selectedDockerVersion 
        })
        results = response.data.results
      } else {
        // 单个节点操作，保持原有API调用
        const response = await apiClient.post(`/nodes/${targetNodes[0]}/docker/install`, {
          version: selectedDockerVersion
        })
        results = response.data.status
        // 检查安装后的状态
        await checkDockerStatus()
      }
    
    // 停止轮询
    clearInterval(pollInterval)
    
    // 最后更新一次日志
    const response = await apiClient.get('/logs')
    if (response.data.logs && response.data.logs.length > 0) {
      const operationLogs = response.data.logs.filter(log => 
        log.operation === 'InstallDocker' && 
        targetNodes.includes(log.nodeId)
      )
      
      if (operationLogs.length > 0) {
        let logsText = ''
        for (const log of operationLogs) {
          logsText += `=== ${log.nodeName} ===\n`
          logsText += `时间: ${formatDate(log.createdAt)}\n`
          logsText += `状态: ${log.status}\n`
          logsText += `命令: ${log.command}\n`
          logsText += `输出: ${log.output}\n\n`
        }
        deployLogs.value = logsText
      } else {
        deployLogs.value = results
      }
    }
    
    emit('showMessage', { text: `Docker安装成功! 共安装了 ${targetNodes.length} 个节点`, type: 'success' })
  } catch (error) {
    // 停止轮询
    clearInterval(pollInterval)
    
    // 记录错误日志
    deployLogs.value = error.response?.data?.error || error.message
    emit('showMessage', { text: 'Docker安装失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 配置Docker
const configureDocker = async () => {
  // 确定要操作的节点列表
  const targetNodes = selectedNodes.value.length > 0 ? selectedNodes.value : (selectedNodeId.value ? [selectedNodeId.value] : [])
  
  if (targetNodes.length === 0) {
    emit('showMessage', { text: '请先选择节点', type: 'warning' })
    return
  }

  // 处理镜像加速地址，转换为数组
  dockerConfig.value.registryMirrors = dockerConfig.value.registryMirrorsInput
    .split(',')
    .map(mirror => mirror.trim())
    .filter(mirror => mirror)

  isDeploying.value = true
  deployLogs.value = ''
  
  // 启动轮询，获取最新日志
  const pollInterval = setInterval(async () => {
    try {
      const response = await apiClient.get('/logs')
      if (response.data.logs && response.data.logs.length > 0) {
        // 只显示与当前操作相关的日志
        const operationLogs = response.data.logs.filter(log => 
          log.operation === 'ConfigureDocker' && 
          targetNodes.includes(log.nodeId)
        )
        
        if (operationLogs.length > 0) {
          // 格式化日志
          let logsText = ''
          for (const log of operationLogs) {
            logsText += `=== ${log.nodeName} ===\n`
            logsText += `时间: ${formatDate(log.createdAt)}\n`
            logsText += `状态: ${log.status}\n`
            logsText += `命令: ${log.command}\n`
            logsText += `输出: ${log.output}\n\n`
          }
          deployLogs.value = logsText
        }
      }
    } catch (error) {
      console.error('获取日志失败:', error)
    }
  }, 1000) // 每秒轮询一次

  try {
    let results = ''
    
    // 如果是批量操作，调用批量配置API
    if (targetNodes.length > 1) {
      const response = await apiClient.post('/nodes/docker/batch-configure', { 
        nodeIds: targetNodes, 
        config: dockerConfig.value 
      })
      results = response.data.results
    } else {
      // 单个节点操作，保持原有API调用
      const response = await apiClient.post(`/nodes/${targetNodes[0]}/docker/configure`, dockerConfig.value)
      results = response.data.status
      // 检查配置后的状态
      await checkDockerStatus()
    }
    
    // 停止轮询
    clearInterval(pollInterval)
    
    // 最后更新一次日志
    const response = await apiClient.get('/logs')
    if (response.data.logs && response.data.logs.length > 0) {
      const operationLogs = response.data.logs.filter(log => 
        log.operation === 'ConfigureDocker' && 
        targetNodes.includes(log.nodeId)
      )
      
      if (operationLogs.length > 0) {
        let logsText = ''
        for (const log of operationLogs) {
          logsText += `=== ${log.nodeName} ===\n`
          logsText += `时间: ${formatDate(log.createdAt)}\n`
          logsText += `状态: ${log.status}\n`
          logsText += `命令: ${log.command}\n`
          logsText += `输出: ${log.output}\n\n`
        }
        deployLogs.value = logsText
      } else {
        deployLogs.value = results
      }
    }
    
    emit('showMessage', { text: `Docker配置成功! 共配置了 ${targetNodes.length} 个节点`, type: 'success' })
  } catch (error) {
    // 停止轮询
    clearInterval(pollInterval)
    
    // 记录错误日志
    deployLogs.value = error.response?.data?.error || error.message
    emit('showMessage', { text: 'Docker配置失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 启动Docker
const startDocker = async () => {
  // 确定要操作的节点列表
  const targetNodes = selectedNodes.value.length > 0 ? selectedNodes.value : (selectedNodeId.value ? [selectedNodeId.value] : [])
  
  if (targetNodes.length === 0) {
    emit('showMessage', { text: '请先选择节点', type: 'warning' })
    return
  }

  isDeploying.value = true
  deployLogs.value = ''
  
  // 启动轮询，获取最新日志
  const pollInterval = setInterval(async () => {
    try {
      const response = await apiClient.get('/logs')
      if (response.data.logs && response.data.logs.length > 0) {
        // 只显示与当前操作相关的日志
        const operationLogs = response.data.logs.filter(log => 
          log.operation === 'StartDocker' && 
          targetNodes.includes(log.nodeId)
        )
        
        if (operationLogs.length > 0) {
          // 格式化日志
          let logsText = ''
          for (const log of operationLogs) {
            logsText += `=== ${log.nodeName} ===\n`
            logsText += `时间: ${formatDate(log.createdAt)}\n`
            logsText += `状态: ${log.status}\n`
            logsText += `命令: ${log.command}\n`
            logsText += `输出: ${log.output}\n\n`
          }
          deployLogs.value = logsText
        }
      }
    } catch (error) {
      console.error('获取日志失败:', error)
    }
  }, 1000) // 每秒轮询一次

  try {
    let results = ''
    
    // 如果是批量操作，调用批量启动API
    if (targetNodes.length > 1) {
      const response = await apiClient.post('/nodes/docker/batch-start', { nodeIds: targetNodes })
      results = response.data.results
    } else {
      // 单个节点操作，保持原有API调用
      const response = await apiClient.post(`/nodes/${targetNodes[0]}/docker/start`)
      results = response.data.status
      // 检查启动后的状态
      await checkDockerStatus()
    }
    
    // 停止轮询
    clearInterval(pollInterval)
    
    // 最后更新一次日志
    const response = await apiClient.get('/logs')
    if (response.data.logs && response.data.logs.length > 0) {
      const operationLogs = response.data.logs.filter(log => 
        log.operation === 'StartDocker' && 
        targetNodes.includes(log.nodeId)
      )
      
      if (operationLogs.length > 0) {
        let logsText = ''
        for (const log of operationLogs) {
          logsText += `=== ${log.nodeName} ===\n`
          logsText += `时间: ${formatDate(log.createdAt)}\n`
          logsText += `状态: ${log.status}\n`
          logsText += `命令: ${log.command}\n`
          logsText += `输出: ${log.output}\n\n`
        }
        deployLogs.value = logsText
      } else {
        deployLogs.value = results
      }
    }
    
    emit('showMessage', { text: `Docker启动成功! 共启动了 ${targetNodes.length} 个节点`, type: 'success' })
  } catch (error) {
    // 停止轮询
    clearInterval(pollInterval)
    
    // 记录错误日志
    deployLogs.value = error.response?.data?.error || error.message
    emit('showMessage', { text: 'Docker启动失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 停止Docker
const stopDocker = async () => {
  // 确定要操作的节点列表
  const targetNodes = selectedNodes.value.length > 0 ? selectedNodes.value : (selectedNodeId.value ? [selectedNodeId.value] : [])
  
  if (targetNodes.length === 0) {
    emit('showMessage', { text: '请先选择节点', type: 'warning' })
    return
  }

  isDeploying.value = true
  deployLogs.value = ''
  
  // 启动轮询，获取最新日志
  const pollInterval = setInterval(async () => {
    try {
      const response = await apiClient.get('/logs')
      if (response.data.logs && response.data.logs.length > 0) {
        // 只显示与当前操作相关的日志
        const operationLogs = response.data.logs.filter(log => 
          log.operation === 'StopDocker' && 
          targetNodes.includes(log.nodeId)
        )
        
        if (operationLogs.length > 0) {
          // 格式化日志
          let logsText = ''
          for (const log of operationLogs) {
            logsText += `=== ${log.nodeName} ===\n`
            logsText += `时间: ${formatDate(log.createdAt)}\n`
            logsText += `状态: ${log.status}\n`
            logsText += `命令: ${log.command}\n`
            logsText += `输出: ${log.output}\n\n`
          }
          deployLogs.value = logsText
        }
      }
    } catch (error) {
      console.error('获取日志失败:', error)
    }
  }, 1000) // 每秒轮询一次

  try {
    let results = ''
    
    // 如果是批量操作，调用批量停止API
    if (targetNodes.length > 1) {
      const response = await apiClient.post('/nodes/docker/batch-stop', { nodeIds: targetNodes })
      results = response.data.results
    } else {
      // 单个节点操作，保持原有API调用
      const response = await apiClient.post(`/nodes/${targetNodes[0]}/docker/stop`)
      results = response.data.status
      // 检查停止后的状态
      await checkDockerStatus()
    }
    
    // 停止轮询
    clearInterval(pollInterval)
    
    // 最后更新一次日志
    const response = await apiClient.get('/logs')
    if (response.data.logs && response.data.logs.length > 0) {
      const operationLogs = response.data.logs.filter(log => 
        log.operation === 'StopDocker' && 
        targetNodes.includes(log.nodeId)
      )
      
      if (operationLogs.length > 0) {
        let logsText = ''
        for (const log of operationLogs) {
          logsText += `=== ${log.nodeName} ===\n`
          logsText += `时间: ${formatDate(log.createdAt)}\n`
          logsText += `状态: ${log.status}\n`
          logsText += `命令: ${log.command}\n`
          logsText += `输出: ${log.output}\n\n`
        }
        deployLogs.value = logsText
      } else {
        deployLogs.value = results
      }
    }
    
    emit('showMessage', { text: `Docker停止成功! 共停止了 ${targetNodes.length} 个节点`, type: 'success' })
  } catch (error) {
    // 停止轮询
    clearInterval(pollInterval)
    
    // 记录错误日志
    deployLogs.value = error.response?.data?.error || error.message
    emit('showMessage', { text: 'Docker停止失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 检查Docker状态
const checkDockerStatus = async () => {
  // 确定要操作的节点列表
  const targetNodes = selectedNodes.value.length > 0 ? selectedNodes.value : (selectedNodeId.value ? [selectedNodeId.value] : [])
  
  if (targetNodes.length === 0) {
    emit('showMessage', { text: '请先选择节点', type: 'warning' })
    return
  }

  isDeploying.value = true
  deployLogs.value = ''
  
  // 启动轮询，获取最新日志
  const pollInterval = setInterval(async () => {
    try {
      const response = await apiClient.get('/logs')
      if (response.data.logs && response.data.logs.length > 0) {
        // 只显示与当前操作相关的日志
        const operationLogs = response.data.logs.filter(log => 
          log.operation === 'CheckDockerStatus' && 
          targetNodes.includes(log.nodeId)
        )
        
        if (operationLogs.length > 0) {
          // 格式化日志
          let logsText = ''
          for (const log of operationLogs) {
            logsText += `=== ${log.nodeName} ===\n`
            logsText += `时间: ${formatDate(log.createdAt)}\n`
            logsText += `状态: ${log.status}\n`
            logsText += `命令: ${log.command}\n`
            logsText += `输出: ${log.output}\n\n`
          }
          deployLogs.value = logsText
        }
      }
    } catch (error) {
      console.error('获取日志失败:', error)
    }
  }, 1000) // 每秒轮询一次

  try {
    if (targetNodes.length > 1) {
      // 批量检查状态
      const response = await apiClient.post('/nodes/docker/batch-status', { nodeIds: targetNodes })
      const statusMap = response.data.status
      let statusText = '各节点Docker状态:\n'
      for (const [nodeId, status] of Object.entries(statusMap)) {
        const node = nodes.value.find(n => n.id === nodeId)
        statusText += `${node?.name || nodeId}: ${status}\n`
      }
      deployLogs.value = statusText
      emit('showMessage', { text: `已获取 ${targetNodes.length} 个节点的Docker状态`, type: 'info' })
    } else {
      // 单个节点操作，保持原有API调用
      const response = await apiClient.get(`/nodes/${targetNodes[0]}/docker/status`)
      dockerStatus.value = response.data.status
      emit('showMessage', { text: `Docker状态: ${response.data.status}`, type: 'info' })
    }
    
    // 停止轮询
    clearInterval(pollInterval)
    
    // 最后更新一次日志
    const response = await apiClient.get('/logs')
    if (response.data.logs && response.data.logs.length > 0) {
      const operationLogs = response.data.logs.filter(log => 
        log.operation === 'CheckDockerStatus' && 
        targetNodes.includes(log.nodeId)
      )
      
      if (operationLogs.length > 0) {
        let logsText = ''
        for (const log of operationLogs) {
          logsText += `=== ${log.nodeName} ===\n`
          logsText += `时间: ${formatDate(log.createdAt)}\n`
          logsText += `状态: ${log.status}\n`
          logsText += `命令: ${log.command}\n`
          logsText += `输出: ${log.output}\n\n`
        }
        deployLogs.value = logsText
      }
    }
  } catch (error) {
    // 停止轮询
    clearInterval(pollInterval)
    
    emit('showMessage', { text: '获取Docker状态失败: ' + error.message, type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 启用Docker开机自启
const enableDocker = async () => {
  // 确定要操作的节点列表
  const targetNodes = selectedNodes.value.length > 0 ? selectedNodes.value : (selectedNodeId.value ? [selectedNodeId.value] : [])
  
  if (targetNodes.length === 0) {
    emit('showMessage', { text: '请先选择节点', type: 'warning' })
    return
  }

  isDeploying.value = true
  deployLogs.value = ''
  
  // 启动轮询，获取最新日志
  const pollInterval = setInterval(async () => {
    try {
      const response = await apiClient.get('/logs')
      if (response.data.logs && response.data.logs.length > 0) {
        // 只显示与当前操作相关的日志
        const operationLogs = response.data.logs.filter(log => 
          log.operation === 'EnableDocker' && 
          targetNodes.includes(log.nodeId)
        )
        
        if (operationLogs.length > 0) {
          // 格式化日志
          let logsText = ''
          for (const log of operationLogs) {
            logsText += `=== ${log.nodeName} ===\n`
            logsText += `时间: ${formatDate(log.createdAt)}\n`
            logsText += `状态: ${log.status}\n`
            logsText += `命令: ${log.command}\n`
            logsText += `输出: ${log.output}\n\n`
          }
          deployLogs.value = logsText
        }
      }
    } catch (error) {
      console.error('获取日志失败:', error)
    }
  }, 1000) // 每秒轮询一次

  try {
    let results = ''
  
    // 如果是批量操作，调用批量启用API
    if (targetNodes.length > 1) {
      const response = await apiClient.post('/nodes/docker/batch-enable', { nodeIds: targetNodes })
      results = response.data.results
    } else {
      // 单个节点操作
      const response = await apiClient.post(`/nodes/${targetNodes[0]}/docker/enable`)
      results = response.data.status
      // 检查启用后的状态
      await checkDockerStatus()
    }
  
    // 停止轮询
    clearInterval(pollInterval)
  
    // 最后更新一次日志
    const response = await apiClient.get('/logs')
    if (response.data.logs && response.data.logs.length > 0) {
      const operationLogs = response.data.logs.filter(log => 
        log.operation === 'EnableDocker' && 
        targetNodes.includes(log.nodeId)
      )
      
      if (operationLogs.length > 0) {
        let logsText = ''
        for (const log of operationLogs) {
          logsText += `=== ${log.nodeName} ===\n`
          logsText += `时间: ${formatDate(log.createdAt)}\n`
          logsText += `状态: ${log.status}\n`
          logsText += `命令: ${log.command}\n`
          logsText += `输出: ${log.output}\n\n`
        }
        deployLogs.value = logsText
      } else {
        deployLogs.value = results
      }
    }
  
    emit('showMessage', { text: `Docker自启启用成功! 共启用了 ${targetNodes.length} 个节点`, type: 'success' })
  } catch (error) {
    // 停止轮询
    clearInterval(pollInterval)
  
    // 记录错误日志
    deployLogs.value = error.response?.data?.error || error.message
    emit('showMessage', { text: 'Docker自启启用失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 禁用Docker开机自启
const disableDocker = async () => {
  // 确定要操作的节点列表
  const targetNodes = selectedNodes.value.length > 0 ? selectedNodes.value : (selectedNodeId.value ? [selectedNodeId.value] : [])
  
  if (targetNodes.length === 0) {
    emit('showMessage', { text: '请先选择节点', type: 'warning' })
    return
  }

  isDeploying.value = true
  deployLogs.value = ''
  
  // 启动轮询，获取最新日志
  const pollInterval = setInterval(async () => {
    try {
      const response = await apiClient.get('/logs')
      if (response.data.logs && response.data.logs.length > 0) {
        // 只显示与当前操作相关的日志
        const operationLogs = response.data.logs.filter(log => 
          log.operation === 'DisableDocker' && 
          targetNodes.includes(log.nodeId)
        )
        
        if (operationLogs.length > 0) {
          // 格式化日志
          let logsText = ''
          for (const log of operationLogs) {
            logsText += `=== ${log.nodeName} ===\n`
            logsText += `时间: ${formatDate(log.createdAt)}\n`
            logsText += `状态: ${log.status}\n`
            logsText += `命令: ${log.command}\n`
            logsText += `输出: ${log.output}\n\n`
          }
          deployLogs.value = logsText
        }
      }
    } catch (error) {
      console.error('获取日志失败:', error)
    }
  }, 1000) // 每秒轮询一次

  try {
    let results = ''
  
    // 如果是批量操作，调用批量禁用API
    if (targetNodes.length > 1) {
      const response = await apiClient.post('/nodes/docker/batch-disable', { nodeIds: targetNodes })
      results = response.data.results
    } else {
      // 单个节点操作
      const response = await apiClient.post(`/nodes/${targetNodes[0]}/docker/disable`)
      results = response.data.status
      // 检查禁用后的状态
      await checkDockerStatus()
    }
  
    // 停止轮询
    clearInterval(pollInterval)
  
    // 最后更新一次日志
    const response = await apiClient.get('/logs')
    if (response.data.logs && response.data.logs.length > 0) {
      const operationLogs = response.data.logs.filter(log => 
        log.operation === 'DisableDocker' && 
        targetNodes.includes(log.nodeId)
      )
      
      if (operationLogs.length > 0) {
        let logsText = ''
        for (const log of operationLogs) {
          logsText += `=== ${log.nodeName} ===\n`
          logsText += `时间: ${formatDate(log.createdAt)}\n`
          logsText += `状态: ${log.status}\n`
          logsText += `命令: ${log.command}\n`
          logsText += `输出: ${log.output}\n\n`
        }
        deployLogs.value = logsText
      } else {
        deployLogs.value = results
      }
    }
  
    emit('showMessage', { text: `Docker自启禁用成功! 共禁用了 ${targetNodes.length} 个节点`, type: 'success' })
  } catch (error) {
    // 停止轮询
    clearInterval(pollInterval)
  
    // 记录错误日志
    deployLogs.value = error.response?.data?.error || error.message
    emit('showMessage', { text: 'Docker自启禁用失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 删除Docker
const removeDocker = async () => {
  // 确定要操作的节点列表
  const targetNodes = selectedNodes.value.length > 0 ? selectedNodes.value : (selectedNodeId.value ? [selectedNodeId.value] : [])
  
  if (targetNodes.length === 0) {
    emit('showMessage', { text: '请先选择节点', type: 'warning' })
    return
  }

  // 确认删除操作
  if (!confirm('确定要删除Docker吗？此操作将删除Docker及其所有数据，不可恢复！')) {
    return
  }

  isDeploying.value = true
  deployLogs.value = ''
  
  // 启动轮询，获取最新日志
  const pollInterval = setInterval(async () => {
    try {
      const response = await apiClient.get('/logs')
      if (response.data.logs && response.data.logs.length > 0) {
        // 只显示与当前操作相关的日志
        const operationLogs = response.data.logs.filter(log => 
          log.operation === 'RemoveDocker' && 
          targetNodes.includes(log.nodeId)
        )
        
        if (operationLogs.length > 0) {
          // 格式化日志
          let logsText = ''
          for (const log of operationLogs) {
            logsText += `=== ${log.nodeName} ===\n`
            logsText += `时间: ${formatDate(log.createdAt)}\n`
            logsText += `状态: ${log.status}\n`
            logsText += `命令: ${log.command}\n`
            logsText += `输出: ${log.output}\n\n`
          }
          deployLogs.value = logsText
        }
      }
    } catch (error) {
      console.error('获取日志失败:', error)
    }
  }, 1000) // 每秒轮询一次

  try {
    let results = ''
  
    // 如果是批量操作，调用批量删除API
    if (targetNodes.length > 1) {
      const response = await apiClient.post('/nodes/docker/batch-remove', { nodeIds: targetNodes })
      results = response.data.results
    } else {
      // 单个节点操作
      const response = await apiClient.post(`/nodes/${targetNodes[0]}/docker/remove`)
      results = response.data.status
      // 重置Docker状态
      dockerStatus.value = ''
    }
  
    // 停止轮询
    clearInterval(pollInterval)
  
    // 最后更新一次日志
    const response = await apiClient.get('/logs')
    if (response.data.logs && response.data.logs.length > 0) {
      const operationLogs = response.data.logs.filter(log => 
        log.operation === 'RemoveDocker' && 
        targetNodes.includes(log.nodeId)
      )
      
      if (operationLogs.length > 0) {
        let logsText = ''
        for (const log of operationLogs) {
          logsText += `=== ${log.nodeName} ===\n`
          logsText += `时间: ${formatDate(log.createdAt)}\n`
          logsText += `状态: ${log.status}\n`
          logsText += `命令: ${log.command}\n`
          logsText += `输出: ${log.output}\n\n`
        }
        deployLogs.value = logsText
      } else {
        deployLogs.value = results
      }
    }
  
    emit('showMessage', { text: `Docker删除成功! 共删除了 ${targetNodes.length} 个节点`, type: 'success' })
  } catch (error) {
    // 停止轮询
    clearInterval(pollInterval)
  
    // 记录错误日志
    deployLogs.value = error.response?.data?.error || error.message
    emit('showMessage', { text: 'Docker删除失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 格式化日期
const formatDate = (dateString) => {
  const date = new Date(dateString)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// 页面加载时获取节点列表和Docker版本
onMounted(() => {
  getNodes()
  getDockerVersions()
})
</script>

<style scoped>
/* Docker管理区域 */
.docker-manager {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

.docker-section {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

.docker-section h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 20px 0;
}

/* 表单容器 */
.docker-form-container {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

/* 节点选择 */
.node-select {
  width: 100%;
  padding: 12px 15px;
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 0.95rem;
  transition: all 0.3s ease;
  font-family: inherit;
}

.node-select:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

/* Docker操作按钮 */
.docker-actions {
  display: flex;
  gap: 12px;
  margin: 20px 0;
  flex-wrap: wrap;
}

/* Docker配置部分 */
.docker-config-section {
  margin-top: 25px;
  padding-top: 20px;
  border-top: 1px solid var(--border-color);
}

.docker-config-section h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
}

.docker-config-form {
  display: flex;
  flex-direction: column;
  gap: 15px;
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

.form-group small {
  font-size: 0.8rem;
  color: var(--text-muted);
}

/* Docker状态区域 */
.docker-status-section {
  margin-top: 20px;
  padding: 15px;
  background-color: var(--bg-input);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.docker-status-section h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
}

.status-box {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 20px;
  border-radius: var(--radius-sm);
  font-weight: 600;
}

.status-running {
  background-color: rgba(39, 174, 96, 0.1);
  color: var(--success-color);
  border: 1px solid var(--success-color);
}

.status-enabled {
  background-color: rgba(243, 156, 18, 0.1);
  color: var(--warning-color);
  border: 1px solid var(--warning-color);
}

.status-stopped {
  background-color: rgba(231, 76, 60, 0.1);
  color: var(--error-color);
  border: 1px solid var(--error-color);
}

.status-icon {
  font-size: 1.2rem;
}

/* 日志区域 */
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

.btn-warning {
  background-color: var(--warning-color);
  color: white;
}

.btn-warning:hover:not(:disabled) {
  background-color: #d35400;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(243, 156, 18, 0.3);
}

.btn-info {
  background-color: var(--info-color);
  color: white;
}

.btn-info:hover:not(:disabled) {
  background-color: #2980b9;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(52, 152, 219, 0.3);
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