<template>
  <div class="log-manager">
    <!-- 日志管理区域 -->
    <div class="logs-section">
      <h3>部署日志</h3>
      <div class="logs-controls">
        <button class="btn btn-secondary" @click="clearLogs">清除日志</button>
      </div>
      
      <div class="logs-container">
        <pre v-if="logs" class="logs-content">{{ logs }}</pre>
        <div v-else class="empty-logs">
          <div class="empty-icon"></div>
          <p>暂无部署日志</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import axios from 'axios'

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080/api',
  timeout: 60000 // 60秒超时
})

// 状态变量
const logs = ref('')

// 定义组件的事件
const emit = defineEmits(['showMessage'])

// 获取日志
const getLogs = async () => {
  try {
    const response = await apiClient.get('/logs')
    logs.value = response.data.logs || ''
  } catch (error) {
    emit('showMessage', { text: '获取日志失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  }
}

// 清除日志
const clearLogs = async () => {
  if (!confirm('确定要清除所有日志吗?')) {
    return
  }
  
  try {
    await apiClient.delete('/logs')
    logs.value = ''
    emit('showMessage', { text: '日志已清除!', type: 'success' })
  } catch (error) {
    emit('showMessage', { text: '清除日志失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  }
}

// 页面加载时获取日志
onMounted(() => {
  getLogs()
  
  // 每隔5秒刷新一次日志
  const interval = setInterval(() => {
    getLogs()
  }, 5000)
  
  // 组件卸载时清除定时器
  onUnmounted(() => {
    clearInterval(interval)
  })
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
  padding: 20px;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.9rem;
  line-height: 1.6;
}

.logs-content {
  margin: 0;
  color: var(--text-secondary);
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
</style>