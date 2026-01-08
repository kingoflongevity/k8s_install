<template>
  <div class="node-manager">
    <!-- 节点管理区域 -->
    <div class="nodes-section">
      <div class="nodes-controls">
        <button class="btn btn-primary" @click="showAddNodeForm = !showAddNodeForm; editMode = false; resetNewNodeForm()">
          {{ showAddNodeForm ? '取消添加' : '添加节点' }}
        </button>
        <button 
          class="btn btn-secondary" 
          @click="configureSSHPasswdless"
          :disabled="nodes.length < 2"
        >
          配置节点SSH免密互通
        </button>
      </div>

      <!-- 添加/编辑节点表单 -->
      <div v-if="showAddNodeForm" class="add-node-panel">
        <h3>{{ editMode ? '编辑节点' : '添加新节点' }}</h3>
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
                  <option value="master">Master (主节点)</option>
                  <option value="worker">Worker (工作节点)</option>
                </select>
              </div>
          </div>
          
          <div class="form-group">
            <label for="nodePrivateKeyAdd">私钥 (可选):</label>
            <textarea id="nodePrivateKeyAdd" v-model="newNode.privateKey" placeholder="-----BEGIN RSA PRIVATE KEY-----..." rows="5"></textarea>
          </div>
          
          <div class="form-actions">
            <button type="submit" class="btn btn-primary">
              {{ editMode ? '保存修改' : '添加节点' }}
            </button>
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
                  测试连接
                </button>
                <button 
                  class="btn btn-small btn-primary" 
                  @click="configureNodeSSH(node.id)"
                  :disabled="node.status === 'deploying'"
                >
                  配置SSH
                </button>
                <button 
                  class="btn btn-small btn-secondary" 
                  @click="editNode(node)"
                  :disabled="node.status === 'deploying'"
                >
                  编辑
                </button>
                <button 
                  class="btn btn-small btn-danger" 
                  @click="deleteNode(node.id)"
                  :disabled="node.status === 'deploying'"
                >
                  删除
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
import { ref, onMounted, onActivated } from 'vue'
import axios from 'axios'

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 300000 // 5分钟超时，适应Kubernetes组件安装的耗时过程
})

// 本地状态
const nodes = ref([])
const showAddNodeForm = ref(false)
const editMode = ref(false)
const editingNodeId = ref('')
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

// 添加或编辑节点
const addNode = async () => {
  try {
    let response
    if (editMode.value) {
      // 编辑现有节点
      response = await apiClient.put(`/nodes/${editingNodeId.value}`, newNode.value)
      // 更新本地节点列表
      const index = nodes.value.findIndex(n => n.id === editingNodeId.value)
      if (index !== -1) {
        nodes.value[index] = response.data
      }
      emit('showMessage', { text: '节点更新成功!', type: 'success' })
    } else {
      // 添加新节点
      response = await apiClient.post('/nodes', newNode.value)
      nodes.value.push(response.data)
      emit('showMessage', { text: '节点添加成功!', type: 'success' })
    }
    showAddNodeForm.value = false
    resetNewNodeForm()
    editMode.value = false
    editingNodeId.value = ''
  } catch (error) {
    emit('showMessage', { text: `${editMode.value ? '更新' : '添加'}节点失败: ` + (error.response?.data?.error || error.message), type: 'error' })
  }
}

// 编辑节点
const editNode = (node) => {
  editingNodeId.value = node.id
  editMode.value = true
  showAddNodeForm.value = true
  
  // 填充表单数据
  newNode.value = {
    name: node.name,
    ip: node.ip,
    port: node.port,
    username: node.username,
    password: node.password,
    privateKey: node.privateKey,
    nodeType: node.nodeType
  }
}

// 删除节点
const deleteNode = async (nodeId) => {
  if (!confirm('确定要删除该节点吗?')) {
    return
  }
  
  try {
    await apiClient.delete(`/nodes/${nodeId}`)
    // 从本地列表中移除节点
    nodes.value = nodes.value.filter(n => n.id !== nodeId)
    emit('showMessage', { text: '节点删除成功!', type: 'success' })
  } catch (error) {
    emit('showMessage', { text: '删除节点失败: ' + (error.response?.data?.error || error.message), type: 'error' })
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
    let errorMessage = '测试节点连接失败: '
    const errorDetails = error.response?.data?.error || error.message
    
    // 根据错误信息提供更友好的提示
    if (errorDetails.includes('connection refused')) {
      errorMessage += '连接被拒绝，可能是目标服务器未开机或SSH服务未运行'
    } else if (errorDetails.includes('timeout')) {
      errorMessage += '连接超时，可能是网络问题或目标服务器防火墙设置'
    } else if (errorDetails.includes('permission denied')) {
      errorMessage += '权限被拒绝，可能是用户名或密码错误'
    } else if (errorDetails.includes('no route to host')) {
      errorMessage += '无法访问目标主机，可能是IP地址错误或网络不通'
    } else if (errorDetails.includes('failed to parse private key')) {
      errorMessage += '私钥解析失败，可能是私钥格式错误'
    } else if (errorDetails.includes('either password or privateKey must be provided')) {
      errorMessage += '请提供密码或私钥'
    } else if (errorDetails.includes('unable to authenticate')) {
      if (errorDetails.includes('attempted methods [none password]')) {
        errorMessage += '认证失败，尝试了密码认证但失败，可能是密码错误或目标服务器不允许密码认证'
      } else if (errorDetails.includes('attempted methods [none publickey]')) {
        errorMessage += '认证失败，尝试了公钥认证但失败，可能是私钥不匹配或目标服务器未配置公钥'
      } else {
        errorMessage += '认证失败，可能是用户名、密码或私钥错误'
      }
    } else {
      errorMessage += errorDetails
    }
    
    emit('showMessage', { text: errorMessage, type: 'error' })
  }
}



// 配置单个节点的SSH设置
const configureNodeSSH = async (nodeId) => {
  try {
    await apiClient.post(`/nodes/${nodeId}/ssh/configure`)
    emit('showMessage', { text: '节点SSH配置成功!', type: 'success' })
    // 刷新节点列表
    await getNodes()
  } catch (error) {
    emit('showMessage', { text: '配置节点SSH失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  }
}

// 配置所有节点之间的SSH免密互通
const configureSSHPasswdless = async () => {
  if (nodes.value.length < 2) {
    emit('showMessage', { text: '至少需要2个节点才能配置SSH免密互通!', type: 'warning' })
    return
  }

  try {
    await apiClient.post('/nodes/ssh/passwdless')
    emit('showMessage', { text: '节点SSH免密互通配置成功!', type: 'success' })
  } catch (error) {
    emit('showMessage', { text: '配置节点SSH免密互通失败: ' + (error.response?.data?.error || error.message), type: 'error' })
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

// 组件激活时刷新数据
onActivated(() => {
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

.btn-danger {
  background-color: var(--error-color);
  color: white;
}

.btn-danger:hover:not(:disabled) {
  background-color: #c0392b;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(231, 76, 60, 0.3);
}

.btn-small {
  padding: 8px 16px;
  font-size: 0.85rem;
  text-transform: none;
  letter-spacing: 0;
}
</style>