<template>
  <Layout 
    :active-menu="activeMenu" 
    :system-online="systemOnline"
    :message="message"
    @update:activeMenu="activeMenu = $event"
    @close-message="closeMessage"
  >
    <keep-alive>
      <component 
      :is="currentComponent" 
      :key="activeMenu"
      :available-versions="availableVersions"
      :kubeadm-version="kubeadmVersion"
      :nodes="nodes"
      :system-online="systemOnline"
      :api-status="apiStatus"
      @show-message="showMessage"
      @set-kubeadm-version="kubeadmVersion = $event"
      @update:nodes="getNodes"
    />
    </keep-alive>
  </Layout>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import axios from 'axios'

// 导入组件
import Layout from './components/Layout.vue'
import Dashboard from './components/Dashboard.vue'
import KubeadmManager from './components/KubeadmManager.vue'
import NodeManager from './components/NodeManager.vue'
import ClusterManager from './components/ClusterManager.vue'
import LogManager from './components/LogManager.vue'
import DeploymentManager from './components/DeploymentManager.vue'

// 组件映射
const componentMap = {
  dashboard: Dashboard,
  kubeadm: KubeadmManager,
  nodes: NodeManager,
  cluster: ClusterManager,
  logs: LogManager,
  deployment: DeploymentManager
}

// 当前活动组件
const currentComponent = computed(() => {
  try {
    const component = componentMap[activeMenu.value] || Dashboard
    if (!component) {
      console.error('无效的组件:', activeMenu.value)
      return Dashboard
    }
    return component
  } catch (error) {
    console.error('获取当前组件失败:', error)
    return Dashboard
  }
})

// API 配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 300000 // 5分钟超时，适应Kubernetes组件安装的耗时过程
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
      // API返回的数据格式错误，显示警告消息
      message.value = { 
        text: "API返回的数据格式错误，期望versions字段为数组类型", 
        type: "warning" 
      }
    }
  } catch (error) {
    // 使用默认版本列表，显示错误消息
    availableVersions.value = ['v1.30.0', 'v1.29.4', 'v1.28.8', 'v1.27.12']
    message.value = { 
      text: `获取Kubeadm版本列表失败: ${error.message}`, 
      type: "error" 
    }
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
      // API返回的数据格式错误，显示警告消息
      message.value = { 
        text: "API返回的数据格式错误，期望数组类型", 
        type: "warning" 
      }
    }
  } catch (error) {
    // 使用空数组，显示错误消息
    nodes.value = []
    message.value = { 
      text: `获取节点列表失败: ${error.message}`, 
      type: "error" 
    }
  }
}

// 显示消息
const showMessage = (messageData) => {
  // 支持两种调用方式：1. showMessage('消息文本', 'success') 2. showMessage({ text: '消息文本', type: 'success' })
  let text, type
  if (typeof messageData === 'string') {
    text = messageData
    type = arguments[1] || 'info'
  } else {
    text = messageData.text
    type = messageData.type || 'info'
  }
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
