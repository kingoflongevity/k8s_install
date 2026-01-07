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
      :docker-version="dockerVersion"
      :nodes="nodes"
      :system-online="systemOnline"
      :api-status="apiStatus"
    />
    
    <!-- Kubeadm 包管理 -->
    <KubeadmManager 
      v-else-if="activeMenu === 'kubeadm'"
      :available-versions="availableVersions"
      @show-message="showMessage"
      @set-kubeadm-version="kubeadmVersion = $event"
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
    
    <!-- Docker管理 -->
    <DockerManager 
      v-else-if="activeMenu === 'docker'"
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
import DockerManager from './components/DockerManager.vue'

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 60000 // 60秒超时
})

// 状态变量
const kubeadmVersion = ref('')
const dockerVersion = ref('')
const message = ref(null)
const systemOnline = ref(true)
const apiStatus = ref('online')
const activeMenu = ref('dashboard')

// Kubeadm 包管理相关状态
const availableVersions = ref([])

// 节点管理相关状态
const nodes = ref([])

// 获取 Kubeadm 版本 - 只在用户下载包后才显示，这里不再自动获取
const getKubeadmVersion = () => {
  // 不自动获取版本，由用户下载包后设置
  // 保持函数定义以兼容现有代码
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

/* CSS变量已在style.css中定义，此处不再重复 */
</style>
