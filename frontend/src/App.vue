<template>
  <Layout 
    :active-menu="activeMenu" 
    :system-online="systemOnline"
    :message="message"
    @update:activeMenu="activeMenu = $event"
    @close-message="closeMessage"
  >
    <!-- 仪表盘概览 -->
    <Dashboard 
      v-if="activeMenu === 'dashboard'"
      :kubeadm-version="kubeadmVersion"
      :nodes="nodes"
      :system-online="systemOnline"
      :api-status="apiStatus"
    />
    
    <!-- Kubeadm 包管理 -->
    <KubeadmManager 
      v-else-if="activeMenu === 'kubeadm'"
      :available-versions="availableVersions"
      @show-message="showMessage"
    />
    
    <!-- 节点管理 -->
    <NodeManager 
      v-else-if="activeMenu === 'nodes'"
      @show-message="showMessage"
    />
    
    <!-- 集群管理 -->
    <ClusterManager 
      v-else-if="activeMenu === 'cluster'"
      @show-message="showMessage"
    />
    
    <!-- 日志管理 -->
    <LogManager 
      v-else-if="activeMenu === 'logs'"
      @show-message="showMessage"
    />
  </Layout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'

// 导入组件
import Layout from './components/Layout.vue'
import Dashboard from './components/Dashboard.vue'
import KubeadmManager from './components/KubeadmManager.vue'
import NodeManager from './components/NodeManager.vue'
import ClusterManager from './components/ClusterManager.vue'
import LogManager from './components/LogManager.vue'

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080/api',
  timeout: 60000 // 60秒超时
})

// 状态变量
const kubeadmVersion = ref('')
const message = ref(null)
const systemOnline = ref(true)
const apiStatus = ref('online')
const activeMenu = ref('dashboard')

// Kubeadm 包管理相关状态
const availableVersions = ref([])

// 节点管理相关状态
const nodes = ref([])

// 获取 Kubeadm 版本
const getKubeadmVersion = async () => {
  try {
    const response = await apiClient.get('/kubeadm/version')
    kubeadmVersion.value = response.data.version
  } catch (error) {
    showMessage('获取 Kubeadm 版本失败: ' + error.message, 'error')
    apiStatus.value = 'offline'
    systemOnline.value = false
  }
}

// 获取可用的 Kubeadm 版本列表
const getAvailableVersions = async () => {
  try {
    const response = await apiClient.get('/kubeadm/packages')
    // 确保availableVersions.value始终是数组
    if (Array.isArray(response.data.versions)) {
      availableVersions.value = response.data.versions
    } else {
      availableVersions.value = ['v1.30.0', 'v1.29.4', 'v1.28.8', 'v1.27.12']
      showMessage('API返回的版本列表格式错误，使用默认版本', 'warning')
    }
  } catch (error) {
    showMessage('获取可用版本列表失败: ' + error.message, 'error')
    // 使用默认版本列表
    availableVersions.value = ['v1.30.0', 'v1.29.4', 'v1.28.8', 'v1.27.12']
  }
}

// 获取节点列表
const getNodes = async () => {
  try {
    const response = await apiClient.get('/nodes')
    // 确保nodes.value始终是数组
    if (Array.isArray(response.data)) {
      nodes.value = response.data
    } else {
      nodes.value = []
      showMessage('API返回的数据格式错误，期望数组类型', 'warning')
    }
  } catch (error) {
    showMessage('获取节点列表失败: ' + error.message, 'error')
    // 确保nodes.value始终是数组
    nodes.value = []
  }
}

// 显示消息
const showMessage = (text, type = 'info') => {
  message.value = { text, type }
  setTimeout(closeMessage, 5000)
}

// 关闭消息
const closeMessage = () => {
  message.value = null
}

// 页面加载时获取状态
onMounted(() => {
  getKubeadmVersion()
  getNodes()
  getAvailableVersions()
})
</script>

<style>
/* 全局样式重置和基础设置 */
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

:root {
  /* 主题颜色 */
  --primary-color: #3498db;
  --primary-dark: #2980b9;
  --secondary-color: #2ecc71;
  --success-color: #27ae60;
  --error-color: #e74c3c;
  --warning-color: #f39c12;
  --info-color: #3498db;
  
  /* 背景颜色 */
  --bg-primary: #0a0e27;
  --bg-secondary: #121735;
  --bg-card: #1e2440;
  --bg-input: #2a2f4c;
  
  /* 文本颜色 */
  --text-primary: #ffffff;
  --text-secondary: #b0b8d4;
  --text-muted: #7a82a6;
  
  /* 边框颜色 */
  --border-color: #3a4167;
  --border-light: #4a5078;
  
  /* 阴影效果 */
  --shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.2);
  --shadow-md: 0 4px 16px rgba(0, 0, 0, 0.3);
  --shadow-lg: 0 8px 32px rgba(0, 0, 0, 0.4);
  
  /* 圆角 */
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;
}

body {
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  background-color: var(--bg-primary);
  color: var(--text-primary);
  line-height: 1.6;
  margin: 0;
  padding: 0;
}
</style>
