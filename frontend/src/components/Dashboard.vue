<template>
  <div class="dashboard">
    <!-- 仪表盘概览 -->
    <section class="dashboard-section">
      <div class="dashboard-cards">
        <!-- 系统状态卡片 -->
        <div class="info-card">
          <div class="card-header">
            <h3>系统状态</h3>
            <div class="card-icon system-icon"></div>
          </div>
          <div class="card-body">
            <div class="status-item">
              <span class="status-label">Kubernetes 版本:</span>
              <span v-if="kubeadmVersion" class="status-value">{{ kubeadmVersion }}</span>
              <span v-else class="status-value status-muted">未安装</span>
            </div>

            <div class="status-item">
              <span class="status-label">API 状态:</span>
              <span class="status-value" :class="{ 'success': apiStatus === 'online', 'error': apiStatus === 'offline' }">{{ apiStatus }}</span>
            </div>
          </div>
        </div>

        <!-- 集群状态卡片 -->
        <div class="info-card">
          <div class="card-header">
            <h3>集群状态</h3>
            <div class="card-icon cluster-icon"></div>
          </div>
          <div class="card-body">
            <div class="stat-grid">
              <div class="stat-item">
                <span class="stat-value">{{ (nodes || []).filter(n => n).length }}</span>
                <span class="stat-label">总节点</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">{{ (nodes || []).filter(n => n && n.nodeType === 'master').length }}</span>
                <span class="stat-label">Master</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">{{ (nodes || []).filter(n => n && n.nodeType === 'worker').length }}</span>
                <span class="stat-label">Worker</span>
              </div>
              <div class="stat-item">
                <span class="stat-value">{{ (nodes || []).filter(n => n && n.status === 'ready').length }}</span>
                <span class="stat-label">就绪</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- 集群操作卡片 -->
    <section class="cluster-actions">
      <div class="info-card">
        <div class="card-header">
          <h3>集群操作</h3>
          <div class="card-icon action-icon"></div>
        </div>
        <div class="card-body">
          <div class="action-buttons">
            <button class="btn btn-primary" @click="$emit('navigate', 'cluster')">
              <span class="btn-icon">📋</span>
              <span>查看集群详情</span>
            </button>
            <button class="btn btn-primary" @click="$emit('navigate', 'kubeadm')">
              <span class="btn-icon">📦</span>
              <span>部署Kubernetes集群</span>
            </button>
            <button class="btn btn-primary" @click="$emit('navigate', 'nodes')">
              <span class="btn-icon">🖥️</span>
              <span>管理节点</span>
            </button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
// 定义组件的属性和事件
const props = defineProps({
  availableVersions: {
    type: Array,
    default: () => []
  },
  kubeadmVersion: {
    type: String,
    default: ''
  },
  apiStatus: {
    type: String,
    default: 'online'
  },
  nodes: {
    type: Array,
    default: () => []
  },
  systemOnline: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['navigate'])
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

/* 仪表盘卡片容器 */
.dashboard-section {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

.dashboard-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 20px;
}

/* 信息卡片 */
.info-card {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  transition: all 0.3s ease;
}

.info-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
  border-color: var(--primary-color);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.card-header h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.card-icon {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  color: var(--text-primary);
}

.system-icon {
  background: linear-gradient(135deg, var(--primary-color), var(--primary-dark));
}

.cluster-icon {
  background: linear-gradient(135deg, var(--secondary-color), var(--success-color));
}

.action-icon {
  background: linear-gradient(135deg, var(--warning-color), var(--error-color));
}

/* 卡片内容 */
.card-body {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

/* 状态项 */
.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid var(--border-light);
}

.status-item:last-child {
  border-bottom: none;
}

.status-label {
  font-size: 0.9rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.status-value {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-primary);
  font-family: 'Courier New', Courier, monospace;
}

.status-value.success {
  color: var(--success-color);
}

.status-value.error {
  color: var(--error-color);
}

.status-value.status-muted {
  color: var(--text-muted);
  font-style: italic;
}

.status-loading {
  color: var(--warning-color);
  font-style: italic;
}

/* 统计网格 */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 15px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 15px;
  background-color: var(--bg-input);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
}

.stat-value {
  font-size: 1.8rem;
  font-weight: 700;
  color: var(--primary-color);
  margin-bottom: 5px;
}

.stat-label {
  font-size: 0.85rem;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 500;
}

/* 集群操作卡片 */
.cluster-actions {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

.action-buttons {
  display: flex;
  gap: 15px;
  flex-wrap: wrap;
}

.action-buttons .btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 24px;
  font-size: 0.95rem;
}

.btn-icon {
  font-size: 1.1rem;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .dashboard-cards {
    grid-template-columns: 1fr;
  }
  
  .stat-grid {
    grid-template-columns: 1fr;
  }
  
  .action-buttons {
    flex-direction: column;
  }
}
</style>