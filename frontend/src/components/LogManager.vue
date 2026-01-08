<template>
  <div class="log-manager">
    <!-- 日志管理区域 -->
    <div class="logs-section">
      <h3>部署日志</h3>
      <div class="logs-controls">
        <button class="btn btn-secondary" @click="clearLogs">清除日志</button>
      </div>
      
      <div class="logs-container">
        <div v-if="logs.length > 0" class="logs-list">
          <div 
            v-for="log in logs" 
            :key="log.id" 
            class="log-item"
            :class="{'log-success': log.status === 'success', 'log-failed': log.status === 'failed'}"
          >
            <div class="log-header">
              <div class="log-meta">
                <span class="log-node">{{ log.nodeName }}</span>
                <span class="log-operation">{{ log.operation }}</span>
                <span class="log-status" :class="log.status">{{ log.status }}</span>
                <span class="log-time">{{ formatDate(log.createdAt) }}</span>
              </div>
              <button 
                class="log-toggle" 
                @click="toggleLogDetail(log.id)"
              >
                {{ expandedLogs.includes(log.id) ? '收起' : '展开' }}
              </button>
            </div>
            <div class="log-content">
              <div class="log-command">{{ log.command }}</div>
              <div 
                v-if="expandedLogs.includes(log.id)" 
                class="log-output"
              >
                <pre>{{ log.output }}</pre>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="empty-logs">
          <div class="empty-icon"></div>
          <p>暂无部署日志</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, onActivated, onDeactivated, watch } from 'vue'
import axios from 'axios'

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 300000 // 5分钟超时，适应Kubernetes组件安装的耗时过程
})

// 状态变量
const logs = ref([])
const expandedLogs = ref([])
let logInterval = null

// 定义组件的事件
const emit = defineEmits(['showMessage'])

// 获取日志
const getLogs = async () => {
  try {
    const response = await apiClient.get('/logs')
    logs.value = response.data.logs || []
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
    emit('showMessage', { text: '日志已清除!', type: 'success' })
  } catch (error) {
    // 静默处理清除日志错误，不显示用户错误信息
    logs.value = []
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

// 页面加载时获取日志
onMounted(() => {
  getLogs()
})

// 组件激活时启动日志刷新定时器
onActivated(() => {
  getLogs()
  
  // 每隔1秒刷新一次日志，实现实时效果
  logInterval = setInterval(() => {
    getLogs()
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
  justify-content: flex-end;
  margin-bottom: 20px;
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