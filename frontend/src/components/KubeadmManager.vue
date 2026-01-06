<template>
  <div class="kubeadm-manager">
    <section class="package-section">
      <div class="package-container">
        <div class="package-form">
          <h3>包下载与部署</h3>
          <form @submit.prevent="downloadAndDeploy">
            <div class="form-row">
              <div class="form-group">
                <label for="kubeadmVersionSelect">选择版本:</label>
                <select id="kubeadmVersionSelect" v-model="selectedVersion" required>
                  <option value="">-- 选择 Kubeadm 版本 --</option>
                  <option v-for="version in availableVersions" :key="version" :value="version">{{ version }}</option>
                </select>
              </div>
              <div class="form-group">
                <label for="archSelect">架构:</label>
                <select id="archSelect" v-model="selectedArch" required>
                  <option value="amd64">amd64</option>
                  <option value="arm64">arm64</option>
                </select>
              </div>
              <div class="form-group">
                <label for="distroSelect">发行版:</label>
                <select id="distroSelect" v-model="selectedDistro" required>
                  <option value="ubuntu">Ubuntu</option>
                  <option value="debian">Debian</option>
                  <option value="centos">CentOS</option>
                  <option value="rhel">RHEL</option>
                </select>
              </div>
            </div>

            <h4>目标节点配置</h4>
            <div class="form-row">
              <div class="form-group">
                <label for="nodeIP">节点 IP:</label>
                <input type="text" id="nodeIP" v-model="targetNode.ip" placeholder="192.168.1.100" required>
              </div>
              <div class="form-group">
                <label for="nodePort">SSH 端口:</label>
                <input type="number" id="nodePort" v-model="targetNode.port" placeholder="22" required>
              </div>
            </div>

            <div class="form-row">
              <div class="form-group">
                <label for="nodeUsername">用户名:</label>
                <input type="text" id="nodeUsername" v-model="targetNode.username" placeholder="root" required>
              </div>
              <div class="form-group">
                <label for="nodePassword">密码:</label>
                <input type="password" id="nodePassword" v-model="targetNode.password" placeholder="密码（或使用私钥）">
              </div>
            </div>

            <div class="form-group">
              <label for="nodePrivateKey">私钥 (可选):</label>
              <textarea id="nodePrivateKey" v-model="targetNode.privateKey" placeholder="-----BEGIN RSA PRIVATE KEY-----..." rows="5"></textarea>
            </div>

            <div class="form-actions">
              <button type="submit" class="btn btn-primary" :disabled="isDeploying">
                <span v-if="isDeploying" class="loading-spinner"></span>
                {{ isDeploying ? '部署中...' : '下载并部署' }}
              </button>
              <button type="button" class="btn btn-secondary" @click="resetPackageForm">重置</button>
            </div>
          </form>
        </div>

        <div class="package-info">
          <h3>包信息</h3>
          <div v-if="currentPackage" class="package-details">
            <div class="detail-item">
              <span class="detail-label">版本:</span>
              <span class="detail-value">{{ currentPackage.version }}</span>
            </div>
            <div class="detail-item">
              <span class="detail-label">架构:</span>
              <span class="detail-value">{{ currentPackage.arch }}</span>
            </div>
            <div class="detail-item">
              <span class="detail-label">发行版:</span>
              <span class="detail-value">{{ currentPackage.distro }}</span>
            </div>
            <div class="detail-item">
              <span class="detail-label">路径:</span>
              <span class="detail-value">{{ currentPackage.path }}</span>
            </div>
          </div>
          <div v-else class="no-package">
            <p>请选择版本并部署</p>
          </div>
        </div>
      </div>
    </section>

    <!-- 部署日志 -->
    <section v-if="deployLogs" class="logs-section">
      <h3>部署日志</h3>
      <div class="logs-container">
        <pre>{{ deployLogs }}</pre>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref } from 'vue'

// 定义组件的属性和事件
const props = defineProps({
  availableVersions: {
    type: Array,
    default: () => []
  },
  isDeploying: {
    type: Boolean,
    default: false
  },
  deployLogs: {
    type: String,
    default: ''
  },
  currentPackage: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['downloadAndDeploy', 'resetPackageForm'])

// 本地状态
const selectedVersion = ref('')
const selectedArch = ref('amd64')
const selectedDistro = ref('ubuntu')
const targetNode = ref({
  ip: '',
  port: 22,
  username: '',
  password: '',
  privateKey: ''
})

// 触发下载和部署事件
const downloadAndDeploy = () => {
  emit('downloadAndDeploy', {
    version: selectedVersion.value,
    arch: selectedArch.value,
    distro: selectedDistro.value,
    node: targetNode.value
  })
}

// 触发重置表单事件
const resetPackageForm = () => {
  selectedVersion.value = ''
  selectedArch.value = 'amd64'
  selectedDistro.value = 'ubuntu'
  targetNode.value = {
    ip: '',
    port: 22,
    username: '',
    password: '',
    privateKey: ''
  }
  emit('resetPackageForm')
}
</script>

<style scoped>
.kubeadm-manager {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

/* 包管理区域 */
.package-section {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

.package-container {
  display: grid;
  grid-template-columns: 1fr 350px;
  gap: 25px;
}

@media (max-width: 1024px) {
  .package-container {
    grid-template-columns: 1fr;
  }
}

.package-form {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.package-form h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 20px 0;
}

.package-form h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 20px 0 15px 0;
}

.package-info {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.package-info h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
}

.package-details {
  background-color: var(--bg-input);
  border-radius: var(--radius-sm);
  padding: 15px;
  border: 1px solid var(--border-color);
}

.detail-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-light);
}

.detail-item:last-child {
  border-bottom: none;
}

.detail-label {
  font-size: 0.9rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.detail-value {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
  font-family: 'Courier New', Courier, monospace;
  word-break: break-all;
}

.no-package {
  text-align: center;
  color: var(--text-muted);
  padding: 20px;
  font-style: italic;
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
  margin-bottom: 15px;
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

/* 加载动画 */
.loading-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top: 2px solid white;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* 日志区域 */
.logs-section {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
  margin-top: 25px;
}

.logs-container {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  max-height: 400px;
  overflow-y: auto;
  padding: 20px;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.9rem;
  line-height: 1.6;
}

.logs-container pre {
  margin: 0;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-all;
}
</style>