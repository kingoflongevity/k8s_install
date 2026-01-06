<template>
  <div class="admin-layout">
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
              @click.prevent="activeMenu = 'dashboard'"
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
              @click.prevent="activeMenu = 'kubeadm'"
            >
              <span class="nav-icon">📦</span>
              <span class="nav-text">Kubeadm 管理</span>
            </a>
          </li>
          <li>
            <a 
              href="#" 
              class="nav-item" 
              :class="{ active: activeMenu === 'nodes' }"
              @click.prevent="activeMenu = 'nodes'"
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
              @click.prevent="activeMenu = 'cluster'"
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
              @click.prevent="activeMenu = 'logs'"
            >
              <span class="nav-icon">📝</span>
              <span class="nav-text">日志管理</span>
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

// 获取页面标题
const getPageTitle = () => {
  const titles = {
    dashboard: '仪表盘概览',
    kubeadm: 'Kubeadm 包管理',
    nodes: '节点管理',
    cluster: '集群管理',
    logs: '部署日志'
  }
  return titles[props.activeMenu] || 'K8s Deploy'
}

// 关闭消息
const closeMessage = () => {
  emit('closeMessage')
}
</script>

<style scoped>
.admin-layout {
  display: flex;
  min-height: 100vh;
  background-color: var(--bg-primary);
  color: var(--text-primary);
}

/* 侧边导航栏 */
.sidebar {
  width: 250px;
  background-color: var(--bg-secondary);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  box-shadow: var(--shadow-md);
  position: fixed;
  height: 100vh;
  overflow-y: auto;
  z-index: 100;
}

.sidebar-header {
  padding: 25px 20px;
  border-bottom: 1px solid var(--border-color);
}

.sidebar-title {
  font-size: 1.5rem;
  font-weight: 700;
  margin: 0;
  color: var(--text-primary);
}

.title-highlight {
  color: var(--primary-color);
}

.sidebar-subtitle {
  font-size: 0.85rem;
  color: var(--text-secondary);
  margin: 5px 0 0 0;
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
}

.nav-item:hover {
  background-color: rgba(52, 152, 219, 0.1);
  color: var(--text-primary);
  border-left-color: var(--primary-color);
}

.nav-item.active {
  background-color: rgba(52, 152, 219, 0.15);
  color: var(--primary-color);
  border-left-color: var(--primary-color);
  font-weight: 600;
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
}

/* 顶部工具栏 */
.top-bar {
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  padding: 20px 30px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: var(--shadow-sm);
}

.top-bar-left .page-title {
  font-size: 1.5rem;
  font-weight: 700;
  margin: 0;
  color: var(--text-primary);
}

.top-bar-right {
  display: flex;
  align-items: center;
  gap: 20px;
}

/* 内容区域 */
.content {
  flex: 1;
  padding: 25px 30px;
  overflow-y: auto;
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
</style>