<template>
  <div class="deployment-manager">
    <h2>部署流程管理</h2>
    
    <!-- 简化的部署源管理，只展示当前选择的源 -->
    <div class="source-switcher simple">
      <div class="section-header">
        <h3>部署源管理</h3>
      </div>
      <div class="source-options simple">
        <div class="source-option simple">
          <div class="source-label">{{ deploymentSources[activeSystem] && deploymentSources[activeSystem].length > 0 && selectedSources[activeSystem] ? deploymentSources[activeSystem].find(source => source.id === selectedSources[activeSystem])?.name || '默认源' : '默认源' }}</div>
          <div class="source-url">{{ deploymentSources[activeSystem] && deploymentSources[activeSystem].length > 0 && selectedSources[activeSystem] ? deploymentSources[activeSystem].find(source => source.id === selectedSources[activeSystem])?.url || 'https://pkgs.k8s.io/' : 'https://pkgs.k8s.io/' }}</div>
        </div>
      </div>
    </div>
    
    <!-- 简化的部署流程列表，只展示基本步骤信息 -->
    <div class="process-list simple">
      <div class="section-header">
        <h3>部署流程列表</h3>
        <div class="header-actions">
          <button 
            class="btn" 
            style="background-color: var(--primary-color); color: white; margin-right: 8px;" 
            @click="syncScriptsToBackend" 
            :disabled="isSyncing"
            title="将当前脚本同步到后端"
          >
            <span v-if="isSyncing" class="loading-spinner"></span>
            <span v-else>📤</span>
            {{ isSyncing ? '同步中...' : '同步到后端' }}
          </button>
          <button 
            class="btn btn-sync" 
            @click="resetScriptsToDefault" 
            :disabled="isSyncing"
            title="将所有脚本重置为后端默认值"
          >
            <span v-if="isSyncing" class="loading-spinner"></span>
            <span v-else>🔄</span>
            {{ isSyncing ? '重置中...' : '重置所有脚本' }}
          </button>
        </div>
        <div class="system-tabs simple">
          <button 
            v-for="system in systems" 
            :key="system" 
            class="tab-btn" 
            :class="{ active: activeSystem === system }"
            @click="activeSystem = system"
          >
            {{ system }}
          </button>
        </div>
      </div>
      
      <div class="process-steps simple">
        <div 
          v-for="(step, index) in currentProcess.steps" 
          :key="index" 
          class="process-step simple"
        >
          <div class="step-header">
            <div class="step-number">{{ index + 1 }}</div>
            <div class="step-info">
              <h4>{{ step.name || '未命名步骤' }}</h4>
              <p class="step-description">{{ step.description || '无描述' }}</p>
            </div>
            <button 
              class="btn btn-small btn-primary edit-script-btn"
              @click="editScript(step, index)"
            >
              <span class="btn-icon">✏️</span>
              编辑脚本
            </button>
          </div>
          <div class="step-script">
            <h5>脚本内容</h5>
            <pre>{{ step.script || '无脚本内容' }}</pre>
          </div>
        </div>
      </div>
      
      <!-- 同步结果提示 -->
      <div v-if="syncResult" class="sync-result" :class="{ 'sync-success': syncResult.success, 'sync-failed': !syncResult.success }">
        <div class="sync-result-header">
          <span class="sync-icon">{{ syncResult.success ? '✅' : '❌' }}</span>
          <span class="sync-message">{{ syncResult.message }}</span>
          <span class="sync-time">{{ syncResult.time }}</span>
        </div>
      </div>
    </div>
  </div>
  
  <!-- 脚本编辑对话框 -->
  <div v-if="showEditScriptDialog" class="dialog-overlay" @click="closeEditScriptDialog">
    <div class="dialog-content dialog-large" @click.stop>
      <div class="dialog-header">
        <h4>{{ currentEditingStep ? `编辑步骤: ${currentEditingStep.name}` : '编辑脚本' }}</h4>
        <button class="dialog-close" @click="closeEditScriptDialog">&times;</button>
      </div>
      <div class="dialog-body">
        <div class="form-group">
          <label for="editScriptTextarea">脚本内容:</label>
          <textarea 
            id="editScriptTextarea" 
            class="form-textarea" 
            v-model="editingScript" 
            placeholder="输入部署脚本..."
            rows="15"
          ></textarea>
        </div>
      </div>
      <div class="dialog-footer">
        <button class="btn btn-secondary" @click="closeEditScriptDialog">取消</button>
        <button class="btn" style="background-color: var(--warning-color); color: white;" @click="restoreDefaultScript">恢复默认值</button>
        <button class="btn btn-primary" @click="saveScript">保存脚本</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import axios from 'axios'

// 定义version变量，用于模板字符串解析，避免ReferenceError
const version = 'v1.28'

// localStorage辅助函数
const loadFromLocalStorage = (key, defaultValue) => {
  try {
    const stored = localStorage.getItem(key)
    return stored ? JSON.parse(stored) : defaultValue
  } catch (error) {
    // 静默处理localStorage错误
    return defaultValue
  }
}

const saveToLocalStorage = (key, value) => {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch (error) {
    // 静默处理localStorage错误
  }
}

// API基础URL
const API_BASE_URL = 'http://localhost:8080'

// 部署源管理相关状态
const defaultDeploymentSources = {
  centos: [
    {
      id: 'centos-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'centos-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ],
  ubuntu: [
    {
      id: 'ubuntu-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'ubuntu-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ],
  debian: [
    {
      id: 'debian-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'debian-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ],
  rocky: [
    {
      id: 'rocky-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'rocky-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ],
  almalinux: [
    {
      id: 'almalinux-1',
      name: '官方源',
      url: 'https://pkgs.k8s.io/'
    },
    {
      id: 'almalinux-2',
      name: '阿里云镜像源',
      url: 'https://mirrors.aliyun.com/kubernetes-new/'
    }
  ]
}

const deploymentSources = ref(loadFromLocalStorage('deploymentSources', defaultDeploymentSources))

// 按发行版本存储选中的源
const defaultSelectedSources = {
  centos: 'centos-1',
  ubuntu: 'ubuntu-1',
  debian: 'debian-1',
  rocky: 'rocky-1',
  almalinux: 'almalinux-1'
}

const selectedSources = ref(loadFromLocalStorage('selectedSources', defaultSelectedSources))

// 脚本编辑相关状态
const showEditScriptDialog = ref(false)
const currentEditingStepIndex = ref(-1)
const currentEditingStep = ref(null)
const editingScript = ref('')

// 支持的系统类型
const systems = ref(['centos', 'ubuntu', 'debian', 'rocky', 'almalinux'])
const activeDistro = ref('centos')

// 定义组件的属性和事件
const props = defineProps({
  availableVersions: { type: Array, default: () => [] },
  kubeadmVersion: { type: String, default: '' },
  nodes: { type: Array, default: () => [] },
  systemOnline: { type: Boolean, default: true },
  apiStatus: { type: String, default: 'online' }
})

const emit = defineEmits(['showMessage'])

// 确保activeSystem的初始值是有效的
const activeSystem = ref(systems.value[0] || 'centos')

// 支持的Kubernetes版本
const kubernetesVersions = ref(['v1.28', 'v1.29', 'v1.30'])
const selectedKubernetesVersion = ref(loadFromLocalStorage('selectedKubernetesVersion', 'v1.28'))

// 简化的部署流程默认数据
const defaultProcessData = {
  centos: {
    name: 'CentOS/RHEL 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点'
      },
      {
        name: '生成Worker节点加入命令',
        description: '在Master节点上生成kubeadm join命令'
      },
      {
        name: 'Worker节点加入集群',
        description: '执行kubeadm join将Worker节点加入集群'
      }
    ]
  },
  ubuntu: {
    name: 'Ubuntu 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点'
      },
      {
        name: '生成Worker节点加入命令',
        description: '在Master节点上生成kubeadm join命令'
      },
      {
        name: 'Worker节点加入集群',
        description: '执行kubeadm join将Worker节点加入集群'
      }
    ]
  },
  debian: {
    name: 'Debian 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点'
      },
      {
        name: '生成Worker节点加入命令',
        description: '在Master节点上生成kubeadm join命令'
      },
      {
        name: 'Worker节点加入集群',
        description: '执行kubeadm join将Worker节点加入集群'
      }
    ]
  },
  rocky: {
    name: 'Rocky Linux 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点'
      },
      {
        name: '生成Worker节点加入命令',
        description: '在Master节点上生成kubeadm join命令'
      },
      {
        name: 'Worker节点加入集群',
        description: '执行kubeadm join将Worker节点加入集群'
      }
    ]
  },
  almalinux: {
    name: 'AlmaLinux 部署流程',
    steps: [
      {
        name: '系统准备',
        description: '禁用swap、配置时间同步、关闭防火墙等'
      },
      {
        name: '安装容器运行时',
        description: '安装containerd容器运行时'
      },
      {
        name: '配置容器运行时',
        description: '配置containerd并启动服务'
      },
      {
        name: '添加Kubernetes仓库',
        description: '添加官方Kubernetes仓库'
      },
      {
        name: '安装Kubernetes组件',
        description: '安装kubelet、kubeadm和kubectl'
      },
      {
        name: '初始化Kubernetes集群',
        description: '执行kubeadm init初始化Master节点'
      },
      {
        name: '生成Worker节点加入命令',
        description: '在Master节点上生成kubeadm join命令'
      },
      {
        name: 'Worker节点加入集群',
        description: '执行kubeadm join将Worker节点加入集群'
      }
    ]
  }
}

// 从localStorage加载processData，如果无效则使用默认数据
const loadProcessDataFromStorage = () => {
  const storedData = loadFromLocalStorage('processData', null)
  if (storedData && typeof storedData === 'object' && Object.keys(storedData).length > 0) {
    // 验证数据结构
    for (const system of systems.value) {
      if (!storedData[system] || !storedData[system].steps) {
        storedData[system] = defaultProcessData[system]
      }
    }
    return storedData
  }
  return defaultProcessData
}

// 初始化processData
const processData = ref(loadProcessDataFromStorage())

// 计算属性：当前激活的系统流程
const currentProcess = computed(() => {
  // 添加默认值，确保始终返回一个有效的对象
  const systemProcess = processData.value[activeSystem.value]
  if (systemProcess) {
    return {
      ...systemProcess,
      steps: systemProcess.steps || []
    }
  }
  return {
    name: '默认部署流程',
    steps: []
  }
})

// 编辑脚本
const editScript = (step, index) => {
  currentEditingStepIndex.value = index
  currentEditingStep.value = step
  editingScript.value = step.script || ''
  showEditScriptDialog.value = true
}

// 关闭脚本编辑对话框
const closeEditScriptDialog = () => {
  showEditScriptDialog.value = false
  currentEditingStepIndex.value = -1
  currentEditingStep.value = null
  editingScript.value = ''
}

// 保存脚本
const saveScript = () => {
  if (currentEditingStepIndex.value >= 0 && currentEditingStep.value) {
    // 更新当前步骤的脚本
    processData.value[activeSystem.value].steps[currentEditingStepIndex.value].script = editingScript.value
    
    // 保存到localStorage
    saveToLocalStorage('processData', processData.value)
    
    // 显示成功消息
    emit('showMessage', { text: '脚本保存成功!', type: 'success' })
    
    // 关闭对话框
    closeEditScriptDialog()
  }
}

// 恢复默认脚本
const restoreDefaultScript = async () => {
  if (!currentEditingStep.value) return
  
  try {
    // 根据步骤名称确定对应的脚本名称
    let scriptName = ''
    switch (currentEditingStep.value.name) {
      case '系统准备':
        scriptName = 'system_prep'
        break
      case '安装容器运行时':
        scriptName = 'containerd_install'
        break
      case '配置容器运行时':
        scriptName = 'containerd_config'
        break
      case '添加Kubernetes仓库':
      case '安装Kubernetes组件':
        scriptName = 'k8s_components'
        break
      case '初始化Kubernetes集群':
        scriptName = 'k8s_init'
        break
      case '生成Worker节点加入命令':
        scriptName = 'k8s_init' // 使用相同的脚本，因为join命令是在master初始化后生成的
        break
      case 'Worker节点加入集群':
        scriptName = 'k8s_join'
        break
      default:
        scriptName = 'system_prep'
    }
    
    // 调用API获取默认脚本
    const response = await apiClient.get(`/deployment-process/scripts/${scriptName}/default`)
    
    if (response.data.status === 'success') {
      // 更新编辑框中的脚本内容
      editingScript.value = response.data.scriptContent
      
      // 显示成功消息
      emit('showMessage', { text: '脚本已恢复为默认值!', type: 'success' })
    } else {
      throw new Error(response.data.message || '恢复默认脚本失败')
    }
  } catch (error) {
    console.error('恢复默认脚本失败:', error)
    emit('showMessage', { text: `恢复默认脚本失败: ${error.message}`, type: 'error' })
  }
}

// API配置
const apiClient = axios.create({
  baseURL: 'http://localhost:8080',
  timeout: 600000 // 10分钟超时
})

// 同步状态变量
const isSyncing = ref(false)
const syncResult = ref(null)

// 重置所有脚本为后端默认脚本
const resetScriptsToDefault = async () => {
  if (isSyncing.value) return
  
  if (!confirm('确定要将所有脚本重置为后端的默认脚本吗？\n这将覆盖当前所有自定义脚本。')) {
    return
  }
  
  isSyncing.value = true
  syncResult.value = null
  
  try {
    const response = await apiClient.post('/deployment-process/scripts/reset')
    
    if (response.data.status === 'scripts reset to default') {
      // 重置成功后，从后端重新加载脚本
      await loadDefaultScripts()
      
      syncResult.value = {
        success: true,
        message: `成功重置 ${response.data.scriptsCount} 个脚本为默认值`,
        time: new Date().toLocaleString('zh-CN')
      }
      
      emit('showMessage', { text: syncResult.value.message, type: 'success' })
    } else {
      throw new Error(response.data.error || '重置失败')
    }
  } catch (error) {
    console.error('重置脚本失败:', error)
    syncResult.value = {
      success: false,
      message: `重置失败: ${error.message || '未知错误'}`,
      time: new Date().toLocaleString('zh-CN')
    }
    emit('showMessage', { text: syncResult.value.message, type: 'error' })
  } finally {
    isSyncing.value = false
  }
}

// 将当前脚本同步到后端
const syncScriptsToBackend = async () => {
  if (isSyncing.value) return
  
  if (!confirm('确定要将当前脚本同步到后端吗？\n这将更新后端存储的脚本。')) {
    return
  }
  
  isSyncing.value = true
  syncResult.value = null
  
  try {
    // 收集所有脚本，按脚本名称组织
    const scriptsToSync = {}
    
    // 遍历所有系统的步骤，提取脚本内容
    for (const system of systems.value) {
      if (processData.value[system] && processData.value[system].steps) {
        processData.value[system].steps.forEach(step => {
          // 根据步骤名称确定脚本名称
          let scriptName = ''
          switch (step.name) {
            case '系统准备':
              scriptName = 'system_prep'
              break
            case '安装容器运行时':
              scriptName = 'containerd_install'
              break
            case '配置容器运行时':
              scriptName = 'containerd_config'
              break
            case '添加Kubernetes仓库':
            case '安装Kubernetes组件':
              scriptName = 'k8s_components'
              break
            case '初始化Kubernetes集群':
            case '生成Worker节点加入命令':
              scriptName = 'k8s_init'
              break
            case 'Worker节点加入集群':
              scriptName = 'k8s_join'
              break
            default:
              return // 跳过未知步骤
          }
          
          // 只同步有脚本内容的步骤
          if (step.script) {
            scriptsToSync[scriptName] = step.script
          }
        })
      }
    }
    
    // 调用API保存脚本
    const response = await apiClient.post('/deployment-process/scripts', scriptsToSync)
    
    if (response.data.status === 'scripts saved successfully') {
      syncResult.value = {
        success: true,
        message: `成功同步 ${Object.keys(scriptsToSync).length} 个脚本到后端`,
        time: new Date().toLocaleString('zh-CN')
      }
      
      emit('showMessage', { text: syncResult.value.message, type: 'success' })
    } else {
      throw new Error(response.data.message || '同步失败')
    }
  } catch (error) {
    console.error('同步脚本到后端失败:', error)
    syncResult.value = {
      success: false,
      message: `同步失败: ${error.message || '未知错误'}`,
      time: new Date().toLocaleString('zh-CN')
    }
    emit('showMessage', { text: syncResult.value.message, type: 'error' })
  } finally {
    isSyncing.value = false
  }
}

// 从后端获取所有默认脚本并填充到步骤中
const loadDefaultScripts = async () => {
  try {
    // 调用API获取所有默认脚本
    const response = await apiClient.get('/deployment-process/scripts')
    const allScripts = response.data.scripts
    
    // 遍历所有系统的步骤，填充对应脚本
    for (const system of systems.value) {
      if (processData.value[system] && processData.value[system].steps) {
        processData.value[system].steps.forEach(step => {
          // 根据步骤名称确定脚本名称
          let scriptName = ''
          switch (step.name) {
            case '系统准备':
              scriptName = 'system_prep'
              break
            case '安装容器运行时':
              scriptName = 'containerd_install'
              break
            case '配置容器运行时':
              scriptName = 'containerd_config'
              break
            case '添加Kubernetes仓库':
            case '安装Kubernetes组件':
              scriptName = 'k8s_components'
              break
            case '初始化Kubernetes集群':
            case '生成Worker节点加入命令':
              scriptName = 'k8s_init'
              break
            case 'Worker节点加入集群':
              scriptName = 'k8s_join'
              break
            default:
              scriptName = ''
          }
          
          // 如果找到对应的脚本且步骤还没有脚本内容，则填充
          if (scriptName && allScripts[scriptName] && !step.script) {
            step.script = allScripts[scriptName]
          }
        })
      }
    }
    
    // 保存到localStorage
    saveToLocalStorage('processData', processData.value)
  } catch (error) {
    console.error('加载默认脚本失败:', error)
    // 如果加载失败，不影响页面显示，继续使用现有数据
  }
}

// 页面加载时初始化数据
onMounted(async () => {
  // 确保processData是有效的
  if (!processData.value || typeof processData.value !== 'object') {
    processData.value = defaultProcessData
  }
  
  // 确保activeSystem是有效的
  if (!activeSystem.value || !systems.value.includes(activeSystem.value)) {
    activeSystem.value = systems.value[0] || 'centos'
  }
  
  // 确保每个系统都有有效的流程数据
  for (const system of systems.value) {
    if (!processData.value[system] || !processData.value[system].steps) {
      processData.value[system] = defaultProcessData[system]
    }
  }
  
  // 从后端加载默认脚本并填充到步骤中
  await loadDefaultScripts()
})
</script>

<style scoped>
/* 基础样式重置和布局 */
.deployment-manager {
  padding: 20px;
  background: var(--bg-primary);
  flex: 1;
  overflow-x: hidden;
  overflow-y: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
}

/* 确保主容器可以滚动 */
.deployment-manager::-webkit-scrollbar {
  width: 8px;
}

.deployment-manager::-webkit-scrollbar-track {
  background: var(--bg-secondary);
  border-radius: 4px;
}

.deployment-manager::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 4px;
  transition: all 0.3s ease;
}

.deployment-manager::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
  transform: scale(1.1);
}

/* 页面标题 */
.deployment-manager h2 {
  font-size: 1.8rem;
  margin-bottom: 28px;
  color: var(--text-primary);
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 2px solid var(--primary-color);
  background: linear-gradient(135deg, rgba(52, 152, 219, 0.1), transparent);
  padding: 16px 20px;
  border-radius: var(--radius-md);
  box-shadow: 0 2px 8px rgba(52, 152, 219, 0.15);
}

.deployment-manager h2::before {
  content: '🔧';
  font-size: 1.9rem;
  text-shadow: 0 2px 4px rgba(52, 152, 219, 0.3);
}

/* 区块样式 */
.source-switcher, .process-list {
  background: linear-gradient(135deg, var(--bg-secondary) 0%, rgba(255, 255, 255, 0.05) 100%);
  border-radius: var(--radius-lg);
  padding: 24px;
  margin-bottom: 24px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
  transition: all 0.3s ease;
}

.source-switcher:hover, .process-list:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-1px);
}

/* 简化版本样式 */
.source-switcher.simple, .process-list.simple {
  padding: 16px;
  margin-bottom: 16px;
}

.source-options.simple {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.source-option.simple {
  display: flex;
  flex-direction: column;
  padding: 12px;
  background: rgba(52, 152, 219, 0.1);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.source-option.simple .source-label {
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--primary-color);
}

.source-option.simple .source-url {
  font-size: 0.85rem;
  color: var(--text-secondary);
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  word-break: break-all;
}

.system-tabs.simple {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 8px;
}

.process-steps.simple {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.process-step.simple {
  background: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 12px;
  border: 1px solid var(--border-color);
}

.process-step.simple .step-header {
  flex-direction: row;
  gap: 12px;
}

.process-step.simple .step-info {
  flex: 1;
}

.step-script {
  margin-top: 16px;
  padding: 16px;
  background-color: var(--bg-input);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-color);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
}

.step-script:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
  border-color: var(--primary-color);
  transform: translateY(-1px);
}

.step-script h5 {
  margin: 0 0 12px 0;
  font-size: 1rem;
  color: var(--text-primary);
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border-color);
}

.step-script h5::before {
  content: '📝';
  font-size: 1.1rem;
}

.step-script pre {
  margin: 0;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.9rem;
  line-height: 1.6;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-wrap: break-word;
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-color);
  padding: 16px;
  overflow-x: auto;
  max-height: 250px;
  overflow-y: auto;
  border-radius: var(--radius-md);
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
}

.step-script pre:hover {
  background-color: rgba(52, 152, 219, 0.05);
  border-color: var(--primary-color);
}

/* 区块标题 */
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 16px;
}

.section-header h3 {
  font-size: 1.1rem;
  margin: 0;
  color: var(--text-primary);
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-header h3::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

/* 头部操作按钮容器 */
.header-actions {
  display: flex;
  gap: 8px;
  margin-right: auto;
}

/* 同步按钮样式 */
.btn-sync {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  border: none;
  border-radius: var(--radius-sm);
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: 0 2px 8px rgba(102, 126, 234, 0.4);
}

.btn-sync:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.6);
}

.btn-sync:disabled {
  opacity: 0.6;
  cursor: not-allowed;
  transform: none;
}

/* 同步结果提示 */
.sync-result {
  margin-top: 16px;
  padding: 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  animation: slideIn 0.3s ease;
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.sync-result.sync-success {
  background: rgba(46, 204, 113, 0.1);
  border-color: var(--secondary-color);
}

.sync-result.sync-failed {
  background: rgba(231, 76, 60, 0.1);
  border-color: var(--error-color);
}

.sync-result-header {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.sync-icon {
  font-size: 1.2rem;
}

.sync-message {
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
}

.sync-time {
  font-size: 0.8rem;
  color: var(--text-muted);
}

/* 表单元素 */
.form-input {
  padding: 10px 14px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  font-size: 0.9rem;
  background: var(--bg-input);
  color: var(--text-primary);
  transition: all 0.3s ease;
  width: 100%;
  box-sizing: border-box;
}

.form-input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

/* 按钮样式 */
.btn {
  padding: 10px 20px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.3s ease;
  background: var(--bg-card);
  color: var(--text-primary);
  position: relative;
  overflow: hidden;
}

.btn::before {
  content: '';
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.2), transparent);
  transition: left 0.5s ease;
}

.btn:hover::before {
  left: 100%;
}

.btn:hover {
  border-color: var(--primary-color);
  transform: translateY(-1px);
  box-shadow: var(--shadow-sm);
}

.btn-primary {
  background: linear-gradient(135deg, var(--primary-color), var(--primary-color-light));
  color: white;
  border-color: var(--primary-color);
}

.btn-primary:hover {
  background: linear-gradient(135deg, var(--primary-color-dark), var(--primary-color));
  border-color: var(--primary-color-dark);
}

.btn-secondary {
  background: linear-gradient(135deg, var(--bg-secondary), var(--bg-card));
  color: var(--text-primary);
  border-color: var(--border-color);
}

.btn-secondary:hover {
  background: linear-gradient(135deg, var(--bg-card), var(--bg-input));
  border-color: var(--primary-color);
}

.btn-small {
  padding: 6px 12px;
  font-size: 0.8rem;
}

/* 编辑脚本按钮样式 */
.edit-script-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  font-size: 0.85rem;
  font-weight: 600;
  background: linear-gradient(135deg, var(--primary-color), var(--primary-dark));
  border: none;
  color: white;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: 0 2px 8px rgba(52, 152, 219, 0.3);
}

.edit-script-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(52, 152, 219, 0.4);
  background: linear-gradient(135deg, var(--primary-dark), var(--primary-color));
}

.edit-script-btn:active {
  transform: translateY(0);
  box-shadow: 0 2px 6px rgba(52, 152, 219, 0.3);
}

.btn-icon {
  font-size: 1rem;
  line-height: 1;
}

/* 标签页样式 */
.system-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.tab-btn {
  padding: 8px 16px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-card);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 0.9rem;
  position: relative;
  overflow: hidden;
}

.tab-btn::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 2px;
  background: var(--primary-color);
  transition: all 0.3s ease;
}

.tab-btn:hover {
  color: var(--text-primary);
  border-color: var(--primary-color);
}

.tab-btn.active {
  color: var(--primary-color);
  border-color: var(--primary-color);
  background: rgba(52, 152, 219, 0.1);
}

.tab-btn.active::after {
  width: 100%;
}

/* 步骤样式 */
.process-step {
  margin-bottom: 20px;
  background: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  border: 1px solid var(--border-color);
  transition: all 0.3s ease;
}

.process-step:hover {
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
}

.step-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  position: relative;
  padding: 12px 16px;
  background: linear-gradient(135deg, var(--bg-secondary), transparent);
  border-radius: var(--radius-md);
  border-left: 4px solid var(--primary-color);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
}

.step-header:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
  transform: translateX(4px);
}

.step-number {
  width: 40px;
  height: 40px;
  background: linear-gradient(135deg, var(--primary-color), var(--primary-dark));
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 1.2rem;
  flex-shrink: 0;
  box-shadow: 0 4px 12px rgba(52, 152, 219, 0.3);
  transition: all 0.3s ease;
  border: 2px solid var(--bg-secondary);
}

.step-number:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 16px rgba(52, 152, 219, 0.4);
}

.step-info {
  flex: 1;
  min-width: 0;
}

.step-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  flex-wrap: wrap;
  gap: 12px;
}

.step-title-row h4 {
  margin: 0;
  font-size: 1.2rem;
  color: var(--text-primary);
  font-weight: 600;
  transition: all 0.3s ease;
  line-height: 1.3;
}

.step-title-row h4:hover {
  color: var(--primary-color);
}

.step-description {
  font-size: 0.95rem;
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.6;
  background: rgba(52, 152, 219, 0.05);
  padding: 12px 16px;
  border-radius: var(--radius-md);
  border-left: 3px solid var(--primary-color);
  transition: all 0.3s ease;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.step-description:hover {
  background: rgba(52, 152, 219, 0.1);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);
}

/* 脚本编辑对话框 */
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  animation: fadeIn 0.3s ease;
}

.dialog-content {
  background: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 24px;
  box-shadow: var(--shadow-xl);
  border: 1px solid var(--border-color);
  max-width: 90vw;
  max-height: 90vh;
  overflow-y: auto;
  animation: slideUp 0.3s ease;
}

.dialog-large {
  max-width: 80vw;
  width: 800px;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-color);
}

.dialog-header h4 {
  margin: 0;
  font-size: 1.2rem;
  color: var(--text-primary);
  font-weight: 600;
}

.dialog-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  transition: all 0.3s ease;
}

.dialog-close:hover {
  background: var(--bg-input);
  color: var(--text-primary);
  transform: rotate(90deg);
}

.dialog-body {
  margin-bottom: 20px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid var(--border-color);
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-size: 0.9rem;
  color: var(--text-primary);
  font-weight: 500;
}

.form-textarea {
  width: 100%;
  min-height: 200px;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  font-size: 0.9rem;
  background: var(--bg-input);
  color: var(--text-primary);
  resize: vertical;
  transition: all 0.3s ease;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  line-height: 1.6;
}

.form-textarea:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

/* 空状态样式 */
.empty-state {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-secondary);
  background: var(--bg-card);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.empty-state p {
  margin: 8px 0;
  font-size: 0.95rem;
}

/* 动画效果 */
@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* 响应式设计 */
@media (max-width: 768px) {
  .deployment-manager {
    padding: 12px;
  }
  
  .section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  
  .system-tabs {
    width: 100%;
  }
  
  .tab-btn {
    flex: 1;
    min-width: 80px;
    text-align: center;
  }
  
  .step-title-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .dialog-content {
    padding: 16px;
    margin: 12px;
  }
  
  .dialog-large {
    width: calc(100vw - 24px);
  }
}
</style>