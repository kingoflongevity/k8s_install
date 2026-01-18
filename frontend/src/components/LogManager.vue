<template>
  <div class="log-manager">
    <!-- 日志管理区域 -->
    <div class="logs-section">
      <h3>部署日志</h3>
      <div class="logs-controls">
        <div class="sse-status">
          <span class="sse-status-indicator" :class="sseStatus"></span>
          <span class="sse-status-text">{{ sseStatusText }}</span>
        </div>
        <button class="btn btn-secondary" @click="clearLogs">清除日志</button>
      </div>
      
      <div class="logs-container">
        <div class="logs-list">
          <div class="log-statistics">
            <p>总日志数量: {{ (logs || []).length }}</p>
            <p>有ID的日志数量: {{ (logs || []).filter(log => log && log.id).length }}</p>
          </div>
          <div 
            v-for="(log, index) in logs" 
            :key="index" 
            class="log-item"
            :class="{'log-success': log && log.status === 'success', 'log-failed': log && log.status === 'failed', 'log-no-id': !log || !log.id}"
          >
            <div class="log-header">
              <div class="log-meta">
                <span class="log-node">{{ log ? log.nodeName : '无节点名称' }}</span>
                <span class="log-operation">{{ log ? log.operation : '无操作' }}</span>
                <span class="log-status" :class="log ? log.status : ''">{{ log ? log.status : '无状态' }}</span>
                <span class="log-time">{{ log ? formatDate(log.createdAt) : '无时间' }}</span>
              </div>
              <button 
                v-if="log && log.id" 
                class="log-toggle" 
                @click="toggleLogDetail(log.id)"
              >
                {{ log && expandedLogs.includes(log.id) ? '收起' : '展开' }}
              </button>
            </div>
            <div class="log-content">
              <div class="log-command">{{ log ? log.command : '无命令' }}</div>
              <div 
                v-if="log && expandedLogs.includes(log.id)" 
                class="log-output"
              >
                <pre>{{ log.output }}</pre>
              </div>
              <div v-if="!log || !log.id" class="log-error">
                <p>日志缺少ID: {{ JSON.stringify(log) }}</p>
              </div>
            </div>
          </div>
        </div>
        <div v-if="(logs || []).length === 0" class="empty-logs">
          <div class="empty-icon"></div>
          <p>暂无部署日志</p>
          <div class="sse-connection-info" v-if="sseStatus !== 'connected'">
            <p class="sse-connection-message">{{ sseStatusText }}</p>
            <button class="btn btn-secondary" @click="initSSE">重新连接</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, onActivated, onDeactivated } from 'vue'
import axios from 'axios'

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 1800000 // 30分钟超时，适应Kubernetes组件安装的耗时过程
})

// SSE配置
let eventSource = null
let sseReconnectTimer = null

// 状态变量
const logs = ref([])
const expandedLogs = ref([])
const sseStatus = ref('disconnected') // connected, connecting, disconnected, error
const sseStatusText = ref('未连接')
const justClearedLogs = ref(false) // 标记刚刚清除了日志，避免在组件激活时重新获取旧日志
let logInterval = null

// 定义组件的属性和事件
const props = defineProps({
  availableVersions: { type: Array, default: () => [] },
  kubeadmVersion: { type: String, default: '' },
  nodes: { type: Array, default: () => [] },
  systemOnline: { type: Boolean, default: true },
  apiStatus: { type: String, default: 'online' }
})

const emit = defineEmits(['showMessage'])

// 获取日志
const getLogs = async () => {
  // 如果刚刚清除了日志，跳过获取日志，避免重新获取旧日志
  if (justClearedLogs.value) {
    return
  }
  
  try {
    const response = await apiClient.get('/logs')
    const allLogs = response.data.logs || []
    logs.value = allLogs
  } catch (error) {
    // 静默处理日志API错误，不显示用户错误信息
    logs.value = []
  }
}

// 清除日志
const clearLogs = async () => {
  if (!confirm('确定要清除所有日志吗?')) {
    return
  }
  
  try {
    await apiClient.delete('/logs')
    logs.value = []
    expandedLogs.value = []
    justClearedLogs.value = true // 标记刚刚清除了日志，避免在组件激活时重新获取旧日志
    emit('showMessage', { text: '日志已清除!', type: 'success' })
    
    // 重启SSE连接，确保连接状态正确
    initSSE()
  } catch (error) {
    // 静默处理清除日志错误，不显示用户错误信息
    logs.value = []
    expandedLogs.value = []
    justClearedLogs.value = true // 标记刚刚清除了日志，避免在组件激活时重新获取旧日志
    
    // 重启SSE连接，确保连接状态正确
    initSSE()
  }
}

// 切换日志详情展开/收起
const toggleLogDetail = (logId) => {
  const index = expandedLogs.value.indexOf(logId)
  if (index === -1) {
    expandedLogs.value.push(logId)
  } else {
    expandedLogs.value.splice(index, 1)
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

// 初始化SSE连接
const initSSE = () => {
  // 如果已经有连接且状态为OPEN或CONNECTING，直接返回
  if (eventSource && (eventSource.readyState === EventSource.OPEN || eventSource.readyState === EventSource.CONNECTING)) {
    if (eventSource.readyState === EventSource.OPEN) {
      sseStatus.value = 'connected'
      sseStatusText.value = '已连接'
    } else {
      sseStatus.value = 'connecting'
      sseStatusText.value = '连接中...'
    }
    return
  }
  
  // 关闭现有连接
  if (eventSource) {
    try {
      eventSource.close()
      console.log('已关闭现有SSE连接')
    } catch (error) {
      console.error('关闭现有SSE连接失败:', error)
    }
    eventSource = null
  }
  
  // 更新连接状态为连接中
  sseStatus.value = 'connecting'
  sseStatusText.value = '连接中...'
  
  try {
    // 动态构建SSE URL，确保与API使用相同的主机和端口
    const apiBaseUrl = apiClient.defaults.baseURL
    const sseUrl = `${apiBaseUrl}/logs/stream`
    
    console.log('正在创建SSE连接:', sseUrl)
    eventSource = new EventSource(sseUrl, { withCredentials: false })
    
    // 连接打开时的处理
    eventSource.onopen = () => {
      console.log('SSE连接已建立')
      sseStatus.value = 'connected'
      sseStatusText.value = '已连接'
      justClearedLogs.value = false // 连接成功后，重置justClearedLogs标记，允许重新获取日志
    }
    
    // 接收消息时的处理
    eventSource.onmessage = (event) => {
      try {
        const logEntry = JSON.parse(event.data)
        // 忽略心跳事件
        if (logEntry.type === 'heartbeat') {
          return
        }
        // 检查日志是否已存在，避免重复
        const exists = logs.value.some(log => log && log.id === logEntry.id)
        if (!exists) {
          // 将新日志添加到数组开头
          logs.value.unshift(logEntry)
        }
      } catch (error) {
        console.error('解析SSE日志失败:', error)
        console.error('原始日志数据:', event.data)
        // 添加错误日志到界面
        logs.value.unshift({
          id: `error-${Date.now()}`,
          nodeName: '系统',
          operation: 'SSEError',
          command: '解析日志消息',
          output: `解析实时日志失败: ${error.message}`,
          status: 'failed',
          createdAt: new Date(),
          updatedAt: new Date()
        })
      }
    }
    
    // 连接关闭时的处理
    eventSource.onclose = () => {
      console.log('SSE连接已关闭')
      sseStatus.value = 'disconnected'
      sseStatusText.value = '已断开连接，正在重试...'
      // 尝试重新连接
      reconnectSSE()
    }
    
    // 连接错误时的处理
    eventSource.onerror = (error) => {
      console.error('SSE连接错误:', error)
      
      // 检查eventSource.readyState，只有当连接已关闭时才重新连接
      if (eventSource && eventSource.readyState === EventSource.CLOSED) {
        sseStatus.value = 'error'
        sseStatusText.value = '连接已关闭，正在重试...'
        // 尝试重新连接
        reconnectSSE()
      } else if (eventSource && eventSource.readyState === EventSource.CONNECTING) {
        // 连接中状态的错误，暂时不处理，等待onopen或onclose事件
        sseStatus.value = 'connecting'
        sseStatusText.value = '连接中...'
      } else {
        sseStatus.value = 'error'
        sseStatusText.value = '连接错误，正在重试...'
        // 尝试重新连接
        reconnectSSE()
      }
    }
  } catch (error) {
    console.error('创建SSE连接失败:', error)
    sseStatus.value = 'error'
    sseStatusText.value = '创建连接失败，正在重试...'
    // 添加错误日志到界面
    logs.value.unshift({
      id: `error-${Date.now()}`,
      nodeName: '系统',
      operation: 'SSEError',
      command: '建立连接',
      output: `创建实时日志连接失败: ${error.message}`,
      status: 'failed',
      createdAt: new Date(),
      updatedAt: new Date()
    })
    reconnectSSE()
  }
}

// 尝试重新连接SSE
const reconnectSSE = () => {
  // 清除现有定时器
  if (sseReconnectTimer) {
    clearTimeout(sseReconnectTimer)
  }
  
  // 5秒后重新连接
  sseReconnectTimer = setTimeout(() => {
    console.log('尝试重新连接SSE...')
    initSSE()
  }, 5000)
}

// 页面加载时获取日志
onMounted(() => {
  // 页面刷新时，先初始化SSE连接，等待连接成功后再获取日志
  initSSE()
  // 延迟1秒后获取日志，给SSE连接足够的时间建立
  setTimeout(() => {
    getLogs()
  }, 1000)
})

// 组件激活时启动日志刷新定时器
onActivated(() => {
  // 组件激活时，只在SSE连接已建立且不是刚刚清除日志时获取日志
  if ((!eventSource || eventSource.readyState !== EventSource.OPEN) && !justClearedLogs.value) {
    initSSE()
  }
  // 延迟500毫秒后获取日志，避免频繁调用
  setTimeout(() => {
    if (!justClearedLogs.value) {
      getLogs()
    }
  }, 500)
})

// 组件停用时清除日志刷新定时器和SSE连接
onDeactivated(() => {
  if (logInterval) {
    clearInterval(logInterval)
    logInterval = null
  }
  // 关闭SSE连接
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  // 清除重新连接定时器
  if (sseReconnectTimer) {
    clearTimeout(sseReconnectTimer)
    sseReconnectTimer = null
  }
})

// 组件卸载时清理资源
onUnmounted(() => {
  if (logInterval) {
    clearInterval(logInterval)
    logInterval = null
  }
  // 关闭SSE连接
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  // 清除重新连接定时器
  if (sseReconnectTimer) {
    clearTimeout(sseReconnectTimer)
    sseReconnectTimer = null
  }
})
</script>

<style scoped>
/* 日志管理区域 */
.log-manager {
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

/* 日志控制按钮 */
.logs-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

/* SSE状态指示器 */
.sse-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.9rem;
}

.sse-status-indicator {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background-color: var(--text-muted);
  transition: all 0.3s ease;
}

.sse-status-indicator.connected {
  background-color: var(--secondary-color);
  box-shadow: 0 0 10px rgba(46, 204, 113, 0.5);
}

.sse-status-indicator.connecting {
  background-color: var(--primary-color);
  animation: pulse 1.5s infinite;
}

.sse-status-indicator.disconnected {
  background-color: var(--warning-color);
}

.sse-status-indicator.error {
  background-color: var(--error-color);
}

@keyframes pulse {
  0% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.5;
    transform: scale(1.2);
  }
  100% {
    opacity: 1;
    transform: scale(1);
  }
}

.sse-status-text {
  color: var(--text-secondary);
  font-size: 0.85rem;
}

/* SSE连接信息 */
.sse-connection-info {
  margin-top: 20px;
  padding: 15px;
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: center;
}

.sse-connection-message {
  color: var(--text-secondary);
  margin: 0;
  font-size: 0.9rem;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .logs-controls {
    flex-direction: column;
    align-items: flex-end;
    gap: 10px;
  }
  
  .sse-status {
    align-self: flex-start;
  }
}

/* 日志容器 */
.logs-container {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  max-height: 600px;
  overflow-y: auto;
  padding: 10px;
}

/* 日志列表 */
.logs-list {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

/* 日志项 */
.log-item {
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  padding: 15px;
  transition: all 0.3s ease;
}

.log-item:hover {
  box-shadow: var(--shadow-sm);
  border-color: var(--primary-color);
}

.log-item.log-success {
  border-left: 4px solid var(--secondary-color);
}

.log-item.log-failed {
  border-left: 4px solid var(--error-color);
}

/* 日志头部 */
.log-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 10px;
  flex-wrap: wrap;
  gap: 10px;
  min-height: 40px;
  align-content: center;
}

/* 日志元数据 */
.log-meta {
  display: flex;
  gap: 15px;
  flex-wrap: wrap;
  align-items: center;
  flex: 1;
  min-width: 0;
}

.log-node {
  font-weight: 600;
  color: var(--primary-color);
  font-size: 0.95rem;
  flex-shrink: 0;
}

.log-operation {
  font-weight: 500;
  color: var(--text-primary);
  font-size: 0.9rem;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.log-status {
  padding: 4px 8px;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  flex-shrink: 0;
}

.log-status.success {
  background-color: rgba(46, 204, 113, 0.2);
  color: var(--secondary-color);
}

.log-status.failed {
  background-color: rgba(231, 76, 60, 0.2);
  color: var(--error-color);
}

.log-time {
  color: var(--text-muted);
  font-size: 0.8rem;
  flex-shrink: 0;
}

/* 日志切换按钮 */
.log-toggle {
  padding: 6px 12px;
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.3s ease;
  color: var(--text-primary);
  min-width: 60px;
  flex-shrink: 0;
  white-space: nowrap;
}

/* 响应式设计 */
@media (max-width: 768px) {
  /* 日志项 */
  .log-item {
    padding: 10px;
  }
  
  /* 日志元数据 */
  .log-meta {
    gap: 10px;
  }
  
  /* 日志节点名 */
  .log-node {
    font-size: 0.85rem;
  }
  
  /* 日志操作 */
  .log-operation {
    font-size: 0.8rem;
  }
  
  /* 日志状态 */
  .log-status {
    font-size: 0.7rem;
    padding: 2px 6px;
  }
  
  /* 日志时间 */
  .log-time {
    font-size: 0.75rem;
    flex: 1 100%;
    text-align: right;
  }
  
  /* 日志内容 */
  .log-content {
    gap: 8px;
  }
  
  /* 日志命令 */
  .log-command {
    font-size: 0.8rem;
    padding: 6px 10px;
  }
  
  /* 日志输出 */
  .log-output {
    font-size: 0.75rem;
    padding: 10px;
    max-height: 200px;
  }
  
  /* 日志切换按钮 */
  .log-toggle {
    padding: 5px 10px;
    font-size: 0.75rem;
    min-width: 50px;
  }
  
  /* 日志容器 */
  .logs-container {
    padding: 5px;
    max-height: 500px;
  }
  
  /* 日志列表间隙 */
  .logs-list {
    gap: 10px;
  }
  
  /* 日志管理区域 */
  .log-manager {
    padding: 15px;
  }
  
  /* 日志标题 */
  .logs-section h3 {
    margin-bottom: 15px;
  }
  
  /* 日志控制按钮 */
  .logs-controls {
    margin-bottom: 15px;
  }
}

.log-toggle:hover {
  background-color: var(--border-color);
  border-color: var(--primary-color);
}

/* 日志内容 */
.log-content {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* 日志命令 */
.log-command {
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.9rem;
  color: var(--text-secondary);
  word-break: break-all;
  background-color: var(--bg-secondary);
  padding: 8px 12px;
  border-radius: var(--radius-xs);
  border: 1px solid var(--border-color);
}

/* 日志输出 */
.log-output {
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.85rem;
  line-height: 1.5;
  color: var(--text-secondary);
  background-color: var(--bg-secondary);
  padding: 12px;
  border-radius: var(--radius-xs);
  border: 1px solid var(--border-color);
  max-height: 300px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

/* 空日志状态 */
.empty-logs {
  text-align: center;
  color: var(--text-muted);
  padding: 40px 20px;
  font-style: italic;
}

.empty-icon {
  width: 60px;
  height: 60px;
  margin: 0 auto 15px;
  background-color: var(--bg-input);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 2rem;
  color: var(--text-muted);
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

/* 滚动条样式 */
.logs-container::-webkit-scrollbar,
.log-output::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.logs-container::-webkit-scrollbar-track,
.log-output::-webkit-scrollbar-track {
  background: var(--bg-secondary);
  border-radius: 4px;
}

.logs-container::-webkit-scrollbar-thumb,
.log-output::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 4px;
}

.logs-container::-webkit-scrollbar-thumb:hover,
.log-output::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}
</style>