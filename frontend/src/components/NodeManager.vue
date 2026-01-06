<template>
  <div class="node-manager">
    <!-- 节点管理区域 -->
    <div class="nodes-section">
      <div class="nodes-controls">
        <button class="btn btn-primary" @click="showAddNodeForm = !showAddNodeForm">
          {{ showAddNodeForm ? '取消添加' : '添加节点' }}
        </button>
      </div>

      <!-- 添加节点表单 -->
      <div v-if="showAddNodeForm" class="add-node-panel">
        <h3>添加新节点</h3>
        <form class="add-node-form" @submit.prevent="addNode">
          <div class="form-row">
            <div class="form-group">
              <label for="nodeName">节点名称:</label>
              <input type="text" id="nodeName" v-model="newNode.name" placeholder="node-1" required>
            </div>
            <div class="form-group">
              <label for="nodeIPAdd">IP 地址:</label>
              <input type="text" id="nodeIPAdd" v-model="newNode.ip" placeholder="192.168.1.101" required>
            </div>
          </div>
          
          <div class="form-row">
            <div class="form-group">
              <label for="nodePortAdd">SSH 端口:</label>
              <input type="number" id="nodePortAdd" v-model="newNode.port" placeholder="22" required>
            </div>
            <div class="form-group">
              <label for="nodeUsernameAdd">用户名:</label>
              <input type="text" id="nodeUsernameAdd" v-model="newNode.username" placeholder="root" required>
            </div>
          </div>
          
          <div class="form-row">
            <div class="form-group">
              <label for="nodePasswordAdd">密码:</label>
              <input type="password" id="nodePasswordAdd" v-model="newNode.password" placeholder="密码（或使用私钥）">
            </div>
            <div class="form-group">
              <label for="nodeTypeAdd">节点类型:</label>
              <select id="nodeTypeAdd" v-model="newNode.nodeType" required>
                <option value="master">Master</option>
                <option value="worker">Worker</option>
              </select>
            </div>
          </div>
          
          <div class="form-group">
            <label for="nodePrivateKeyAdd">私钥 (可选):</label>
            <textarea id="nodePrivateKeyAdd" v-model="newNode.privateKey" placeholder="-----BEGIN RSA PRIVATE KEY-----..." rows="5"></textarea>
          </div>
          
          <div class="form-actions">
            <button type="submit" class="btn btn-primary">添加节点</button>
            <button type="button" class="btn btn-secondary" @click="showAddNodeForm = false">取消</button>
          </div>
        </form>
      </div>
      
      <!-- 节点列表 -->
      <div class="nodes-list">
        <table class="nodes-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>IP 地址</th>
              <th>类型</th>
              <th>状态</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="node in nodes" :key="node.id">
              <td>{{ node.name }}</td>
              <td>{{ node.ip }}</td>
              <td><span class="node-type-badge" :class="node.nodeType">{{ node.nodeType }}</span></td>
              <td>
                <span class="status-badge" :class="node.status">
                  {{ node.status }}
                </span>
              </td>
              <td>{{ formatDate(node.createdAt) }}</td>
              <td class="node-actions">
                <button 
                  class="btn btn-small btn-info" 
                  @click="testNodeConnection(node.id)"
                  :disabled="node.status === 'deploying'"
                >
                  测试
                </button>
                <button 
                  class="btn btn-small btn-primary" 
                  @click="deployNode(node.id)"
                  :disabled="node.status === 'deploying' || node.status === 'ready'"
                >
                  部署
                </button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="nodes.length === 0" class="empty-state">
          <div class="empty-icon"></div>
          <p>暂无节点，请添加节点</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080/api',
  timeout: 60000 // 60秒超时
})

// 节点管理相关状态
const nodes = ref([])
const showAddNodeForm = ref(false)
const newNode = ref({
  name: '',
  ip: '',
  port: 22,
  username: '',
  password: '',
  privateKey: '',
  nodeType: 'worker'
})

// 定义组件的事件
const emit = defineEmits(['showMessage'])

// 获取节点列表
const getNodes = async () => {
  try {
    const response = await apiClient.get('/nodes')
    // 确保nodes.value始终是数组
    if (Array.isArray(response.data)) {
      nodes.value = response.data
    } else {
      nodes.value = []
      emit('showMessage', { text: 'API返回的数据格式错误，期望数组类型', type: 'warning' })
    }
  } catch (error) {
    emit('showMessage', { text: '获取节点列表失败: ' + error.message, type: 'error' })
    // 确保nodes.value始终是数组
    nodes.value = []
  }
}

// 添加节点
const addNode = async () => {
  try {
    const response = await apiClient.post('/nodes', newNode.value)
    nodes.value.push(response.data)
    showAddNodeForm.value = false
    resetNewNodeForm()
    emit('showMessage', { text: '节点添加成功!', type: 'success' })
  } catch (error) {
    emit('showMessage', { text: '添加节点失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  }
}

// 重置添加节点表单
const resetNewNodeForm = () => {
  newNode.value = {
    name: '',
    ip: '',
    port: 22,
    username: '',
    password: '',
    privateKey: '',
    nodeType: 'worker'
  }
}

// 测试节点连接
const testNodeConnection = async (nodeId) => {
  try {
    const response = await apiClient.post(`/nodes/${nodeId}/test-connection`)
    if (response.data.connected) {
      emit('showMessage', { text: '节点连接测试成功!', type: 'success' })
      // 刷新节点列表
      await getNodes()
    } else {
      emit('showMessage', { text: '节点连接测试失败!', type: 'error' })
    }
  } catch (error) {
    emit('showMessage', { text: '测试节点连接失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  }
}

// 部署节点
const deployNode = async (nodeId) => {
  try {
    await apiClient.post(`/nodes/${nodeId}/deploy`)
    emit('showMessage', { text: '节点部署已开始!', type: 'success' })
    // 刷新节点列表
    await getNodes()
    // 每隔3秒刷新一次节点状态，共刷新10次
    let count = 0
    const interval = setInterval(async () => {
      await getNodes()
      count++
      if (count >= 10) {
        clearInterval(interval)
      }
    }, 3000)
  } catch (error) {
    emit('showMessage', { text: '部署节点失败: ' + (error.response?.data?.error || error.message), type: 'error' })
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

// 页面加载时获取节点列表
onMounted(() => {
  getNodes()
})
</script>

<style scoped>
/* 节点管理区域 */
.nodes-section {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

.nodes-controls {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 20px;
}

/* 添加节点面板 */
.add-node-panel {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  margin-bottom: 20px;
}

.add-node-panel h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 20px 0;
}

.add-node-form {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

/* 节点列表 */
.nodes-list {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  overflow-x: auto;
}

.nodes-table {
  width: 100%;
  border-collapse: collapse;
  background-color: var(--bg-card);
}

.nodes-table th,
.nodes-table td {
  padding: 12px 15px;
  text-align: left;
  border-bottom: 1px solid var(--border-color);
  font-size: 0.95rem;
}

.nodes-table th {
  background-color: var(--bg-input);
  font-weight: 600;
  color: var(--text-primary);
  text-transform: uppercase;
  font-size: 0.85rem;
  letter-spacing: 0.5px;
  position: sticky;
  top: 0;
  z-index: 10;
}

.nodes-table tr:hover {
  background-color: rgba(52, 152, 219, 0.05);
}

.nodes-table tr:last-child td {
  border-bottom: none;
}

/* 节点类型标签 */
.node-type-badge {
  padding: 4px 8px;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.node-type-badge.master {
  background-color: rgba(231, 76, 60, 0.2);
  color: var(--error-color);
}

.node-type-badge.worker {
  background-color: rgba(46, 204, 113, 0.2);
  color: var(--secondary-color);
}

/* 状态标签 */
.status-badge {
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 0.8rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.status-badge.online {
  background-color: rgba(46, 204, 113, 0.2);
  color: var(--secondary-color);
}

.status-badge.offline {
  background-color: rgba(122, 130, 166, 0.2);
  color: var(--text-muted);
}

.status-badge.ready {
  background-color: rgba(46, 204, 113, 0.2);
  color: var(--success-color);
}

.status-badge.deploying {
  background-color: rgba(243, 156, 18, 0.2);
  color: var(--warning-color);
}

.status-badge.error {
  background-color: rgba(231, 76, 60, 0.2);
  color: var(--error-color);
}

/* 节点操作按钮 */
.node-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* 空状态 */
.empty-state {
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
.form-group select,
.form-group textarea {
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
.form-group select:focus,
.form-group textarea:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

.form-group textarea {
  resize: vertical;
  min-height: 100px;
}

.form-actions {
  display: flex;
  gap: 12px;
  margin-top: 5px;
  flex-wrap: wrap;
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

.btn-info {
  background-color: var(--info-color);
  color: white;
}

.btn-info:hover:not(:disabled) {
  background-color: var(--primary-dark);
  transform: translateY(-1px);
}

.btn-small {
  padding: 8px 16px;
  font-size: 0.85rem;
  text-transform: none;
  letter-spacing: 0;
}
</style>