<template>
  <div class="kubeadm-manager">
    <!-- 包源管理部分 -->
    <section class="source-section">
      <div class="source-container">
        <div class="source-header">
          <div class="header-left">
            <h3>包源管理</h3>
            <button class="toggle-btn" @click="isSourcesExpanded = !isSourcesExpanded" :title="isSourcesExpanded ? '收起' : '展开'">
              {{ isSourcesExpanded ? '▼' : '▶' }}
            </button>
          </div>
          <button class="btn btn-primary" @click="showAddSourceModal = true">添加新源</button>
        </div>
        
        <div class="sources-list" v-if="isSourcesExpanded">
          <div class="source-item" v-for="(source, index) in packageSources" :key="index">
            <div class="source-info">
              <h4>{{ source.name }}</h4>
              <p class="source-url">{{ source.url }}</p>
              <p class="source-default" v-if="source.default">默认源</p>
            </div>
            <div class="source-actions">
              <button class="btn btn-sm btn-primary" @click="editSource(index)">编辑</button>
              <button class="btn btn-sm btn-danger" @click="deleteSource(index)">删除</button>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- 包下载与部署部分 -->
    <section class="package-section">
      <div class="package-container">
        <div class="package-form">
          <h3>包下载</h3>
          <form @submit.prevent="downloadOnly">
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

            <!-- 包源选择 -->
            <div class="form-row">
              <div class="form-group">
                <label for="packageSourceSelect">选择包源:</label>
                <select id="packageSourceSelect" v-model="selectedSource" required>
                  <option value="">-- 选择包源 --</option>
                  <option v-for="source in packageSources" :key="source.name" :value="source">{{ source.name }}</option>
                </select>
              </div>
            </div>

            <!-- 已下载包列表 -->
            <div class="downloaded-packages">
              <h4>已下载的包</h4>
              <div v-if="downloadedPackages.length > 0" class="packages-list">
                <div class="package-item" v-for="pkg in downloadedPackages" :key="pkg.filePath">
                  <div class="package-info-wrapper" @click="selectDownloadedPackage(pkg)">
                    <div class="package-name">{{ pkg.name }}-{{ pkg.version }}</div>
                    <div class="package-info">{{ pkg.arch }} - {{ pkg.distro }}</div>
                    <div class="package-size">{{ formatFileSize(pkg.size) }}</div>
                  </div>
                  <button class="btn btn-sm btn-danger package-delete-btn" @click.stop="deletePackage(pkg)" :title="'删除 ' + pkg.name + '-' + pkg.version">
                    删除
                  </button>
                </div>
              </div>
              <div v-else class="no-packages">
                <p>暂无已下载的包</p>
              </div>
            </div>

            <div class="form-actions">
              <button type="submit" class="btn btn-primary" :disabled="isDeploying || !selectedVersion">
                <span v-if="isDeploying" class="loading-spinner"></span>
                {{ isDeploying ? '下载中...' : '下载到本地' }}
              </button>
              <button type="button" class="btn btn-secondary" @click="resetPackageForm">重置</button>
            </div>
          
          <!-- 镜像处理部分 -->
          <div class="image-processing">
            <h4>镜像处理</h4>
            <div class="image-actions">
              <button class="btn btn-primary" @click="pullKubernetesImages" :disabled="isDeploying || !selectedVersion">
                <span v-if="isDeploying" class="loading-spinner"></span>
                {{ isDeploying ? '拉取中...' : '拉取K8s镜像到本地' }}
              </button>
              
              <div class="harbor-section">
                <h5>Harbor仓库配置</h5>
                <div class="harbor-config">
                  <div class="form-row">
                    <div class="form-group" style="width: 200px;">
                      <label for="harborEnabled">启用Harbor</label>
                      <input type="checkbox" id="harborEnabled" v-model="harborConfig.enabled">
                    </div>
                  </div>
                  
                  <div class="form-row" v-if="harborConfig.enabled">
                    <div class="form-group">
                      <label for="harborUrl">Harbor URL</label>
                      <input type="text" id="harborUrl" v-model="harborConfig.url" placeholder="https://harbor.example.com">
                    </div>
                  </div>
                  
                  <div class="form-row" v-if="harborConfig.enabled">
                    <div class="form-group">
                      <label for="harborUsername">用户名</label>
                      <input type="text" id="harborUsername" v-model="harborConfig.username" placeholder="admin">
                    </div>
                    <div class="form-group">
                      <label for="harborPassword">密码</label>
                      <input type="password" id="harborPassword" v-model="harborConfig.password" placeholder="密码">
                    </div>
                  </div>
                  
                  <div class="form-row" v-if="harborConfig.enabled">
                    <div class="form-group">
                      <label for="harborProject">项目名称</label>
                      <input type="text" id="harborProject" v-model="harborConfig.project" placeholder="library">
                    </div>
                    <div class="form-group" style="width: 200px;">
                      <label for="harborSkipTls">跳过TLS验证</label>
                      <input type="checkbox" id="harborSkipTls" v-model="harborConfig.skipTls">
                    </div>
                  </div>
                  
                  <div class="form-row" v-if="harborConfig.enabled">
                    <button class="btn btn-primary" @click="pushImagesToHarbor" :disabled="isDeploying || !selectedVersion || !harborConfig.url || !harborConfig.username || !harborConfig.password">
                      <span v-if="isDeploying" class="loading-spinner"></span>
                      {{ isDeploying ? '推送中...' : '推送镜像到Harbor' }}
                    </button>
                  </div>
                </div>
              </div>
            </div>
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

    <!-- 包源管理模态框 -->
    <div v-if="showSourceModal" class="modal-overlay" @click="closeSourceModal">
      <div class="modal-content" @click.stop>
        <div class="modal-header">
          <h3>{{ isEditing ? '编辑包源' : '添加包源' }}</h3>
          <button class="modal-close" @click="closeSourceModal">&times;</button>
        </div>
        <div class="modal-body">
          <form @submit.prevent="saveSource">
            <div class="form-group">
              <label for="sourceName">包源名称:</label>
              <input type="text" id="sourceName" v-model="currentSource.name" placeholder="例如：华为源" required>
            </div>
            <div class="form-group">
              <label for="sourceUrl">包源URL:</label>
              <input type="url" id="sourceUrl" v-model="currentSource.url" placeholder="例如：https://mirrors.huaweicloud.com/kubernetes" required>
            </div>
            <div class="form-group">
              <label class="checkbox-label">
                <input type="checkbox" v-model="currentSource.default">
                设置为默认源
              </label>
            </div>
            <div class="form-actions">
              <button type="button" class="btn btn-secondary" @click="closeSourceModal">取消</button>
              <button type="submit" class="btn btn-primary">{{ isEditing ? '保存' : '添加' }}</button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import axios from 'axios'

// 定义组件的属性和事件
const props = defineProps({
  availableVersions: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['showMessage', 'setKubeadmVersion'])

// API配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 60000 // 60秒超时
})

// 本地状态
const isDeploying = ref(false)
const deployLogs = ref('')
const currentPackage = ref(null)

// 本地状态
const selectedVersion = ref('')
const selectedArch = ref('amd64')
const selectedDistro = ref('ubuntu')

// 包源和已下载包状态
const packageSources = ref([])
const downloadedPackages = ref([])
const selectedSource = ref({})
const selectedDownloadedPackage = ref(null)

// 包源管理模态框状态
const showSourceModal = ref(false)
const isEditing = ref(false)
const currentSource = ref({
  name: '',
  url: '',
  default: false
})
const editingIndex = ref(-1)

// 包源列表收缩状态
const isSourcesExpanded = ref(true)

// Harbor 仓库配置
const harborConfig = ref({
  enabled: false,
  url: '',
  username: '',
  password: '',
  project: 'library',
  skipTls: false
})



// 加载包源列表
const loadPackageSources = async () => {
  try {
    const response = await apiClient.get('/kubeadm/sources')
    if (response.data && Array.isArray(response.data.sources)) {
      packageSources.value = response.data.sources
      // 设置默认包源
      const defaultSource = response.data.sources.find(source => source.default)
      if (defaultSource) {
        selectedSource.value = defaultSource
      }
    }
  } catch (error) {
    emit('showMessage', { text: '获取包源列表失败: ' + error.message, type: 'error' })
  }
}

// 打开添加包源模态框
const openAddSourceModal = () => {
  isEditing.value = false
  currentSource.value = {
    name: '',
    url: '',
    default: false
  }
  editingIndex.value = -1
  showSourceModal.value = true
}

// 编辑包源
const editSource = (index) => {
  isEditing.value = true
  editingIndex.value = index
  currentSource.value = { ...packageSources.value[index] }
  showSourceModal.value = true
}

// 删除包源
const deleteSource = async (index) => {
  if (confirm('确定要删除这个包源吗？')) {
    try {
      await apiClient.delete(`/kubeadm/sources/${index}`)
      await loadPackageSources()
      emit('showMessage', { text: '包源删除成功', type: 'success' })
    } catch (error) {
      emit('showMessage', { text: '删除包源失败: ' + error.message, type: 'error' })
    }
  }
}

// 关闭包源模态框
const closeSourceModal = () => {
  showSourceModal.value = false
  currentSource.value = {
    name: '',
    url: '',
    default: false
  }
  editingIndex.value = -1
}

// 保存包源
const saveSource = async () => {
  try {
    if (isEditing.value) {
      // 更新现有包源
      await apiClient.put(`/kubeadm/sources/${editingIndex.value}`, currentSource.value)
      emit('showMessage', { text: '包源更新成功', type: 'success' })
    } else {
      // 添加新包源
      await apiClient.post('/kubeadm/sources', currentSource.value)
      emit('showMessage', { text: '包源添加成功', type: 'success' })
    }
    await loadPackageSources()
    closeSourceModal()
  } catch (error) {
    emit('showMessage', { text: '保存包源失败: ' + error.message, type: 'error' })
  }
}

// 加载已下载的包列表
const loadDownloadedPackages = async () => {
  try {
    const response = await apiClient.get('/kubeadm/packages/local')
    if (response.data && Array.isArray(response.data.packages)) {
      downloadedPackages.value = response.data.packages
    }
  } catch (error) {
    emit('showMessage', { text: '获取已下载包列表失败: ' + error.message, type: 'error' })
  }
}

// 删除已下载的包
const deletePackage = async (pkg) => {
  if (confirm(`确定要删除包 ${pkg.name}-${pkg.version} 吗？`)) {
    try {
      // 使用 axios.request 方法发送 DELETE 请求，支持 data 选项
      await apiClient.request({
        method: 'delete',
        url: '/kubeadm/packages/local',
        data: {
          name: pkg.name,
          version: pkg.version,
          arch: pkg.arch,
          distro: pkg.distro
        }
      })
      await loadDownloadedPackages()
      emit('showMessage', { text: `包 ${pkg.name}-${pkg.version} 删除成功`, type: 'success' })
    } catch (error) {
      emit('showMessage', { text: '删除包失败: ' + error.message, type: 'error' })
    }
  }
}

// 选择已下载的包
const selectDownloadedPackage = (pkg) => {
  selectedDownloadedPackage.value = pkg
  // 自动填充版本、架构和发行版
  selectedVersion.value = pkg.version
  selectedArch.value = pkg.arch
  selectedDistro.value = pkg.distro
  emit('showMessage', { text: `已选择包: ${pkg.name}-${pkg.version}`, type: 'info' })
}

// 格式化文件大小
const formatFileSize = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 仅下载 Kubeadm 包到本地
const downloadOnly = async () => {
  isDeploying.value = true
  deployLogs.value = ''

  try {
    // 使用后端API下载包
    deployLogs.value = `正在下载包: kubeadm-${selectedVersion.value}-${selectedArch.value}-${selectedDistro.value}`
    
    const response = await apiClient.post('/kubeadm/packages/download', {
      version: selectedVersion.value,
      arch: selectedArch.value,
      distro: selectedDistro.value,
      sourceURL: selectedSource.value?.url || ''
    })
    
    // 保存当前包信息
    currentPackage.value = {
      version: selectedVersion.value,
      arch: selectedArch.value,
      distro: selectedDistro.value,
      path: response.data.packagePath
    }

    // 通知父组件设置 Kubeadm 版本
    emit('setKubeadmVersion', selectedVersion.value)
    emit('showMessage', { text: `Kubeadm 包 ${selectedVersion.value} 下载成功!`, type: 'success' })
    deployLogs.value += '\n下载完成'
    
    // 重新加载已下载包列表
    await loadDownloadedPackages()
  } catch (error) {
    deployLogs.value = '下载失败: ' + (error.response?.data?.error || error.message)
    emit('showMessage', { text: '下载失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}



// 重置表单
const resetPackageForm = () => {
  selectedVersion.value = ''
  selectedArch.value = 'amd64'
  selectedDistro.value = 'ubuntu'
  selectedSource.value = {}
  selectedDownloadedPackage.value = null
  deployLogs.value = ''
}

// 拉取Kubernetes镜像到本地
const pullKubernetesImages = async () => {
  if (!selectedVersion.value) {
    emit('showMessage', { text: '请先选择Kubernetes版本', type: 'warning' })
    return
  }

  isDeploying.value = true
  deployLogs.value = `正在拉取Kubernetes ${selectedVersion.value} 镜像...`

  try {
    const response = await apiClient.post('/kubeadm/images/pull', {
      version: selectedVersion.value
    })
    deployLogs.value += '\n' + response.data.result
    emit('showMessage', { text: `Kubernetes ${selectedVersion.value} 镜像拉取成功`, type: 'success' })
  } catch (error) {
    deployLogs.value += '\n拉取失败: ' + (error.response?.data?.error || error.message)
    emit('showMessage', { text: '拉取Kubernetes镜像失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 推送Kubernetes镜像到Harbor仓库
const pushImagesToHarbor = async () => {
  if (!selectedVersion.value) {
    emit('showMessage', { text: '请先选择Kubernetes版本', type: 'warning' })
    return
  }

  if (!harborConfig.value.enabled) {
    emit('showMessage', { text: '请先启用Harbor配置', type: 'warning' })
    return
  }

  if (!harborConfig.value.url || !harborConfig.value.username || !harborConfig.value.password) {
    emit('showMessage', { text: '请填写完整的Harbor配置信息', type: 'warning' })
    return
  }

  isDeploying.value = true
  deployLogs.value = `正在推送Kubernetes ${selectedVersion.value} 镜像到Harbor仓库...`

  try {
    const response = await apiClient.post('/kubeadm/images/push-to-harbor', {
      harborConfig: harborConfig.value,
      kubernetesVersion: selectedVersion.value
    })
    deployLogs.value += '\n' + response.data.result
    emit('showMessage', { text: `Kubernetes ${selectedVersion.value} 镜像推送成功`, type: 'success' })
  } catch (error) {
    deployLogs.value += '\n推送失败: ' + (error.response?.data?.error || error.message)
    emit('showMessage', { text: '推送Kubernetes镜像失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  } finally {
    isDeploying.value = false
  }
}

// 页面加载时获取包源列表和已下载包列表
onMounted(async () => {
  await loadPackageSources()
  await loadDownloadedPackages()
})
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

/* 已下载包样式 */
.downloaded-packages {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 15px;
  margin: 20px 0;
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-sm);
}

.downloaded-packages h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
}

.packages-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.package-item {
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  padding: 12px 15px;
  transition: all 0.3s ease;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.package-info-wrapper {
  cursor: pointer;
  flex: 1;
}

.package-delete-btn {
  flex-shrink: 0;
}

.package-item:hover {
  background-color: var(--bg-secondary);
  border-color: var(--primary-color);
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
}

.package-name {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 0.95rem;
}

.package-info {
  color: var(--text-secondary);
  font-size: 0.85rem;
}

.package-size {
  color: var(--text-muted);
  font-size: 0.8rem;
}

.no-packages {
  text-align: center;
  color: var(--text-muted);
  padding: 20px;
  font-style: italic;
  font-size: 0.9rem;
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

.btn-success {
  background-color: var(--success-color);
  color: white;
}

.btn-success:hover:not(:disabled) {
  background-color: var(--success-color);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(39, 174, 96, 0.3);
  filter: brightness(0.95);
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

/* 包源管理样式 */
.source-section {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

.source-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.source-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 15px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.source-header h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.toggle-btn {
  background: none;
  border: none;
  color: var(--text-secondary);
  font-size: 0.8rem;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.toggle-btn:hover {
  color: var(--primary-color);
  background-color: var(--bg-input);
}

.toggle-btn:active {
  transform: scale(0.95);
}

.sources-list {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.source-item {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
  transition: all 0.3s ease;
}

.source-item:hover {
  border-color: var(--primary-color);
  box-shadow: var(--shadow-md);
  transform: translateY(-1px);
}

.source-info {
  flex: 1;
}

.source-info h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px 0;
}

.source-url {
  font-size: 0.9rem;
  color: var(--text-secondary);
  margin: 0 0 8px 0;
  word-break: break-all;
  font-family: 'Courier New', Courier, monospace;
}

.source-default {
  display: inline-block;
  background-color: var(--primary-color);
  color: white;
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 0.8rem;
  font-weight: 500;
  margin: 0;
}

.source-actions {
  display: flex;
  gap: 10px;
  flex-shrink: 0;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 0.85rem;
}

/* 模态框样式 */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(4px);
}

.modal-content {
  background-color: var(--bg-card);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  width: 90%;
  max-width: 500px;
  max-height: 90vh;
  overflow-y: auto;
  border: 1px solid var(--border-color);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 25px;
  border-bottom: 1px solid var(--border-color);
}

.modal-header h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.modal-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 0;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  transition: all 0.3s ease;
}

.modal-close:hover {
  background-color: var(--bg-secondary);
  color: var(--text-primary);
}

.modal-body {
  padding: 25px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-weight: 500;
  color: var(--text-primary);
  user-select: none;
}

.checkbox-label input[type="checkbox"] {
  width: 18px;
  height: 18px;
  accent-color: var(--primary-color);
  cursor: pointer;
}
</style>