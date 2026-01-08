<template>
  <div class="admin-layout" :class="{ 'light-theme': isLightTheme }">
    <!-- 侧边导航栏 -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <h2 class="sidebar-title">
          K8s <span class="title-highlight">Deploy</span>
        </h2>
        <p class="sidebar-subtitle">Kubernetes 部署管理平台</p>
      </div>
      
      <nav class="nav-menu">
        <ul>
          <li>
            <a 
              href="#" 
              class="nav-item" 
              :class="{ active: activeMenu === 'dashboard' }"
              @click.prevent="emit('update:activeMenu', 'dashboard')"
            >
              <span class="nav-icon">📊</span>
              <span class="nav-text">仪表盘</span>
            </a>
          </li>
          <li>
            <a 
              href="#" 
              class="nav-item" 
              :class="{ active: activeMenu === 'kubeadm' }"
              @click.prevent="emit('update:activeMenu', 'kubeadm')"
            >
              <span class="nav-icon">📦</span>
              <span class="nav-text">Kubernetes集群部署</span>
            </a>
          </li>
          <li>
            <a 
              href="#" 
              class="nav-item" 
              :class="{ active: activeMenu === 'nodes' }"
              @click.prevent="emit('update:activeMenu', 'nodes')"
            >
              <span class="nav-icon">🖥️</span>
              <span class="nav-text">节点管理</span>
            </a>
          </li>
          <li>
            <a 
              href="#" 
              class="nav-item" 
              :class="{ active: activeMenu === 'cluster' }"
              @click.prevent="emit('update:activeMenu', 'cluster')"
            >
              <span class="nav-icon">🌐</span>
              <span class="nav-text">集群管理</span>
            </a>
          </li>
          <li>
            <a 
              href="#" 
              class="nav-item" 
              :class="{ active: activeMenu === 'logs' }"
              @click.prevent="emit('update:activeMenu', 'logs')"
            >
              <span class="nav-icon">📝</span>
              <span class="nav-text">日志管理</span>
            </a>
          </li>
          <li>
            <a 
              href="#" 
              class="nav-item" 
              :class="{ active: activeMenu === 'deployment' }"
              @click.prevent="emit('update:activeMenu', 'deployment')"
            >
              <span class="nav-icon">📋</span>
              <span class="nav-text">部署流程管理</span>
            </a>
          </li>
        </ul>
      </nav>
    </aside>
    
    <!-- 主内容区域 -->
    <main class="main-content">
      <!-- 顶部工具栏 -->
      <header class="top-bar">
        <div class="top-bar-left">
          <h1 class="page-title">{{ getPageTitle() }}</h1>
        </div>
        <div class="top-bar-right">
          <!-- 主题切换按钮 -->
          <button class="theme-toggle" @click="toggleTheme" title="切换主题">
            <span v-if="isLightTheme">🌙</span>
            <span v-else>☀️</span>
          </button>
          <div class="system-status">
            <div class="status-indicator" :class="{ 'online': systemOnline, 'offline': !systemOnline }"></div>
            <span class="status-text">{{ systemOnline ? '系统在线' : '系统离线' }}</span>
          </div>
        </div>
      </header>
      
      <!-- 内容区域 -->
      <div class="content">
        <slot></slot>
      </div>
    </main>
    
    <!-- 消息提示 -->
    <div v-if="message" class="toast" :class="message.type">
      <div class="toast-content">
        <div class="toast-icon" :class="message.type"></div>
        <span class="toast-text">{{ message.text }}</span>
      </div>
      <button class="toast-close" @click="closeMessage">&times;</button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

// 定义组件的属性和事件
const props = defineProps({
  activeMenu: {
    type: String,
    default: 'dashboard'
  },
  systemOnline: {
    type: Boolean,
    default: true
  },
  message: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['update:activeMenu', 'closeMessage'])

// 主题状态管理
const isLightTheme = ref(false)

// 切换主题
const toggleTheme = () => {
  isLightTheme.value = !isLightTheme.value
  // 保存主题偏好到localStorage
  localStorage.setItem('theme', isLightTheme.value ? 'light' : 'dark')
}

// 页面加载时读取主题偏好
if (localStorage.getItem('theme') === 'light') {
  isLightTheme.value = true
}

// 获取页面标题
  const getPageTitle = () => {
    const titles = {
      dashboard: '仪表盘概览',
      kubeadm: 'Kubernetes集群部署',
      nodes: '节点管理',
      cluster: '集群管理',
      logs: '部署日志',
      deployment: '部署流程管理'
    }
    return titles[props.activeMenu] || 'K8s Deploy'
  }

// 关闭消息
const closeMessage = () => {
  emit('closeMessage')
}
</script>

<style scoped>
/* 主题切换按钮样式 */
.theme-toggle {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  color: var(--text-primary);
  cursor: pointer;
  font-size: 1.2rem;
  height: 40px;
  width: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s ease;
  padding: 0;
}

.theme-toggle:hover {
  background-color: var(--bg-input);
  border-color: var(--primary-color);
  transform: scale(1.1);
}

.theme-toggle:focus {
  outline: none;
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

/* 默认深色主题 */
.admin-layout {
  display: flex;
  min-height: 100vh;
  background-color: var(--bg-primary);
  color: var(--text-primary);
  overflow: auto;
  transition: background-color 0.3s ease, color 0.3s ease;
}

/* 侧边导航栏 */
.sidebar {
  /* 精确宽度，包括边框 */
  width: 250px;
  background-color: var(--bg-secondary);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  box-shadow: var(--shadow-md);
  position: fixed;
  /* 与顶栏保持相同的高度逻辑，确保视觉上一致 */
  height: 100vh;
  overflow-y: auto;
  z-index: 100;
  left: 0;
  top: 0;
  transition: none;
  box-sizing: border-box;
  transition: background-color 0.3s ease, border-color 0.3s ease;
  /* 确保侧边栏与顶栏和主内容区域高度同步 */
  margin: 0;
  padding: 0;
}

/* 侧边栏头部 */
.sidebar-header {
  padding: 25px 20px;
  border-bottom: 1px solid var(--border-color);
  transition: border-color 0.3s ease;
  /* 确保侧边栏头部高度与顶栏协调 */
  height: 80px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  box-sizing: border-box;
}

/* 确保侧边栏内容不会导致布局变化 */
.sidebar-header,
.nav-menu {
  width: 100%;
  transition: none;
}

/* 确保ul和li元素不会导致布局变化 */
.nav-menu ul {
  width: 100%;
  transition: none;
}

.nav-menu li {
  width: 100%;
  transition: none;
  display: block;
}

.sidebar-header {
  padding: 25px 20px;
  border-bottom: 1px solid var(--border-color);
  transition: border-color 0.3s ease;
}

.sidebar-title {
  font-size: 1.5rem;
  font-weight: 700;
  margin: 0;
  color: var(--text-primary);
  transition: color 0.3s ease;
}

.title-highlight {
  color: var(--primary-color);
}

.sidebar-subtitle {
  font-size: 0.85rem;
  color: var(--text-secondary);
  margin: 5px 0 0 0;
  transition: color 0.3s ease;
}

/* 导航菜单 */
.nav-menu {
  flex: 1;
  padding: 20px 0;
}

.nav-menu ul {
  list-style: none;
  padding: 0;
  margin: 0;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 20px;
  color: var(--text-secondary);
  text-decoration: none;
  transition: all 0.3s ease;
  border-left: 3px solid transparent;
  font-weight: 500;
  min-height: 48px;
  box-sizing: border-box;
}

.nav-item:hover {
  background-color: rgba(52, 152, 219, 0.1);
  color: var(--text-primary);
  border-left-color: var(--primary-color);
  transform: translateX(0);
}

.nav-item.active {
  background-color: rgba(52, 152, 219, 0.15);
  color: var(--primary-color);
  border-left-color: var(--primary-color);
  font-weight: 600;
  transform: translateX(0);
}

.nav-icon {
  font-size: 1.1rem;
  width: 20px;
  text-align: center;
}

.nav-text {
  font-size: 0.95rem;
}

/* 主内容区域 */
.main-content {
  flex: 1;
  margin-left: 250px;
  display: flex;
  flex-direction: column;
  /* 为固定顶栏留出空间 */
  margin-top: 80px;
  /* 确保主内容区域占满剩余高度 */
  min-height: calc(100vh - 80px);
  /* 使用深色背景 */
  background-color: var(--bg-primary);
  transition: background-color 0.3s ease;
}

/* 顶部工具栏 */
.top-bar {
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  padding: 0 30px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: var(--shadow-sm);
  /* 固定高度，防止跳动 */
  height: 80px;
  min-height: 80px;
  max-height: 80px;
  /* 固定定位，完全固定在视口顶部 */
  position: fixed;
  top: 0;
  /* 与侧边栏宽度精确匹配 */
  left: 250px;
  /* 从左侧250px开始到右侧 */
  right: 0;
  z-index: 100;
  /* 精确计算宽度，确保与侧边栏同步 */
  width: calc(100% - 250px);
  box-sizing: border-box;
  /* 确保内容对齐 */
  overflow: hidden;
  /* 防止闪烁 */
  will-change: transform;
  transition: background-color 0.3s ease, border-color 0.3s ease;
}

.top-bar-left .page-title {
  font-size: 1.5rem;
  font-weight: 700;
  margin: 0;
  color: var(--text-primary);
  /* 固定样式，确保一致 */
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  /* 确保垂直居中 */
  line-height: 1.5;
  transition: color 0.3s ease;
}

.top-bar-right {
  display: flex;
  align-items: center;
  gap: 20px;
  /* 确保垂直居中 */
  height: 100%;
}

/* 内容区域 */
.content {
  flex: 1;
  padding: 25px 30px;
  overflow-y: auto;
  /* 确保内容不会被顶栏遮挡 */
  margin-top: 0;
  /* 宽度与顶栏完全匹配 */
  width: 100%;
  box-sizing: border-box;
  /* 使用深色背景 */
  background-color: var(--bg-primary);
  transition: background-color 0.3s ease;
}

/* 确保所有元素的宽度计算一致 */
* {
  box-sizing: border-box;
}

/* 响应式设计，确保在不同屏幕尺寸下保持一致 */
@media (max-width: 768px) {
  .sidebar {
    width: 200px;
  }
  
  .top-bar {
    left: 200px;
    width: calc(100% - 200px);
  }
  
  .main-content {
    margin-left: 200px;
  }
}

/* 系统状态指示器 */
.system-status {
  display: flex;
  align-items: center;
  gap: 10px;
  background-color: var(--bg-card);
  padding: 8px 15px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  transition: background-color 0.3s ease, border-color 0.3s ease;
}

.status-indicator {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  transition: all 0.3s ease;
}

.status-indicator.online {
  background-color: var(--success-color);
  box-shadow: 0 0 8px var(--success-color);
}

.status-indicator.offline {
  background-color: var(--error-color);
  box-shadow: 0 0 8px var(--error-color);
}

.status-text {
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-secondary);
  transition: color 0.3s ease;
}

/* 消息提示 */
.toast {
  position: fixed;
  top: 20px;
  right: 20px;
  padding: 15px 20px;
  border-radius: var(--radius-sm);
  color: white;
  font-weight: 600;
  box-shadow: var(--shadow-lg);
  z-index: 1000;
  display: flex;
  align-items: center;
  gap: 15px;
  animation: slideIn 0.3s ease-out;
  border-left: 4px solid transparent;
}

.toast.success {
  background-color: var(--success-color);
  border-left-color: #229954;
}

.toast.error {
  background-color: var(--error-color);
  border-left-color: #c0392b;
}

.toast.info {
  background-color: var(--info-color);
  border-left-color: #2980b9;
}

.toast.warning {
  background-color: var(--warning-color);
  border-left-color: #d35400;
}

.toast-content {
  display: flex;
  align-items: center;
  gap: 10px;
}

.toast-icon {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.2rem;
}

.toast-text {
  font-size: 0.95rem;
}

.toast-close {
  background: none;
  border: none;
  color: white;
  font-size: 1.5rem;
  cursor: pointer;
  padding: 0;
  line-height: 1;
  opacity: 0.8;
  transition: opacity 0.3s ease;
}

.toast-close:hover {
  opacity: 1;
}

@keyframes slideIn {
  from {
    transform: translateX(100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

/* 响应式设计 */
@media (max-width: 768px) {
  .sidebar {
    width: 200px;
  }
  
  .main-content {
    margin-left: 200px;
  }
  
  .content {
    padding: 15px 20px;
  }
  
  .top-bar {
    padding: 15px 20px;
  }
  
  .top-bar-left .page-title {
    font-size: 1.2rem;
  }
}

/* 浅色主题样式已移至全局style.css */
</style>