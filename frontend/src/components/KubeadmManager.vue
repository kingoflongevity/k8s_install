<template>
  <div class="kubeadm-manager">
    <!-- 部署流程页面主容器 -->
    <section class="dashboard-section">
      <h2>Kubernetes集群部署</h2>
      
      <!-- 部署步骤指示器 -->
      <div class="steps-indicator">
        <div 
          v-for="(step, index) in (steps || [])" 
          :key="index" 
          class="step-item"
          :class="{
            'active': currentStep === index,
            'completed': currentStep > index,
            'failed': step.status === 'failed'
          }"
        >
          <div class="step-number">{{ index + 1 }}</div>
          <div class="step-title">{{ step.title }}</div>
          <div class="step-status" v-if="step.status">
            {{ step.status === 'completed' ? '✓' : step.status === 'failed' ? '✗' : '' }}
          </div>
        </div>
      </div>
      
      <!-- 步骤内容 -->
      <div class="step-content">
      <!-- 步骤1: 选择节点 -->
      <div v-if="currentStep === 0" class="step-node-selection">
        <h3>选择节点</h3>
        <div class="node-selection-container">
          <div class="node-filters">
            <div class="form-row">
              <div class="form-group">
                <label for="runtime-filter">容器运行时:</label>
                <select id="runtime-filter" v-model="selectedRuntimeFilter">
                  <option value="">所有</option>
                  <option value="containerd">Containerd</option>
                  <option value="cri-o">CRI-O</option>
                </select>
              </div>
              <div class="form-group">
                <label for="status-filter">状态:</label>
                <select id="status-filter" v-model="selectedStatusFilter">
                  <option value="">所有</option>
                  <option value="ready">就绪</option>
                  <option value="not-ready">未就绪</option>
                </select>
              </div>
            </div>
          </div>
          
          <div class="available-nodes">
            <h4>可用节点</h4>
            <div class="nodes-grid">
              <div 
                v-for="node in filteredNodes" 
                :key="node.id"
                class="node-card"
                :class="{
                  'selected': selectedNodes[node.id] !== undefined,
                  'master': selectedNodes[node.id] === 'master',
                  'worker': selectedNodes[node.id] === 'worker'
                }"
              >
                <div class="node-info">
                  <h5>{{ node.name }}</h5>
                  <div class="node-meta">
                    <span class="node-ip">{{ node.ip }}</span>
                    <span class="node-os">{{ node.os }}</span>
                    <span class="node-runtime">{{ node.containerRuntime }}</span>
                  </div>
                </div>
                <div class="node-selection-actions">
                  <div class="node-type-selector">
                    <button 
                      class="node-type-btn" 
                      :class="{ active: selectedNodes[node.id] === 'master' }"
                      @click="selectNodeType(node.id, 'master')"
                    >
                      主节点
                    </button>
                    <button 
                      class="node-type-btn" 
                      :class="{ active: selectedNodes[node.id] === 'worker' }"
                      @click="selectNodeType(node.id, 'worker')"
                    >
                      工作节点
                    </button>
                    <button 
                      class="node-type-btn" 
                      :class="{ active: selectedNodes[node.id] === undefined }"
                      @click="selectNodeType(node.id, undefined)"
                    >
                      取消
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
          
          <div class="selected-nodes-summary">
            <h4>已选择节点</h4>
            <div class="summary-info">
              <div class="summary-item">
                <span class="summary-label">主节点:</span>
                <span class="summary-value">{{ masterNodesCount }} 个</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">工作节点:</span>
                <span class="summary-value">{{ workerNodesCount }} 个</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">总节点数:</span>
                <span class="summary-value">{{ totalNodesCount }} 个</span>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- 步骤2: 部署配置 -->
      <div v-if="currentStep === 1" class="step-deploy-config">
        <h3>部署配置</h3>
        <div class="deploy-config-form">
          <div class="form-row">
            <div class="form-group">
              <label for="kube-version">Kubernetes版本: <span class="required">*</span></label>
              <select id="kube-version" v-model="deployConfig.kubeVersion" required>
                <option value="">-- 选择版本 --</option>
                <option v-for="version in availableVersions" :key="version" :value="version">{{ version }}</option>
              </select>
              <div class="version-tip">
                <small>提示: 版本列表根据源进行实时同步，确保选择的版本都是稳定可用的</small>
              </div>
            </div>
            <div class="form-group">
              <label for="pod-network">Pod网络插件:</label>
              <select id="pod-network" v-model="deployConfig.podNetwork" required>
                <option value="calico">Calico</option>
                <option value="flannel">Flannel</option>
                <option value="cilium">Cilium</option>
              </select>
            </div>
          </div>
          
          <div class="form-row">
            <div class="form-group">
              <label for="container-runtime">容器运行时:</label>
              <select id="container-runtime" v-model="deployConfig.containerRuntime" required>
                <option value="containerd">Containerd</option>
                <option value="cri-o">CRI-O</option>
              </select>
            </div>
            <div class="form-group">
              <label for="service-cidr">Service CIDR:</label>
              <input 
                type="text" 
                id="service-cidr" 
                v-model="deployConfig.serviceCIDR" 
                placeholder="10.96.0.0/12" 
                required
              >
            </div>
          </div>
          
          <div class="form-row">
            <div class="form-group">
              <label for="pod-cidr">Pod CIDR:</label>
              <input 
                type="text" 
                id="pod-cidr" 
                v-model="deployConfig.podCIDR" 
                placeholder="192.168.0.0/16" 
                required
              >
            </div>
            <div class="form-group">
              <label for="api-server-port">API Server端口:</label>
              <input 
                type="number" 
                id="api-server-port" 
                v-model="deployConfig.apiServerPort" 
                placeholder="6443" 
                required
              >
            </div>
          </div>
          
          <div class="form-row">
            <div class="form-group">
              <label class="checkbox-label">
                <input type="checkbox" v-model="deployConfig.enableHA">
                启用高可用(HA)
              </label>
            </div>
            <div class="form-group">
              <label class="checkbox-label">
                <input type="checkbox" v-model="deployConfig.enableMetrics">
                启用Metrics Server
              </label>
            </div>
          </div>
        </div>
        
        <!-- 高级部署配置 -->
        <div class="advanced-deploy-config">
          <h3 @click="toggleAdvancedDeployConfig" class="advanced-toggle">
            高级部署配置
            <span class="toggle-icon">{{ showAdvancedDeployConfig ? '▼' : '▶' }}</span>
          </h3>
          <div v-if="showAdvancedDeployConfig" class="skip-steps-config">
            <div class="skip-steps-description">
              默认所有步骤都会执行，勾选表示跳过该步骤
            </div>
            <div class="skip-steps-list">
              <div class="skip-step-item" v-for="step in deploySteps" :key="step.id">
                <label class="checkbox-label">
                  <input type="checkbox" v-model="skipSteps[step.id]">
                  跳过 {{ step.name }}
                </label>
                <div class="step-description">{{ step.description }}</div>
              </div>
            </div>
          </div>
        </div>
        
        <!-- 节点配置预览 -->
        <div class="node-configuration-summary">
          <h3>节点配置预览</h3>
          <div class="summary-grid">
            <div class="summary-section">
              <h5>主节点 ({{ masterNodes.length }}个)</h5>
              <div v-if="masterNodes.length > 0" class="preview-node-list">
                <div v-for="node in masterNodes" :key="node.id" class="preview-node">
                  {{ node.name }} ({{ node.ip }})
                </div>
              </div>
              <div v-else class="preview-empty">
                未选择主节点
              </div>
            </div>
            <div class="summary-section">
              <h5>工作节点 ({{ workerNodes.length }}个)</h5>
              <div v-if="workerNodes.length > 0" class="preview-node-list">
                <div v-for="node in workerNodes" :key="node.id" class="preview-node">
                  {{ node.name }} ({{ node.ip }})
                </div>
              </div>
              <div v-else class="preview-empty">
                未选择工作节点
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- 步骤3: 部署主节点 -->
      <div v-if="currentStep === 2" class="step-master-deployment">
        <h3>部署主节点</h3>
        <div class="deployment-progress-container">
          <!-- 部署控制按钮 -->
          <div class="deployment-controls" v-if="isDeploying">
            <button class="btn btn-danger" @click="stopDeployment">
              <span class="btn-icon">⏹️</span>
              停止部署
            </button>
          </div>
          <div class="deployment-controls" v-else>
            <button class="btn btn-primary" @click="deployMasterNodes">
              <span class="btn-icon">▶️</span>
              开始部署主节点
            </button>
          </div>
          
          <div class="master-node-list">
            <div 
              v-for="node in masterNodes" 
              :key="node.id" 
              class="deployment-node-item"
              :class="{ 'deployed': deploymentStatus.master[node.id] === 'completed', 'failed': deploymentStatus.master[node.id] === 'failed' }"
            >
              <div class="node-header">
                <span class="node-name">{{ node.name }} ({{ node.ip }})</span>
                <span class="deployment-status">{{ getDeploymentStatusText(deploymentStatus.master[node.id]) }}</span>
              </div>
              <div class="node-progress-bar">
                <div 
                  class="progress-bar" 
                  :style="{ width: `${deploymentProgress.master[node.id] || 0}%` }"
                  :class="deploymentStatus.master[node.id] === 'failed' ? 'failed' : ''"
                ></div>
              </div>
            </div>
          </div>
          
          <div class="deployment-logs">
            <h4>部署日志</h4>
            <div class="logs-container">
              <pre>{{ deployLogs }}</pre>
            </div>
          </div>
        </div>
      </div>
      
      <!-- 步骤4: 部署工作节点 -->
      <div v-if="currentStep === 3" class="step-worker-deployment modern">
        <h3>部署工作节点</h3>
        
        <!-- 主要控制区 -->
        <div class="main-control-panel">
          <!-- 部署控制按钮 -->
          <div class="deployment-controls" v-if="isDeploying">
            <button class="btn btn-danger" @click="stopDeployment">
              <span class="btn-icon">⏹️</span>
              停止部署
            </button>
          </div>
          <div class="deployment-controls" v-else>
            <button class="btn btn-primary" @click="deployWorkerNodes">
              <span class="btn-icon">▶️</span>
              开始部署工作节点
            </button>
          </div>
          
          <!-- 状态概览卡片 -->
          <div class="status-overview-card">
            <div class="status-overview-header">
              <h4>部署状态概览</h4>
              <div class="status-badge" :class="getOverallStatusClass()">
                {{ getOverallStatusText() }}
              </div>
            </div>
            <div class="status-stats">
              <div class="status-stat-item">
                <div class="stat-number">{{ Object.values(deploymentStatus.worker).filter(s => s === 'completed').length }}</div>
                <div class="stat-label">已完成</div>
                <div class="stat-icon success">✅</div>
              </div>
              <div class="status-stat-item">
                <div class="stat-number">{{ Object.values(deploymentStatus.worker).filter(s => s === 'deploying').length }}</div>
                <div class="stat-label">部署中</div>
                <div class="stat-icon warning">🔄</div>
              </div>
              <div class="status-stat-item">
                <div class="stat-number">{{ Object.values(deploymentStatus.worker).filter(s => s === 'failed').length }}</div>
                <div class="stat-label">失败</div>
                <div class="stat-icon danger">❌</div>
              </div>
              <div class="status-stat-item">
                <div class="stat-number">{{ workerNodes.length - Object.keys(deploymentStatus.worker).length }}</div>
                <div class="stat-label">待部署</div>
                <div class="stat-icon info">⏳</div>
              </div>
            </div>
          </div>
        </div>
        
        <!-- 核心内容区 -->
        <div class="core-content">
          <!-- 左侧：部署操作区 -->
          <div class="deploy-operation-section">
            <!-- Join Token 卡片 -->
            <div class="card join-token-card">
              <div class="card-header">
                <h4>集群加入凭证</h4>
                <span class="badge info">关键</span>
              </div>
              <div class="card-body">
                <div v-if="joinToken || manualJoinToken" class="join-token-content">
                  <div class="token-display">
                    <pre class="token-text">{{ joinToken || manualJoinToken }}</pre>
                    <button class="btn btn-primary copy-btn" @click="copyJoinToken">
                      <span class="btn-icon">📋</span>
                      复制
                    </button>
                  </div>
                  <div class="token-meta">
                    <span class="meta-item"><strong>有效期:</strong> 24小时</span>
                    <span class="meta-item"><strong>安全提示:</strong> 请勿泄露</span>
                  </div>
                </div>
                <div v-else class="token-loading">
                  <div class="loading-spinner"></div>
                  <p>正在获取加入凭证...</p>
                  <p class="hint">主节点初始化完成后自动生成</p>
                </div>
                
                <!-- 手动输入Join Token -->
                <div class="manual-token-input">
                  <h5 @click="toggleManualTokenInput" class="advanced-toggle">
                    手动输入Join Token
                    <span class="toggle-icon">{{ showManualTokenInput ? '▼' : '▶' }}</span>
                  </h5>
                  <div v-if="showManualTokenInput">
                    <textarea 
                      v-model="manualJoinToken" 
                      placeholder="请输入完整的join命令，例如：kubeadm join 192.168.31.206:6443 --token xxxxxx.xxxxxxxxxxxxxxxx --discovery-token-ca-cert-hash sha256:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
                      rows="3"
                    ></textarea>
                    <button class="btn btn-secondary" @click="useManualJoinToken">
                      <span class="btn-icon">🔧</span>
                      使用此Token
                    </button>
                    <p class="hint">如果自动提取失败，可以在此手动输入join命令</p>
                  </div>
                </div>
              </div>
            </div>
            
            <!-- 工作节点步骤选择卡片 -->
            <div class="card worker-steps-card">
              <div class="card-header">
                <h4 @click="toggleWorkerStepsConfig" class="advanced-toggle card-title-toggle">
                  工作节点部署步骤
                  <span class="toggle-icon">{{ showWorkerStepsConfig ? '▼' : '▶' }}</span>
                </h4>
                <span class="badge info">可选择</span>
              </div>
              <div class="card-body" v-if="showWorkerStepsConfig">
                <div class="steps-selection-description">
                  <p>选择要执行的工作节点部署步骤，默认执行所有步骤</p>
                </div>
                <div class="worker-steps-list">
                  <div 
                    v-for="step in workerDeploySteps" 
                    :key="step.id" 
                    class="worker-step-item"
                  >
                    <div class="step-selection">
                      <label class="checkbox-label">
                        <input 
                          type="checkbox" 
                          v-model="selectedWorkerSteps[step.id]"
                          :disabled="isDeploying"
                        >
                        {{ step.name }}
                      </label>
                    </div>
                    <div class="step-description">{{ step.description }}</div>
                  </div>
                </div>
              </div>
            </div>
            
            <!-- 部署操作卡片 -->
            <div class="card deploy-actions-card">
              <div class="card-header">
                <h4>部署操作</h4>
              </div>
              <div class="card-body">
                <div class="action-buttons">
                  <button class="btn btn-primary" @click="startWorkerDeployment" :disabled="isDeploying">
                    <span class="btn-icon">▶️</span>
                    开始部署
                  </button>
                  <button class="btn btn-secondary" @click="checkDeploymentStatus">
                    <span class="btn-icon">🔍</span>
                    检查状态
                  </button>
                  <button class="btn btn-secondary" @click="refreshJoinToken">
                    <span class="btn-icon">🔄</span>
                    刷新凭证
                  </button>
                </div>
                
                <!-- 部署模式切换 -->
                <div class="deploy-mode-toggle">
                  <h5>部署模式</h5>
                  <div class="toggle-group">
                    <button 
                      class="toggle-btn" 
                      :class="{ active: !showManualGuide }"
                      @click="showManualGuide = false"
                    >
                      自动化部署
                    </button>
                    <button 
                      class="toggle-btn" 
                      :class="{ active: showManualGuide }"
                      @click="showManualGuide = true"
                    >
                      手动部署
                    </button>
                  </div>
                </div>
                
                <!-- 部署指南 -->
                <div class="deploy-guide">
                  <h5>{{ showManualGuide ? '手动部署指南' : '自动化部署指南' }}</h5>
                  <ul class="guide-steps modern">
                    <li v-if="!showManualGuide">
                      <span class="step-number">1</span>
                      <div class="step-content">
                        <strong>检查前置条件</strong>
                        <p>确保工作节点已完成系统初始化</p>
                      </div>
                    </li>
                    <li v-if="!showManualGuide">
                      <span class="step-number">2</span>
                      <div class="step-content">
                        <strong>系统自动执行</strong>
                        <p>系统自动将工作节点加入集群</p>
                      </div>
                    </li>
                    <li>
                      <span class="step-number">{{ showManualGuide ? '1' : '3' }}</span>
                      <div class="step-content">
                        <strong>{{ showManualGuide ? '复制加入命令' : '监控部署状态' }}</strong>
                        <p>{{ showManualGuide ? '复制上方加入命令' : '查看下方节点列表监控进度' }}</p>
                      </div>
                    </li>
                    <li>
                      <span class="step-number">{{ showManualGuide ? '2' : '4' }}</span>
                      <div class="step-content">
                        <strong>{{ showManualGuide ? '登录工作节点' : '验证集群状态' }}</strong>
                        <p>{{ showManualGuide ? '使用SSH登录到工作节点' : '部署完成后验证集群状态' }}</p>
                        <code v-if="showManualGuide">ssh root@&lt;工作节点IP&gt;</code>
                      </div>
                    </li>
                    <li v-if="showManualGuide">
                      <span class="step-number">3</span>
                      <div class="step-content">
                        <strong>执行加入命令</strong>
                        <p>在工作节点上粘贴并执行加入命令</p>
                      </div>
                    </li>
                    <li v-if="showManualGuide">
                      <span class="step-number">4</span>
                      <div class="step-content">
                        <strong>验证部署结果</strong>
                        <p>主节点执行 <code>kubectl get nodes</code> 验证</p>
                      </div>
                    </li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
          
          <!-- 右侧：节点状态区 -->
          <div class="nodes-status-section">
            <!-- 节点列表卡片 -->
            <div class="card nodes-list-card">
              <div class="card-header">
                <h4>工作节点列表</h4>
                <span class="badge primary">{{ workerNodes.length }} 个节点</span>
              </div>
              <div class="card-body">
                <div class="nodes-grid">
                  <div 
                    v-for="node in workerNodes" 
                    :key="node.id" 
                    class="node-card"
                    :class="{
                      'status-completed': deploymentStatus.worker[node.id] === 'completed',
                      'status-failed': deploymentStatus.worker[node.id] === 'failed',
                      'status-deploying': deploymentStatus.worker[node.id] === 'deploying'
                    }"
                  >
                    <div class="node-header">
                      <div class="node-name">{{ node.name }}</div>
                      <div class="node-status" :class="deploymentStatus.worker[node.id]">
                        {{ getDeploymentStatusText(deploymentStatus.worker[node.id]) }}
                      </div>
                    </div>
                    <div class="node-info">
                      <div class="node-ip">{{ node.ip }}</div>
                      <div class="node-runtime">{{ node.containerRuntime }}</div>
                    </div>
                    <div class="node-progress">
                      <div class="progress-bar-container">
                        <div 
                          class="progress-bar" 
                          :style="{ width: `${deploymentProgress.worker[node.id] || 0}%` }"
                          :class="deploymentStatus.worker[node.id] === 'failed' ? 'failed' : ''"
                        ></div>
                        <span class="progress-text">{{ deploymentProgress.worker[node.id] || 0 }}%</span>
                      </div>
                    </div>
                    <div class="node-actions" v-if="deploymentStatus.worker[node.id] === 'failed'">
                      <button class="btn btn-sm btn-primary" @click="retryNodeDeployment(node.id)">
                        <span class="btn-icon">🔄</span>
                        重试
                      </button>
                    </div>
                  </div>
                </div>
                <div v-if="workerNodes.length === 0" class="empty-state">
                  <div class="empty-icon">📦</div>
                  <p>暂无工作节点</p>
                  <p class="hint">请先在节点管理中添加工作节点</p>
                </div>
              </div>
            </div>
            
            <!-- 日志卡片 -->
            <div class="card logs-card">
              <div class="card-header">
                <h4>实时部署日志</h4>
                <div class="logs-actions">
                  <button class="btn btn-sm btn-secondary" @click="clearLogs" title="清空日志">
                    <span class="btn-icon">🗑️</span>
                    清空
                  </button>
                  <button class="btn btn-sm btn-secondary" @click="toggleAutoScroll" :title="autoScrollLogs ? '暂停滚动' : '自动滚动'">
                    <span class="btn-icon">{{ autoScrollLogs ? '⏸️' : '▶️' }}</span>
                    {{ autoScrollLogs ? '暂停' : '自动' }}
                  </button>
                </div>
              </div>
              <div class="card-body">
                <div class="logs-container" ref="logsContainer">
                  <pre class="logs-content">{{ deployLogs }}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- 步骤5: 部署完成 -->
      <div v-if="currentStep === 4" class="step-completion">
        <h3>部署完成</h3>
        <div class="completion-summary">
          <div class="summary-card success">
            <h4>部署结果</h4>
            <div class="summary-stats">
              <div class="stat-item">
                <span class="stat-label">主节点:</span>
                <span class="stat-value">{{ masterNodes.length }} / {{ masterNodes.length }} 成功</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">工作节点:</span>
                <span class="stat-value">{{ workerNodes.length }} / {{ workerNodes.length }} 成功</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">集群状态:</span>
                <span class="stat-value success">正常运行</span>
              </div>
            </div>
          </div>
          
          <div class="summary-card info">
            <h4>集群信息</h4>
            <div class="cluster-info">
              <div class="info-item">
                <span class="info-label">Kubernetes版本:</span>
                <span class="info-value">{{ deployConfig.kubeVersion }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">API Server地址:</span>
                <span class="info-value">{{ clusterInfo.apiServerAddress }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">Pod网络插件:</span>
                <span class="info-value">{{ deployConfig.podNetwork }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">容器运行时:</span>
                <span class="info-value">{{ deployConfig.containerRuntime }}</span>
              </div>
            </div>
          </div>
          
          <div class="summary-card warning">
            <h4>后续操作建议</h4>
            <ul class="next-steps">
              <li>安装Helm包管理器</li>
              <li>部署Ingress Controller</li>
              <li>配置监控系统(Prometheus + Grafana)</li>
              <li>设置日志收集系统(ELK或Loki)</li>
              <li>定期备份etcd数据</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
    </section>
    
    <!-- 步骤导航按钮 -->
    <div class="step-navigation">
      <button 
        class="btn btn-secondary" 
        @click="goToPreviousStep" 
        :disabled="currentStep === 0 || isDeploying"
      >
        上一步
      </button>
      <button 
        class="btn btn-danger" 
        @click="resetDeployment" 
        :disabled="isDeploying"
        title="重置部署步骤，清除所有选择和配置"
      >
        重置部署
      </button>
      <button 
        v-if="currentStep < (steps || []).length - 1" 
        class="btn btn-primary" 
        @click="goToNextStep" 
        :disabled="!canProceedToNextStep() || isDeploying"
      >
        <span v-if="isDeploying" class="loading-spinner"></span>
        {{ isDeploying ? '部署中...' : '下一步' }}
      </button>
      <button 
        v-else 
        class="btn btn-success" 
        @click="finishDeployment"
      >
        完成部署
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch, onUnmounted } from 'vue'
import axios from 'axios'

// 定义组件的属性和事件
const props = defineProps({
  availableVersions: {
    type: Array,
    default: () => []
  },
  nodes: {
    type: Array,
    default: () => []
  },
  kubeadmVersion: {
    type: String,
    default: ''
  },
  systemOnline: {
    type: Boolean,
    default: true
  },
  apiStatus: {
    type: String,
    default: 'online'
  }
})

const emit = defineEmits(['showMessage', 'setKubeadmVersion'])

// API配置 - 动态获取当前页面的主机和端口，避免硬编码
const getApiBaseUrl = () => {
  // 获取当前页面的URL
  const currentUrl = window.location.origin;
  // 前端开发环境可能使用不同端口，需要根据实际情况调整
  // 将任何端口替换为后端端口8080
  return currentUrl.replace(/:\d+$/, ':8080');
};

const apiClient = axios.create({
  baseURL: getApiBaseUrl(),
  timeout: 1800000, // 30分钟超时，适应Kubernetes组件安装的耗时过程
  headers: {
    'Content-Type': 'application/json'
  }
})

// SSE配置
const eventSource = ref(null)
const sseConnected = ref(false)
const sseReconnectTimer = ref(null)
const reconnectAttempts = ref(0)
const maxReconnectAttempts = ref(10) // 增加最大重试次数
const reconnectInterval = ref(3000) // 初始重试间隔3秒

// 初始化SSE连接
const initSSE = () => {
  // 如果已经有连接且状态为OPEN，直接返回
  if (eventSource.value && eventSource.value.readyState === EventSource.OPEN) {
    console.log('SSE连接已存在，跳过创建')
    sseConnected.value = true
    return
  }
  
  // 关闭现有连接
  if (eventSource.value) {
    try {
      eventSource.value.close()
      console.log('已关闭现有SSE连接')
    } catch (error) {
      console.error('关闭现有SSE连接失败:', error)
    }
    eventSource.value = null
  }
  
  deployLogs.value += `[${new Date().toLocaleString()}] 正在连接实时日志流...\n`
  
  try {
    // 动态构建SSE URL，确保与API使用相同的主机和端口
    const apiBaseUrl = getApiBaseUrl()
    const sseUrl = `${apiBaseUrl}/logs/stream`
    
    console.log('创建SSE连接:', sseUrl)
    eventSource.value = new EventSource(sseUrl, { withCredentials: false })
    
    // 连接打开时的处理
    eventSource.value.onopen = () => {
      console.log('SSE连接已建立')
      sseConnected.value = true
      reconnectAttempts.value = 0
      reconnectInterval.value = 3000 // 重置重试间隔
      deployLogs.value += `[${new Date().toLocaleString()}] 实时日志流已连接\n\n`
    }
    
    // 接收消息时的处理
    eventSource.value.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)
        handleSSEMessage(message)
      } catch (error) {
        console.error('解析SSE消息失败:', error)
        deployLogs.value += `[${new Date().toLocaleString()}] 解析实时日志失败: ${error.message}\n`
        deployLogs.value += `原始消息: ${event.data}\n\n`
      }
    }
    
    // 连接关闭时的处理
    eventSource.value.onclose = () => {
      console.log('SSE连接已关闭')
      sseConnected.value = false
      deployLogs.value += `[${new Date().toLocaleString()}] 实时日志流已关闭\n`
      // 尝试重新连接
      reconnectSSE()
    }
    
    // 连接错误时的处理
    eventSource.value.onerror = (error) => {
      console.error('SSE连接错误:', error)
      sseConnected.value = false
      // 立即重连，不等待onclose事件
      if (eventSource.value && eventSource.value.readyState === EventSource.CLOSED) {
        reconnectSSE()
      }
    }
  } catch (error) {
    console.error('创建SSE连接失败:', error)
    deployLogs.value += `[${new Date().toLocaleString()}] 创建实时日志流连接失败: ${error.message}\n\n`
    // 尝试重新连接
    reconnectSSE()
  }
}

// 处理SSE消息
const handleSSEMessage = (message) => {
  // 后端发送的是log.LogEntry类型，直接使用日志信息
  if (message) {
    // 处理各种类型的日志消息
    if (message.Operation === 'DeployK8sCluster' || message.Operation === 'InitMaster' || 
        message.Operation === 'JoinWorker' || message.Operation === 'InstallKubernetesComponents' ||
        message.Operation === 'SSHCommandExecution') {
      // 标准日志消息
      const timestamp = message.CreatedAt ? new Date(message.CreatedAt).toLocaleString() : new Date().toLocaleString()
      deployLogs.value += `[${timestamp}] [${message.NodeName || message.NodeID}] ${message.Operation}: ${message.Command || ''}\n`
      deployLogs.value += `${message.Output || ''}\n`
      deployLogs.value += `状态: ${message.Status === 'success' ? '成功' : message.Status === 'failed' ? '失败' : '运行中'}\n\n`
      
      // 更新部署状态
      const nodeType = message.NodeID in selectedNodes.value ? selectedNodes.value[message.NodeID] : 
                      message.Operation.includes('Master') ? 'master' : 'worker'
      
      const statusMap = {
        'success': 'completed',
        'failed': 'failed',
        'running': 'deploying'
      }
      
      if (nodeType === 'master') {
        deploymentStatus.value.master[message.NodeID] = statusMap[message.Status] || 'deploying'
        if (message.Status === 'running') {
          deploymentProgress.value.master[message.NodeID] = Math.min((deploymentProgress.value.master[message.NodeID] || 0) + 15, 90)
        } else if (message.Status === 'success') {
          deploymentProgress.value.master[message.NodeID] = 100
        }
      } else if (nodeType === 'worker') {
        deploymentStatus.value.worker[message.NodeID] = statusMap[message.Status] || 'deploying'
        if (message.Status === 'running') {
          deploymentProgress.value.worker[message.NodeID] = Math.min((deploymentProgress.value.worker[message.NodeID] || 0) + 15, 90)
        } else if (message.Status === 'success') {
          deploymentProgress.value.worker[message.NodeID] = 100
        }
      }
      
      // 如果是JoinWorker操作成功，自动更新状态
      if (message.Operation === 'JoinWorker' && message.Status === 'success') {
        deploymentStatus.value.worker[message.NodeID] = 'completed'
        deploymentProgress.value.worker[message.NodeID] = 100
        
        // 检查所有工作节点是否都已部署完成
        const allWorkersCompleted = Object.values(deploymentStatus.value.worker).every(status => status === 'completed')
        const hasWorkers = Object.keys(deploymentStatus.value.worker).length > 0
        
        if (allWorkersCompleted && hasWorkers && currentStep.value === 3) {
          // 所有工作节点部署完成，进入完成步骤
          currentStep.value = 4
          isDeploying.value = false
        }
      }
      
      // 检查所有类型的消息中是否包含join token，无论Operation和Status
      if (message.Output) {
        // 使用更宽松的正则表达式提取join命令，匹配包含换行符的格式
        // 匹配"kubeadm join"开头，包含"--token"和"--discovery-token-ca-cert-hash"的完整命令
        const joinTokenMatch = message.Output.match(/kubeadm join[\s\S]*?--token[\s\S]*?--discovery-token-ca-cert-hash[\s\S]*?(?=\n\n|\n$|$)/)
        if (joinTokenMatch) {
          const joinCommand = joinTokenMatch[0]
          deployLogs.value += `[${new Date().toLocaleString()}] 已提取join命令: ${joinCommand}\n\n`
          
          // 持久化存储join命令，方便后续加入其他节点使用
          // 1. 保存到localStorage，持久化存储
          localStorage.setItem('kubeadmJoinCommand', joinCommand)
          // 2. 保存到sessionStorage，当前会话可用
          sessionStorage.setItem('kubeadmJoinCommand', joinCommand)
          // 3. 保存到ref中，用于UI显示
          joinToken.value = joinCommand
          
          // 保存token有效期信息，默认24小时
          const tokenExpiry = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString()
          localStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
          sessionStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
          
          // 只要提取到join命令，就认为主节点初始化完成，更新部署状态
          isDeploying.value = false
          steps.value[2].status = 'completed'
          
          // 检测到token，自动进入下一步
          const workerNodeIds = Object.keys(selectedNodes.value).filter(nodeId => selectedNodes.value[nodeId] === 'worker')
          if (workerNodeIds.length > 0 && currentStep.value === 2) {
            deployLogs.value += `[${new Date().toLocaleString()}] 检测到主节点已生成token，自动进入工作节点部署步骤\n\n`
            // 自动进入工作节点部署步骤
            currentStep.value = 3
            // 自动开始部署工作节点
            deployLogs.value += `[${new Date().toLocaleString()}] 自动开始部署工作节点...\n\n`
            deployWorkerNodes()
          } else if (workerNodeIds.length === 0 && currentStep.value === 2) {
            deployLogs.value += `[${new Date().toLocaleString()}] 检测到主节点已生成token，没有工作节点需要部署，直接进入完成步骤\n\n`
            // 如果没有工作节点，直接进入完成步骤
            currentStep.value = 4
          }
        } else {
          // 添加调试信息，帮助排查问题
          deployLogs.value += `[${new Date().toLocaleString()}] 尝试提取join命令，但未匹配到完整格式\n`
          // 记录输出的前500个字符，帮助调试
          deployLogs.value += `输出片段: ${message.Output.substring(0, 500)}...\n\n`
          // 尝试使用更简单的正则表达式提取
          const simpleJoinTokenMatch = message.Output.match(/kubeadm join.*?--token.*?\n/)
          if (simpleJoinTokenMatch) {
            deployLogs.value += `[${new Date().toLocaleString()}] 尝试使用简单正则表达式提取到join命令: ${simpleJoinTokenMatch[0]}\n\n`
          }
        }
      }
      
      // 处理部署完成的情况
      if ((message.Operation === 'DeployK8sCluster' || message.Operation === 'InitMaster') && 
          message.Status === 'success') {
        // 部署成功，更新状态
        isDeploying.value = false
        
        // 检查是否有工作节点需要部署
        const hasWorkerNodes = Object.keys(selectedNodes.value).some(nodeId => selectedNodes.value[nodeId] === 'worker')
        
        // 如果部署的是主节点，且没有工作节点，直接进入完成步骤
        if ((message.Operation === 'InitMaster' || message.Operation === 'DeployK8sCluster') && !hasWorkerNodes) {
          currentStep.value = 4
          steps.value[2].status = 'completed'
          steps.value[3].status = 'completed'
        } else if (message.Operation === 'DeployK8sCluster') {
          // 如果是完整集群部署，直接进入完成步骤
          currentStep.value = 4
          steps.value[3].status = 'completed'
        }
      } else if ((message.Operation === 'DeployK8sCluster' || message.Operation === 'InitMaster') && 
                 message.Status === 'failed') {
        // 如果部署失败，且还没有提取到join命令，才更新状态为失败
        if (!joinToken.value) {
          // 无论部署结果如何，都先将isDeploying设置为false
          isDeploying.value = false
          steps.value[2].status = 'failed'
        }
      }
      
      // 检查日志内容中是否包含部署完成的关键字
      if (message.Output && (message.Output.includes('=== Kubernetes集群部署完成 ===') || 
          message.Output.includes('Worker节点加入集群成功') || 
          message.Output.includes('Kubernetes集群部署完成'))) {
        // 部署完成，更新UI状态
        isDeploying.value = false
        
        // 检查当前步骤，如果是在工作节点部署步骤，自动进入完成步骤
        if (currentStep.value === 3) {
          currentStep.value = 4
          steps.value[3].status = 'completed'
        }
      }
    } else {
      // 未知消息格式，记录原始内容
      deployLogs.value += `[${new Date().toLocaleString()}] 收到未知格式日志: ${JSON.stringify(message)}\n\n`
    }
  }
}

// 尝试重新连接SSE
const reconnectSSE = () => {
  if (reconnectAttempts.value < maxReconnectAttempts.value) {
    reconnectAttempts.value++
    const delay = reconnectInterval.value * Math.pow(1.5, reconnectAttempts.value - 1) // 指数退避，最多1分钟
    console.log(`尝试重新连接SSE (${reconnectAttempts.value}/${maxReconnectAttempts.value})... 延迟 ${delay}ms`)
    deployLogs.value += `[${new Date().toLocaleString()}] 尝试重新连接实时日志流 (${reconnectAttempts.value}/${maxReconnectAttempts.value})...\n`
    
    sseReconnectTimer.value = setTimeout(() => {
      initSSE()
    }, delay)
  } else {
    deployLogs.value += `[${new Date().toLocaleString()}] 实时日志流重连失败，已达到最大重试次数\n`
    deployLogs.value += '请检查网络连接和后端服务状态\n\n'
  }
}

// 关闭SSE连接
const closeSSE = () => {
  if (eventSource.value) {
    try {
      eventSource.value.close()
    } catch (error) {
      console.error('关闭SSE连接失败:', error)
    }
    eventSource.value = null
  }
  if (sseReconnectTimer.value) {
    clearTimeout(sseReconnectTimer.value)
    sseReconnectTimer.value = null
  }
  sseConnected.value = false
}

// 手动重新连接SSE
const manualReconnectSSE = () => {
  reconnectAttempts.value = 0
  reconnectInterval.value = 3000
  deployLogs.value += `[${new Date().toLocaleString()}] 手动重新连接实时日志流...\n`
  initSSE()
}

// 部署步骤
const steps = ref([
  { title: '选择节点', status: '' },
  { title: '部署配置', status: '' },
  { title: '部署主节点', status: '' },
  { title: '部署工作节点', status: '' },
  { title: '部署完成', status: '' }
])

// 当前步骤
const currentStep = ref(0)

// 节点相关状态
const selectedNodes = ref({})
const selectedRuntimeFilter = ref('')
const selectedStatusFilter = ref('')

// 部署配置
const deployConfig = ref({
  kubeVersion: '',
  podNetwork: 'calico',
  containerRuntime: 'containerd',
  serviceCIDR: '10.96.0.0/12',
  podCIDR: '192.168.0.0/16',
  apiServerPort: 6443,
  enableHA: false,
  enableMetrics: true,
  distro: 'ubuntu' // 默认发行版，可根据实际情况调整
})

// 部署步骤配置
const deploySteps = ref([
  { id: 'system_preparation', name: '系统准备', description: '执行系统准备脚本，包括关闭防火墙、禁用SELinux等' },
  { id: 'ip_forward_configuration', name: 'IP转发配置', description: '配置IP转发和内核参数' },
  { id: 'container_runtime_installation', name: '容器运行时安装', description: '安装和配置容器运行时(containerd/cri-o)' },
  { id: 'kubernetes_repository_configuration', name: 'Kubernetes仓库配置', description: '添加Kubernetes仓库' },
  { id: 'kubernetes_components_installation', name: 'Kubernetes组件安装', description: '安装kubelet、kubeadm和kubectl' },
  { id: 'master_initialization', name: 'Master节点初始化', description: '初始化Kubernetes Master节点' },
  { id: 'worker_join', name: 'Worker节点加入', description: '将Worker节点加入集群' },
  { id: 'cluster_verification', name: '集群验证', description: '验证集群状态' }
])

// 步骤跳过配置 - 默认所有步骤都不跳过（复选框未勾选状态）
const skipSteps = ref({})

// 部署状态
const isDeploying = ref(false)
const deployLogs = ref('Kubernetes集群部署日志\n=====================\n')
const deploymentStatus = ref({
  master: {},
  worker: {}
})
const deploymentProgress = ref({
  master: {},
  worker: {}
})
const deploymentTimestamps = ref({
  master: {},
  worker: {}
})

// 高级部署配置显示控制
const showAdvancedDeployConfig = ref(false)
const toggleAdvancedDeployConfig = () => {
  showAdvancedDeployConfig.value = !showAdvancedDeployConfig.value
}

// 工作节点部署步骤配置显示控制
const showWorkerStepsConfig = ref(false)
const toggleWorkerStepsConfig = () => {
  showWorkerStepsConfig.value = !showWorkerStepsConfig.value
}

// 工作节点部署步骤跟踪
const workerDeploymentStep = ref(0)

// 工作节点部署步骤配置
const workerDeploySteps = ref([
  { id: 'worker_system_preparation', name: '工作节点系统准备', description: '执行工作节点系统准备，包括关闭防火墙、禁用SELinux等' },
  { id: 'worker_ip_forward_configuration', name: '工作节点IP转发配置', description: '配置工作节点IP转发和内核参数' },
  { id: 'worker_container_runtime_installation', name: '工作节点容器运行时安装', description: '安装和配置工作节点容器运行时' },
  { id: 'worker_kubernetes_components_installation', name: '工作节点Kubernetes组件安装', description: '安装工作节点kubelet和kubeadm' },
  { id: 'worker_join', name: '工作节点加入集群', description: '执行kubeadm join命令将工作节点加入集群' },
  { id: 'worker_verification', name: '工作节点验证', description: '验证工作节点是否成功加入集群' }
])

// 选中的工作节点部署步骤 - 默认全选
const selectedWorkerSteps = ref({
  worker_system_preparation: true,
  worker_ip_forward_configuration: true,
  worker_container_runtime_installation: true,
  worker_kubernetes_components_installation: true,
  worker_join: true,
  worker_verification: true
})

// 部署指南控制
const showManualGuide = ref(false)

// 日志相关状态
const autoScrollLogs = ref(true)
const logsContainer = ref(null)

// 保存提取的join token
const joinToken = ref('')
// 手动输入的join token
const manualJoinToken = ref('')
// 手动输入Join Token显示控制
const showManualTokenInput = ref(false)
const toggleManualTokenInput = () => {
  showManualTokenInput.value = !showManualTokenInput.value
}

// 集群信息
const clusterInfo = ref({
  apiServerAddress: '',
  clusterName: '',
  clusterId: ''
})

// 部署取消令牌
const abortController = ref(null)

// 自动同步节点类型到selectedNodes
const syncNodeTypes = () => {
  const updatedNodes = { ...selectedNodes.value }
  
  // 遍历所有节点，自动设置节点类型
  props.nodes.forEach(node => {
    if (node.nodeType && (node.nodeType === 'master' || node.nodeType === 'worker')) {
      // 只在节点未被手动选择时自动设置类型
      if (!(node.id in updatedNodes)) {
        updatedNodes[node.id] = node.nodeType
        deployLogs.value += `[${new Date().toLocaleString()}] 自动选择节点: ${node.name} (${node.ip}) 作为 ${node.nodeType}\n`
      }
    }
  })
  
  selectedNodes.value = updatedNodes
}

// 监听节点列表变化，自动同步节点类型
watch(() => props.nodes, () => {
  syncNodeTypes()
}, { deep: true, immediate: true })

// 保存页面状态到localStorage
const saveState = () => {
  const state = {
    currentStep: currentStep.value,
    selectedNodes: selectedNodes.value,
    deployConfig: deployConfig.value,
    skipSteps: skipSteps.value,
    steps: steps.value
    // joinToken不再保存到状态对象中，而是单独持久化存储
  }
  localStorage.setItem('kubeadmManagerState', JSON.stringify(state))
}

// 从localStorage恢复页面状态
const loadState = () => {
  const savedState = localStorage.getItem('kubeadmManagerState')
  if (savedState) {
    try {
      const state = JSON.parse(savedState)
      currentStep.value = state.currentStep || 0
      selectedNodes.value = state.selectedNodes || {}
      deployConfig.value = state.deployConfig || {
        kubeVersion: '',
        podNetwork: 'calico',
        containerRuntime: 'containerd',
        serviceCIDR: '10.96.0.0/12',
        podCIDR: '192.168.0.0/16',
        apiServerPort: 6443,
        enableHA: false,
        enableMetrics: true,
        distro: 'ubuntu' // 默认发行版，可根据实际情况调整
      }
      skipSteps.value = state.skipSteps || {}
      if (state.steps) {
        steps.value = state.steps
      }
    } catch (error) {
      console.error('恢复状态失败:', error)
    }
  }
  
  // 优先从localStorage恢复join token，支持后续加入其他节点
  const savedJoinToken = localStorage.getItem('kubeadmJoinCommand')
  if (savedJoinToken) {
    joinToken.value = savedJoinToken
    deployLogs.value += `[${new Date().toLocaleString()}] 已从持久化存储中恢复join命令: ${savedJoinToken}\n\n`
    
    // 检查token是否过期
    const tokenExpiry = localStorage.getItem('kubeadmJoinTokenExpiry')
    if (tokenExpiry) {
      const expiryDate = new Date(tokenExpiry)
      const now = new Date()
      if (now > expiryDate) {
        deployLogs.value += `[${new Date().toLocaleString()}] 注意：保存的join命令已过期 (${expiryDate.toLocaleString()})\n\n`
      } else {
        const timeLeft = Math.floor((expiryDate - now) / (1000 * 60)) // 剩余分钟数
        deployLogs.value += `[${new Date().toLocaleString()}] join命令将在 ${timeLeft} 分钟后过期\n\n`
      }
    }
  } else {
    deployLogs.value += `[${new Date().toLocaleString()}] 没有找到保存的join命令\n\n`
  }
}

// 监听状态变化，保存到localStorage
watch([currentStep, selectedNodes, deployConfig, skipSteps, steps, joinToken], () => {
  saveState()
}, { deep: true })

// 组件挂载时的调试信息
onMounted(() => {
  try {
    console.log('KubeadmManager组件已挂载')
    // 加载保存的状态
    loadState()
    // 初始化SSE连接
    initSSE()
  } catch (error) {
    console.error('KubeadmManager组件挂载失败:', error)
    deployLogs.value += `[${new Date().toLocaleString()}] 组件初始化失败: ${error.message}\n\n`
    emit('showMessage', `组件初始化失败: ${error.message}`, 'error')
  }
})

// 组件卸载时关闭SSE连接
onUnmounted(() => {
  console.log('KubeadmManager组件已卸载')
  closeSSE()
  // 保存状态
  saveState()
})

// 计算属性：过滤后的节点
const filteredNodes = computed(() => {
  return props.nodes.filter(node => {
    const matchesRuntime = !selectedRuntimeFilter.value || node.containerRuntime === selectedRuntimeFilter.value
    const matchesStatus = !selectedStatusFilter.value || node.status === selectedStatusFilter.value
    return matchesRuntime && matchesStatus
  })
})

// 计算属性：主节点数量
const masterNodesCount = computed(() => {
  return Object.values(selectedNodes.value).filter(type => type === 'master').length
})

// 计算属性：工作节点数量
const workerNodesCount = computed(() => {
  return Object.values(selectedNodes.value).filter(type => type === 'worker').length
})

// 计算属性：总节点数量
const totalNodesCount = computed(() => {
  return Object.keys(selectedNodes.value).length
})

// 计算属性：主节点列表
const masterNodes = computed(() => {
  return (props.nodes || []).filter(node => selectedNodes.value[node.id] === 'master')
})

// 计算属性：工作节点列表
const workerNodes = computed(() => {
  return (props.nodes || []).filter(node => selectedNodes.value[node.id] === 'worker')
})

// 选择节点类型
const selectNodeType = (nodeId, type) => {
  const node = (props.nodes || []).find(n => n.id === nodeId)
  if (node) {
    if (type === undefined) {
      // 取消选择
      deployLogs.value += `[${new Date().toLocaleString()}] 取消选择节点: ${node.name} (${node.ip})\n`
      delete selectedNodes.value[nodeId]
    } else {
      // 选择节点类型
      const oldType = selectedNodes.value[nodeId]
      if (oldType) {
        deployLogs.value += `[${new Date().toLocaleString()}] 将节点 ${node.name} (${node.ip}) 从 ${oldType} 改为 ${type}\n`
      } else {
        deployLogs.value += `[${new Date().toLocaleString()}] 选择节点 ${node.name} (${node.ip}) 作为 ${type}\n`
      }
      selectedNodes.value[nodeId] = type
    }
  }
}

// 判断是否可以进入下一步
const canProceedToNextStep = () => {
  switch (currentStep.value) {
    case 0: // 选择节点
      // 允许只选择工作节点，只要有join token（持久化存储或手动输入）
      const hasJoinToken = joinToken.value || localStorage.getItem('kubeadmJoinCommand')
      return (masterNodesCount.value > 0 || (workerNodesCount.value > 0 && hasJoinToken)) && totalNodesCount.value > 0
    case 1: // 部署配置
      return deployConfig.value.kubeVersion && deployConfig.value.podNetwork && deployConfig.value.containerRuntime
    case 2: // 部署主节点
      // 如果没有主节点，或者有join token，可以跳过主节点部署
      if (masterNodes.value.length === 0) {
        return true
      }
      // 允许手动推进：如果主节点部署请求已发送（isDeploying为false），则允许用户手动点击下一步
      // 这样即使用户没有收到SSE消息，也可以继续部署流程
      return Object.values(deploymentStatus.value.master).every(status => status === 'completed') || !isDeploying.value
    case 3: // 部署工作节点
      return Object.values(deploymentStatus.value.worker).every(status => status === 'completed') || !isDeploying.value
    default:
      return true
  }
}

// 获取整体部署状态文本
const getOverallStatusText = () => {
  const completed = Object.values(deploymentStatus.value.worker).filter(s => s === 'completed').length
  const deploying = Object.values(deploymentStatus.value.worker).filter(s => s === 'deploying').length
  const failed = Object.values(deploymentStatus.value.worker).filter(s => s === 'failed').length
  
  if (completed === workerNodes.value.length && workerNodes.value.length > 0) {
    return '部署完成'
  } else if (failed > 0) {
    return '部署失败'
  } else if (deploying > 0) {
    return '部署中'
  } else {
    return '待部署'
  }
}

// 获取整体部署状态样式类
const getOverallStatusClass = () => {
  const completed = Object.values(deploymentStatus.value.worker).filter(s => s === 'completed').length
  const deploying = Object.values(deploymentStatus.value.worker).filter(s => s === 'deploying').length
  const failed = Object.values(deploymentStatus.value.worker).filter(s => s === 'failed').length
  
  if (completed === workerNodes.value.length && workerNodes.value.length > 0) {
    return 'success'
  } else if (failed > 0) {
    return 'danger'
  } else if (deploying > 0) {
    return 'warning'
  } else {
    return 'info'
  }
}

// 检查节点容器运行时状态
const checkContainerRuntime = () => {
  const selectedNodeIds = Object.keys(selectedNodes.value)
  const nodesWithoutRuntime = selectedNodeIds.filter(nodeId => {
    const node = (props.nodes || []).find(n => n.id === nodeId)
    return !node.containerRuntime || node.containerRuntime === ''
  })
  
  return {
    hasNodesWithoutRuntime: nodesWithoutRuntime.length > 0,
    nodesWithoutRuntime: nodesWithoutRuntime
  }
}



// 进入下一步
const goToNextStep = async (skipRuntimeCheck = false) => {
  deployLogs.value += `[${new Date().toLocaleString()}] 准备进入下一步：从步骤 ${currentStep.value + 1} 到步骤 ${currentStep.value + 2}\n`
  
  // 如果是从步骤1进入步骤2，检查节点容器运行时
  if (currentStep.value === 0 && currentStep.value + 1 === 1 && !skipRuntimeCheck) {
    deployLogs.value += `[${new Date().toLocaleString()}] 检查节点容器运行时...\n`
    const { hasNodesWithoutRuntime, nodesWithoutRuntime } = checkContainerRuntime()
    
    if (hasNodesWithoutRuntime) {
        const nodeNames = nodesWithoutRuntime.map(nodeId => {
          const node = (props.nodes || []).find(n => n.id === nodeId)
          return node ? node.name : nodeId
        })
      deployLogs.value += `[${new Date().toLocaleString()}] 发现 ${nodeNames.length} 个节点没有安装容器运行时：${nodeNames.join(', ')}\n`
      
      if (confirm(`以下节点没有安装容器运行时: ${nodeNames.join(', ')}\n是否自动安装${deployConfig.value.containerRuntime}?`)) {
        await installContainerRuntime()
        return
      } else {
        // 用户取消安装，不允许继续
        deployLogs.value += `[${new Date().toLocaleString()}] 用户取消安装容器运行时，部署流程终止\n`
        emit('showMessage', { text: '请先为所有节点安装容器运行时', type: 'warning' })
        return
      }
    } else {
      deployLogs.value += `[${new Date().toLocaleString()}] 所有节点已安装容器运行时，继续部署...\n`
    }
  }
  
  if (!canProceedToNextStep()) {
    deployLogs.value += `[${new Date().toLocaleString()}] 无法进入下一步，检查是否满足条件\n`
    return
  }
  
  // 保存当前步骤状态
  steps.value[currentStep.value].status = 'completed'
  
  // 检查是否需要跳过主节点部署步骤
  const hasMasterNodes = masterNodes.value.length > 0
  const hasJoinToken = joinToken.value || localStorage.getItem('kubeadmJoinCommand')
  
  if (currentStep.value === 1 && !hasMasterNodes && hasJoinToken) {
    // 没有主节点，但有join token，直接跳过步骤2（部署主节点），进入步骤3（部署工作节点）
    deployLogs.value += `[${new Date().toLocaleString()}] 没有选择主节点，但检测到有join token，直接跳过主节点部署步骤\n`
    steps.value[2].status = 'completed' // 标记主节点部署步骤为已完成
    currentStep.value = 3 // 直接进入工作节点部署步骤
  } else {
    // 正常进入下一步
    currentStep.value++
  }
  
  deployLogs.value += `[${new Date().toLocaleString()}] 进入步骤 ${currentStep.value + 1}: ${steps.value[currentStep.value].title}\n`
  
  // 移除自动部署逻辑，改为手动点击开始部署按钮
}

// 回到上一步
const goToPreviousStep = () => {
  if (currentStep.value > 0) {
    deployLogs.value += `[${new Date().toLocaleString()}] 回到上一步：从步骤 ${currentStep.value + 1} 到步骤 ${currentStep.value}\n`
    currentStep.value--
  }
}

// 部署主节点
const deployMasterNodes = async () => {
  // 如果没有主节点，直接返回
  if (masterNodes.value.length === 0) {
    deployLogs.value += `[${new Date().toLocaleString()}] 没有选择主节点，跳过主节点部署\n`
    isDeploying.value = false
    steps.value[2].status = 'completed' // 标记主节点部署步骤为已完成
    // 自动进入工作节点部署步骤
    if (workerNodes.value.length > 0) {
      deployLogs.value += `[${new Date().toLocaleString()}] 检测到有工作节点，自动进入工作节点部署步骤\n`
      currentStep.value = 3
      await deployWorkerNodes()
    } else {
      // 如果也没有工作节点，直接进入完成步骤
      deployLogs.value += `[${new Date().toLocaleString()}] 没有工作节点需要部署，直接进入完成步骤\n`
      currentStep.value = 4
    }
    return
  }

  isDeploying.value = true
  deployLogs.value += `[${new Date().toLocaleString()}] 开始部署主节点...\n`

  // 初始化部署状态
  masterNodes.value.forEach(node => {
    deploymentStatus.value.master[node.id] = 'deploying'
    deploymentProgress.value.master[node.id] = 0
  })

  try {
    // 只支持单主节点部署，取第一个主节点
    const masterNode = masterNodes.value[0]
    
    // 调用后端API初始化主节点
    const response = await apiClient.post('/kubeadm/init', {
      masterNodeId: masterNode.id,
      config: {
        apiVersion: "kubeadm.k8s.io/v1beta3",
        kind: "InitConfiguration",
        localAPIEndpoint: {
          advertiseAddress: masterNode.ip,
          bindPort: deployConfig.value.apiServerPort
        },
        nodeRegistration: {
          criSocket: `unix:///run/${deployConfig.value.containerRuntime}/${deployConfig.value.containerRuntime}.sock`
        },
        clusterConfiguration: {
          kubernetesVersion: deployConfig.value.kubeVersion,
          networking: {
            podSubnet: deployConfig.value.podCIDR,
            serviceSubnet: deployConfig.value.serviceCIDR,
            dnsDomain: "cluster.local"
          }
        }
      },
      skipSteps: Object.keys(skipSteps.value).filter(stepId => skipSteps.value[stepId])
    })
    
    deployLogs.value += `主节点部署请求已发送，正在等待部署结果...\n`
    deployLogs.value += `初始化主节点的脚本正在执行中，完成后会自动进入下一步...\n`
    
    // 处理API响应中的joinCommand字段
    if (response.data && response.data.joinCommand) {
      const apiJoinCommand = response.data.joinCommand
      deployLogs.value += `[${new Date().toLocaleString()}] 从API响应中获取到join命令: ${apiJoinCommand}\n\n`
      
      // 保存join命令到本地存储和状态
      joinToken.value = apiJoinCommand
      localStorage.setItem('kubeadmJoinCommand', apiJoinCommand)
      sessionStorage.setItem('kubeadmJoinCommand', apiJoinCommand)
      
      // 保存token有效期信息，默认24小时
      const tokenExpiry = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString()
      localStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
      sessionStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
      
      // 更新部署状态
      isDeploying.value = false
      steps.value[2].status = 'completed'
      
      // 自动进入下一步
      const workerNodeIds = Object.keys(selectedNodes.value).filter(nodeId => selectedNodes.value[nodeId] === 'worker')
      if (workerNodeIds.length > 0 && currentStep.value === 2) {
        deployLogs.value += `[${new Date().toLocaleString()}] 从API获取到join命令，自动进入工作节点部署步骤\n\n`
        currentStep.value = 3
        deployLogs.value += `[${new Date().toLocaleString()}] 自动开始部署工作节点...\n\n`
        deployWorkerNodes()
      } else if (workerNodeIds.length === 0 && currentStep.value === 2) {
        deployLogs.value += `[${new Date().toLocaleString()}] 从API获取到join命令，没有工作节点需要部署，直接进入完成步骤\n\n`
        currentStep.value = 4
      }
    }
    
    // API调用成功，isDeploying状态已在前面根据joinCommand处理结果设置
    
    // 删除了120秒超时提示，改为自动检测join token并推进流程
  } catch (error) {
    deployLogs.value += '部署请求发送失败: ' + (error.response?.data?.error || error.message) + '\n'
    steps.value[2].status = 'failed'
    // 设置所有主节点为失败状态
    masterNodes.value.forEach(node => {
      deploymentStatus.value.master[node.id] = 'failed'
    })
    isDeploying.value = false
  }
}

// 部署工作节点
const deployWorkerNodes = async () => {
  startWorkerDeployment()
}

// 开始工作节点部署
const startWorkerDeployment = async () => {
  isDeploying.value = true
  deployLogs.value += `\n[${new Date().toLocaleString()}] 开始部署工作节点...\n`
  
  // 初始化部署步骤
  workerDeploymentStep.value = 0
  
  // 获取用户选择的部署步骤
  const selectedStepIds = Object.keys(selectedWorkerSteps.value).filter(stepId => selectedWorkerSteps.value[stepId])
  const selectedStepNames = selectedStepIds.map(stepId => {
    const step = workerDeploySteps.value.find(s => s.id === stepId)
    return step ? step.name : stepId
  })
  deployLogs.value += `[${new Date().toLocaleString()}] 执行的工作节点部署步骤: ${selectedStepNames.length > 0 ? selectedStepNames.join(', ') : '无'}\n\n`
  
  // 初始化部署状态
  workerNodes.value.forEach(node => {
    deploymentStatus.value.worker[node.id] = 'deploying'
    deploymentProgress.value.worker[node.id] = 0
    deploymentTimestamps.value.worker[node.id] = new Date().toISOString()
  })
  
  try {
    // 获取join token，优先使用手动输入的，然后是自动提取的，最后是localStorage中的
    const token = manualJoinToken.value || joinToken.value || localStorage.getItem('kubeadmJoinCommand')
    if (!token) {
      throw new Error('没有找到join token，请先部署主节点或手动输入join token')
    }
    
    // 如果是手动输入的token，保存到localStorage和ref中
    if (manualJoinToken.value) {
      joinToken.value = manualJoinToken.value
      localStorage.setItem('kubeadmJoinCommand', manualJoinToken.value)
      sessionStorage.setItem('kubeadmJoinCommand', manualJoinToken.value)
      // 保存token有效期信息，默认24小时
      const tokenExpiry = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString()
      localStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
      sessionStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
    }
    
    // 解析join token，提取必要信息
    const tokenMatch = token.match(/--token\s+(\S+)/)
    const caCertHashMatch = token.match(/--discovery-token-ca-cert-hash\s+(\S+)/)
    
    if (!tokenMatch || !caCertHashMatch) {
      throw new Error('join token格式不正确，无法提取必要信息')
    }
    
    const joinTokenValue = tokenMatch[1]
    const caCertHash = caCertHashMatch[1]
    const apiServerAddress = token.match(/kubeadm join\s+(\S+)/)[1]
    
    deployLogs.value += `[${new Date().toLocaleString()}] 解析join token成功: API Server地址: ${apiServerAddress}, Token: ${joinTokenValue}, CA Cert Hash: ${caCertHash}\n`
    
    // 获取工作节点ID列表
    const workerNodeIds = workerNodes.value.map(node => node.id)
    
    // 获取要跳过的步骤
    const allWorkerStepIds = workerDeploySteps.value.map(step => step.id)
    const skipWorkerSteps = allWorkerStepIds.filter(stepId => !selectedStepIds.includes(stepId))
    const convertedSkipSteps = skipWorkerSteps.map(stepId => stepId.replace('worker_', ''))
    
    // 调用完整的部署API，而不是直接调用kubeadm join
    // 这样可以确保所有必要的前置步骤（如安装kubeadm）都被执行
    deployLogs.value += `[${new Date().toLocaleString()}] 准备调用API: ${getApiBaseUrl()}/k8s/deploy\n`
    deployLogs.value += `[${new Date().toLocaleString()}] 请求参数: ${JSON.stringify({
      kubeVersion: deployConfig.value.kubeVersion,
      arch: 'amd64',
      distro: workerNodes.value[0]?.os || deployConfig.value.distro,
      nodeIds: workerNodeIds,
      skipSteps: convertedSkipSteps,
      joinToken: joinTokenValue,
      caCertHash: caCertHash,
      controlPlaneEndpoint: apiServerAddress
    })}\n`
    try {
      await apiClient.post('/k8s/deploy', {
        kubeVersion: deployConfig.value.kubeVersion,
        arch: 'amd64',
        distro: workerNodes.value[0]?.os || deployConfig.value.distro,
        nodeIds: workerNodeIds,
        skipSteps: convertedSkipSteps,
        // 将join token信息传递给后端
        joinToken: joinTokenValue,
        caCertHash: caCertHash,
        controlPlaneEndpoint: apiServerAddress
      })
    } catch (error) {
      deployLogs.value += `[${new Date().toLocaleString()}] API调用失败详情: ${JSON.stringify(error, Object.getOwnPropertyNames(error))}\n`
      throw error
    }
    
    deployLogs.value += `[${new Date().toLocaleString()}] 工作节点部署请求已发送，正在等待部署结果...\n`
    
    // 保持isDeploying为true，直到收到部署完成的SSE消息
    // 这样可以更准确地反映实际部署状态
    
    // 更新部署步骤到配置工作节点
    workerDeploymentStep.value = 1
  } catch (error) {
    deployLogs.value += `[${new Date().toLocaleString()}] 部署请求发送失败: ${error.response?.data?.error || error.message}\n\n`
    steps.value[3].status = 'failed'
    workerDeploymentStep.value = -1 // 部署失败
    // 设置所有工作节点为失败状态
    workerNodes.value.forEach(node => {
      deploymentStatus.value.worker[node.id] = 'failed'
      deploymentTimestamps.value.worker[node.id] = new Date().toISOString()
    })
    isDeploying.value = false
  }
}

// 日志自动滚动处理
const scrollLogsToBottom = () => {
  if (autoScrollLogs.value && logsContainer.value) {
    const container = logsContainer.value
    container.scrollTop = container.scrollHeight
  }
}

// 监听日志变化，自动滚动到底部
watch(deployLogs, () => {
  scrollLogsToBottom()
})

// 组件挂载时添加日志滚动监听
onMounted(() => {
  try {
    console.log('KubeadmManager组件已挂载')
    // 加载保存的状态
    loadState()
    // 初始化SSE连接
    initSSE()
    // 初始滚动日志到底部
    scrollLogsToBottom()
  } catch (error) {
    console.error('KubeadmManager组件挂载失败:', error)
    deployLogs.value += `[${new Date().toLocaleString()}] 组件初始化失败: ${error.message}\n\n`
    emit('showMessage', `组件初始化失败: ${error.message}`, 'error')
  }
})

// 完整集群部署
const deployFullCluster = async (workerStepIds = []) => {
  deployLogs.value += `[${new Date().toLocaleString()}] 开始完整集群部署...\n`
  
  try {
    // 创建新的AbortController，用于取消部署
    abortController.value = new AbortController()
    
    // 获取所有选中节点ID
    const selectedNodeIds = Object.keys(selectedNodes.value)
    
    // 获取第一个选中节点的操作系统类型，假设所有节点使用相同的操作系统
    let distro = 'ubuntu' // 默认值
    if (selectedNodeIds.length > 0) {
      const firstNodeId = selectedNodeIds[0]
      const firstNode = props.nodes.find(node => node.id === firstNodeId)
      if (firstNode && firstNode.os) {
        // 将操作系统名称转换为小写，确保与后端期望的格式一致
        distro = firstNode.os.toLowerCase()
      }
    }
    
    // 工作节点部署步骤完全独立，不依赖主节点设置
    let skipStepArray = []
    
    // 如果是工作节点部署，根据用户选择的工作节点步骤来确定跳过的步骤
    if (workerStepIds.length > 0) {
      // 获取所有工作节点步骤的ID列表
      const allWorkerStepIds = workerDeploySteps.value.map(step => step.id)
      
      // 确定要跳过的工作节点步骤（即未被选中的步骤）
      const skipWorkerSteps = allWorkerStepIds.filter(stepId => !workerStepIds.includes(stepId))
      
      // 将工作节点跳过步骤转换为与后端期望的格式一致
      const convertedSkipWorkerSteps = skipWorkerSteps.map(stepId => {
        // 将工作节点步骤ID转换为主节点步骤ID格式
        // 例如：worker_system_preparation -> system_preparation
        return stepId.replace('worker_', '')
      })
      
      // 仅使用工作节点选择的跳过步骤，不合并主节点跳过步骤
      skipStepArray = convertedSkipWorkerSteps
      
      deployLogs.value += `[${new Date().toLocaleString()}] 工作节点部署跳过的步骤: ${skipStepArray.length > 0 ? skipStepArray.join(', ') : '无'}\n`
    } else {
      // 主节点部署，使用主节点的跳过步骤
      skipStepArray = Object.keys(skipSteps.value).filter(stepId => skipSteps.value[stepId])
      deployLogs.value += `[${new Date().toLocaleString()}] 主节点部署跳过的步骤: ${skipStepArray.length > 0 ? skipStepArray.join(', ') : '无'}\n`
    }
    
    // 调用完整部署API，传递跳过的步骤
    await apiClient.post('/k8s/deploy', {
      kubeVersion: deployConfig.value.kubeVersion,
      arch: 'amd64',
      distro: distro,
      nodeIds: selectedNodeIds,
      skipSteps: skipStepArray
    }, {
      signal: abortController.value.signal
    })
    
    deployLogs.value += `完整集群部署请求已发送，正在等待部署结果...\n`
  } catch (error) {
    if (error.name === 'AbortError') {
      deployLogs.value += `[${new Date().toLocaleString()}] 部署已被用户取消\n`
    } else {
      deployLogs.value += `完整集群部署失败: ${error.response?.data?.error || error.message}\n`
      throw error
    }
  }
}

// 停止部署
const stopDeployment = () => {
  if (abortController.value) {
    deployLogs.value += `[${new Date().toLocaleString()}] 正在取消部署...\n`
    abortController.value.abort()
    abortController.value = null
    isDeploying.value = false
    emit('showMessage', { text: '部署已取消!', type: 'warning' })
  }
}

// 安装容器运行时
const installContainerRuntime = async () => {
  isDeploying.value = true
  deployLogs.value = '开始检查容器运行时...\n'
  
  // 获取没有容器运行时的节点
  const selectedNodeIds = Object.keys(selectedNodes.value)
  const nodesWithoutRuntime = selectedNodeIds.filter(nodeId => {
    const node = props.nodes.find(n => n.id === nodeId)
    return !node.containerRuntime || node.containerRuntime === ''
  })
  
  try {
    // 检查是否有节点需要安装容器运行时
    if (nodesWithoutRuntime.length === 0) {
      deployLogs.value += '所有节点都已安装容器运行时，跳过安装步骤...\n'
      isDeploying.value = false
      // 直接进入下一步
      await goToNextStep(true)
      return
    }
    
    // 由于后端暂不支持自动安装容器运行时，显示提示信息
    const nodeNames = nodesWithoutRuntime.map(nodeId => {
      const node = props.nodes.find(n => n.id === nodeId)
      return node ? node.name : nodeId
    })
    
    deployLogs.value += `以下节点需要安装容器运行时: ${nodeNames.join(', ')}\n`
    deployLogs.value += `当前版本的后端暂不支持自动安装容器运行时，请手动在这些节点上安装 ${deployConfig.value.containerRuntime}\n`
    deployLogs.value += `安装完成后，请更新节点信息并重新开始部署流程\n`
    
    emit('showMessage', { 
      text: `请手动在节点 ${nodeNames.join(', ')} 上安装 ${deployConfig.value.containerRuntime} 容器运行时`, 
      type: 'warning' 
    })
    
    isDeploying.value = false
  } catch (error) {
    deployLogs.value += '检查容器运行时失败: ' + (error.response?.data?.error || error.message) + '\n'
    emit('showMessage', { text: '检查容器运行时失败: ' + (error.response?.data?.error || error.message), type: 'error' })
    isDeploying.value = false
  }
}

// 复制join token到剪贴板
const copyJoinToken = async () => {
  const tokenToCopy = joinToken.value || manualJoinToken.value
  if (tokenToCopy) {
    try {
      await navigator.clipboard.writeText(tokenToCopy)
      emit('showMessage', { text: 'Join命令已复制到剪贴板!', type: 'success' })
    } catch (error) {
      // 如果剪贴板API不可用，使用传统的复制方式
      const textArea = document.createElement('textarea')
      textArea.value = tokenToCopy
      textArea.style.position = 'fixed'
      textArea.style.left = '-999999px'
      textArea.style.top = '-999999px'
      document.body.appendChild(textArea)
      textArea.focus()
      textArea.select()
      try {
        document.execCommand('copy')
        emit('showMessage', { text: 'Join命令已复制到剪贴板!', type: 'success' })
      } catch (err) {
        emit('showMessage', { text: '复制失败，请手动复制!', type: 'error' })
      }
      document.body.removeChild(textArea)
    }
  }
}

// 刷新join命令
const refreshJoinToken = async () => {
  deployLogs.value += `[${new Date().toLocaleString()}] 开始刷新join命令...\n`
  
  try {
    // 调用后端API获取join命令
    const response = await apiClient.get('/kubeadm/join-command')
    
    if (response.data && response.data.command) {
      const freshJoinCommand = response.data.command
      deployLogs.value += `[${new Date().toLocaleString()}] 成功获取最新join命令: ${freshJoinCommand}\n\n`
      
      // 更新本地存储和状态
      joinToken.value = freshJoinCommand
      localStorage.setItem('kubeadmJoinCommand', freshJoinCommand)
      sessionStorage.setItem('kubeadmJoinCommand', freshJoinCommand)
      
      // 保存token有效期信息，默认24小时
      const tokenExpiry = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString()
      localStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
      sessionStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
      
      emit('showMessage', { text: 'join命令刷新成功', type: 'success' })
    } else {
      deployLogs.value += `[${new Date().toLocaleString()}] 获取join命令失败: 响应格式不正确\n\n`
      emit('showMessage', { text: '获取join命令失败: 响应格式不正确', type: 'error' })
    }
  } catch (error) {
    deployLogs.value += `[${new Date().toLocaleString()}] 获取join命令失败: ${error.response?.data?.error || error.message}\n\n`
    emit('showMessage', { text: '获取join命令失败: ' + (error.response?.data?.error || error.message), type: 'error' })
  }
}

// 使用手动输入的Join Token
const useManualJoinToken = () => {
  if (manualJoinToken.value) {
    // 保存手动输入的token到ref和localStorage中
    joinToken.value = manualJoinToken.value
    localStorage.setItem('kubeadmJoinCommand', manualJoinToken.value)
    sessionStorage.setItem('kubeadmJoinCommand', manualJoinToken.value)
    
    // 保存token有效期信息，默认24小时
    const tokenExpiry = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString()
    localStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
    sessionStorage.setItem('kubeadmJoinTokenExpiry', tokenExpiry)
    
    deployLogs.value += `[${new Date().toLocaleString()}] 已保存手动输入的join token\n\n`
    emit('showMessage', { text: '已保存手动输入的Join命令!', type: 'success' })
  }
}

// 刷新Join Token函数已在上方实现

// 重试节点部署
const retryNodeDeployment = async (nodeId) => {
  try {
    const node = workerNodes.value.find(n => n.id === nodeId)
    if (!node) return
    
    deployLogs.value += `[${new Date().toLocaleString()}] 正在重试部署节点: ${node.name} (${node.ip})\n`
    
    // 重置节点状态
    deploymentStatus.value.worker[nodeId] = 'deploying'
    deploymentProgress.value.worker[nodeId] = 0
    deploymentTimestamps.value.worker[nodeId] = new Date().toISOString()
    
    // 这里可以添加调用后端API重试节点部署的逻辑
    // 目前我们只是模拟重试过程
    setTimeout(() => {
      deployLogs.value += `[${new Date().toLocaleString()}] 节点部署重试功能尚未实现，可手动操作或重新开始部署\n\n`
      emit('showMessage', { text: `已重置节点 ${node.name} 状态，可重新部署`, type: 'info' })
      
      // 重置状态为待部署
      deploymentStatus.value.worker[nodeId] = undefined
      deploymentProgress.value.worker[nodeId] = 0
    }, 2000)
  } catch (error) {
    deployLogs.value += `[${new Date().toLocaleString()}] 重试节点部署失败: ${error.message}\n\n`
    emit('showMessage', { text: '重试节点部署失败!', type: 'error' })
  }
}

// 清空日志
const clearLogs = () => {
  deployLogs.value = 'Kubernetes集群部署日志\n=====================\n'
  emit('showMessage', { text: '日志已清空!', type: 'info' })
}

// 切换日志自动滚动
const toggleAutoScroll = () => {
  autoScrollLogs.value = !autoScrollLogs.value
  emit('showMessage', { text: autoScrollLogs.value ? '已开启日志自动滚动' : '已关闭日志自动滚动', type: 'info' })
}

// 手动检查部署状态
const checkDeploymentStatus = async () => {
  try {
    // 这里可以添加调用后端API检查部署状态的逻辑
    // 目前我们只是更新UI状态，显示检查中
    deployLogs.value += `[${new Date().toLocaleString()}] 正在检查部署状态...\n`
    
    // 模拟检查过程
    setTimeout(() => {
      deployLogs.value += `[${new Date().toLocaleString()}] 部署状态检查完成\n\n`
      emit('showMessage', { text: '部署状态检查完成!', type: 'info' })
      
      // 检查所有工作节点是否都已部署完成
      const allWorkersCompleted = Object.values(deploymentStatus.value.worker).every(status => status === 'completed')
      const hasWorkers = Object.keys(deploymentStatus.value.worker).length > 0
      
      if (allWorkersCompleted && hasWorkers && currentStep.value === 3) {
        // 所有工作节点部署完成，更新步骤状态
        steps.value[3].status = 'completed'
        workerDeploymentStep.value = 4 // 完成所有步骤
        // 可以选择自动进入完成步骤
        // currentStep.value = 4
        // isDeploying.value = false
      }
    }, 1000)
  } catch (error) {
    deployLogs.value += `[${new Date().toLocaleString()}] 检查部署状态失败: ${error.message}\n\n`
    emit('showMessage', { text: '检查部署状态失败!', type: 'error' })
  }
}

// 完成部署
const finishDeployment = () => {
  deployLogs.value += `[${new Date().toLocaleString()}] Kubernetes集群部署完成!\n`
  deployLogs.value += `==========================================\n`
  deployLogs.value += `集群信息：\n`
  deployLogs.value += `- 版本：${deployConfig.value.kubeVersion}\n`
  deployLogs.value += `- 主节点数：${masterNodes.value.length}\n`
  deployLogs.value += `- 工作节点数：${workerNodes.value.length}\n`
  deployLogs.value += `- 容器运行时：${deployConfig.value.containerRuntime}\n`
  deployLogs.value += `- Pod网络插件：${deployConfig.value.podNetwork}\n`
  deployLogs.value += `- API Server地址：${clusterInfo.value.apiServerAddress}\n`
  deployLogs.value += `==========================================\n`
  emit('showMessage', { text: 'Kubernetes集群部署完成!', type: 'success' })
  // 可以添加跳转到集群管理页面的逻辑
}

// 重置部署步骤
const resetDeployment = () => {
  // 显示确认对话框
  if (confirm('确定要重置部署步骤吗？这将清除所有选择和配置，不可恢复。')) {
    // 重置当前步骤
    currentStep.value = 0
    
    // 重置节点选择
    selectedNodes.value = {}
    selectedRuntimeFilter.value = ''
    selectedStatusFilter.value = ''
    
    // 重置部署配置
    deployConfig.value = {
      kubeVersion: '',
      podNetwork: 'calico',
      containerRuntime: 'containerd',
      serviceCIDR: '10.96.0.0/12',
      podCIDR: '192.168.0.0/16',
      apiServerPort: 6443,
      enableHA: false,
      enableMetrics: true,
      distro: 'ubuntu' // 默认发行版，可根据实际情况调整
    }
    
    // 重置步骤跳过配置
    skipSteps.value = {}
    
    // 重置部署状态
    isDeploying.value = false
    deployLogs.value = 'Kubernetes集群部署日志\n=====================\n'
    deploymentStatus.value = {
      master: {},
      worker: {}
    }
    deploymentProgress.value = {
      master: {},
      worker: {}
    }
    
    // 重置join token
    joinToken.value = ''
    localStorage.removeItem('kubeadmJoinCommand')
    
    // 重置步骤状态
    steps.value = [
      { title: '选择节点', status: '' },
      { title: '部署配置', status: '' },
      { title: '部署主节点', status: '' },
      { title: '部署工作节点', status: '' },
      { title: '部署完成', status: '' }
    ]
    
    // 重置集群信息
    clusterInfo.value = {
      apiServerAddress: '',
      clusterName: '',
      clusterId: ''
    }
    
    // 清除本地存储的状态
    localStorage.removeItem('kubeadmManagerState')
    
    // 显示成功消息
    emit('showMessage', { text: '部署步骤已重置', type: 'info' })
  }
}

// 获取部署状态文本
const getDeploymentStatusText = (status) => {
  const statusMap = {
    '': '未开始',
    'deploying': '部署中',
    'completed': '已完成',
    'failed': '部署失败'
  }
  return statusMap[status] || '未知状态'
}
</script>

<style scoped>
.kubeadm-manager {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

/* 统一的页面主容器样式 */
.dashboard-section {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 25px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}

/* 步骤指示器 */
.steps-indicator {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 20px 0 30px 0;
  padding: 20px;
  border-radius: var(--radius-lg);
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-color);
  position: relative;
  overflow: hidden;
}

/* 步骤进度线 */
.steps-indicator::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 5%;
  width: 90%;
  height: 4px;
  background-color: var(--border-color);
  transform: translateY(-50%);
  z-index: 0;
}

.steps-indicator::after {
  content: '';
  position: absolute;
  top: 50%;
  left: 5%;
  width: calc(90% * (var(--current-step, 0) / var(--total-steps, 4)));
  height: 4px;
  background-color: var(--primary-color);
  transform: translateY(-50%);
  z-index: 1;
  transition: width 0.5s ease;
}

.step-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  position: relative;
  flex: 1;
  z-index: 2;
  transition: all 0.3s ease;
  padding: 15px 10px;
}

.step-item::after {
  display: none;
}

.step-number {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background-color: var(--bg-card);
  color: var(--text-secondary);
  display: flex;
  justify-content: center;
  align-items: center;
  font-weight: 700;
  font-size: 1.2rem;
  margin-bottom: 10px;
  transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  border: 3px solid var(--border-color);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.step-title {
  font-size: 0.95rem;
  color: var(--text-secondary);
  text-align: center;
  transition: all 0.3s ease;
  font-weight: 500;
  line-height: 1.3;
  min-height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.step-status {
  margin-top: 8px;
  font-size: 1.4rem;
  font-weight: bold;
  transition: all 0.3s ease;
  opacity: 0;
  transform: scale(0.8);
}

/* 活跃步骤样式 */
.step-item.active .step-number {
  background-color: var(--primary-color);
  color: white;
  transform: scale(1.2);
  border-color: var(--primary-color);
  box-shadow: 0 4px 16px rgba(66, 153, 225, 0.4);
}

.step-item.active .step-title {
  color: var(--primary-color);
  font-weight: 700;
  transform: translateY(-2px);
}

.step-item.active .step-status {
  opacity: 1;
  transform: scale(1);
}

/* 已完成步骤样式 */
.step-item.completed .step-number {
  background-color: var(--success-color);
  color: white;
  border-color: var(--success-color);
  box-shadow: 0 4px 16px rgba(46, 204, 113, 0.4);
}

.step-item.completed .step-title {
  color: var(--success-color);
  font-weight: 600;
}

.step-item.completed .step-status {
  opacity: 1;
  transform: scale(1);
  color: var(--success-color);
}

/* 失败步骤样式 */
.step-item.failed .step-number {
  background-color: var(--error-color);
  color: white;
  border-color: var(--error-color);
  box-shadow: 0 4px 16px rgba(231, 76, 60, 0.4);
  animation: pulse 1s infinite;
}

.step-item.failed .step-title {
  color: var(--error-color);
  font-weight: 600;
}

.step-item.failed .step-status {
  opacity: 1;
  transform: scale(1);
  color: var(--error-color);
}

/* 脉冲动画 */
@keyframes pulse {
  0%, 100% {
    box-shadow: 0 4px 16px rgba(231, 76, 60, 0.4);
  }
  50% {
    box-shadow: 0 4px 24px rgba(231, 76, 60, 0.6);
  }
}

/* 步骤内容 */
.step-content {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 25px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  margin-bottom: 25px;
  margin-top: 20px;
}

/* 步骤标题样式 */
.step-content h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 20px 0;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  gap: 10px;
}

/* 步骤标题图标 */
.step-content h3::before {
  content: '📋';
  font-size: 1.2rem;
}

/* 步骤内容区域通用样式 */
.step-node-selection,
.step-deploy-config,
.step-master-deployment,
.step-worker-deployment,
.step-completion {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

/* 部署配置表单样式优化 */
.deploy-config-form {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-md);
  padding: 20px;
  border: 1px solid var(--border-color);
  margin-bottom: 20px;
}

/* 版本提示样式 */
.version-tip {
  margin-top: 8px;
  padding: 8px 12px;
  background-color: var(--info-color-light);
  border-left: 3px solid var(--info-color);
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  font-size: 0.85rem;
  line-height: 1.4;
}

/* 节点选择容器样式 */
.node-selection-container {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

/* 节点过滤器样式 */
.node-filters {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-md);
  padding: 20px;
  border: 1px solid var(--border-color);
}

/* 部署进度容器样式 */
.deployment-progress-container {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

/* 节点列表样式 */
.master-node-list,
.worker-node-list {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-md);
  padding: 20px;
  border: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  gap: 15px;
}

/* 部署节点项样式 */
.deployment-node-item {
  background: linear-gradient(135deg, var(--bg-card) 0%, rgba(255, 255, 255, 0.05) 100%);
  border-radius: var(--radius-sm);
  padding: 15px;
  border: 1px solid var(--border-color);
  transition: all 0.3s ease;
  box-shadow: var(--shadow-sm);
  position: relative;
  overflow: hidden;
}

.deployment-node-item::before {
  content: '';
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(52, 152, 219, 0.1), transparent);
  transition: left 0.5s ease;
}

.deployment-node-item:hover {
  box-shadow: var(--shadow-md);
  border-color: var(--primary-color);
  transform: translateY(-1px);
}

.deployment-node-item:hover::before {
  left: 100%;
}

.deployment-node-item.deployed {
  border-color: var(--success-color);
  background: linear-gradient(135deg, rgba(39, 174, 96, 0.05) 0%, rgba(39, 174, 96, 0.1) 100%);
  box-shadow: 0 0 0 1px var(--success-color), var(--shadow-sm);
}

.deployment-node-item.failed {
  border-color: var(--error-color);
  background: linear-gradient(135deg, rgba(231, 76, 60, 0.05) 0%, rgba(231, 76, 60, 0.1) 100%);
  box-shadow: 0 0 0 1px var(--error-color), var(--shadow-sm);
  animation: shake 0.5s ease-in-out 1;
}

/* 部署节点项头部 */
.node-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.node-name {
  font-weight: 600;
  font-size: 0.95rem;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 8px;
}

.deployment-status {
  font-size: 0.8rem;
  font-weight: 600;
  padding: 4px 12px;
  border-radius: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  background-color: var(--bg-input);
  color: var(--text-secondary);
}

.deployment-status:empty {
  display: none;
}

.deployment-node-item.deployed .deployment-status {
  background-color: rgba(39, 174, 96, 0.2);
  color: var(--success-color);
}

.deployment-node-item.failed .deployment-status {
  background-color: rgba(231, 76, 60, 0.2);
  color: var(--error-color);
}

/* 进度条样式 */
.node-progress-bar {
  background-color: var(--bg-input);
  border-radius: var(--radius-full);
  height: 8px;
  overflow: hidden;
  margin-top: 10px;
  position: relative;
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.1);
}

.progress-bar {
  height: 100%;
  background: linear-gradient(90deg, var(--primary-color), var(--primary-color-light));
  border-radius: var(--radius-full);
  transition: width 0.3s ease;
  position: relative;
  overflow: hidden;
}

.progress-bar::after {
  content: '';
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.4), transparent);
  animation: progress-shine 2s infinite;
}

.progress-bar.failed {
  background: linear-gradient(90deg, var(--error-color), var(--error-color-light));
}

@keyframes progress-shine {
  0% {
    left: -100%;
  }
  100% {
    left: 100%;
  }
}

@keyframes shake {
  0%, 100% {
    transform: translateX(0);
  }
  10%, 30%, 50%, 70%, 90% {
    transform: translateX(-5px);
  }
  20%, 40%, 60%, 80% {
    transform: translateX(5px);
  }
}

/* 部署日志样式 */
.deployment-logs {
  background: linear-gradient(135deg, var(--bg-secondary) 0%, rgba(255, 255, 255, 0.05) 100%);
  border-radius: var(--radius-md);
  padding: 20px;
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-sm);
  position: relative;
  overflow: hidden;
}

/* 部署完成步骤样式 */
.step-completion .completion-summary {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* 页面主标题样式 */
.kubeadm-manager h2 {
  font-size: 1.2rem;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
  padding: 0;
}

/* 步骤内容卡片内部的子卡片样式 */
.summary-card {
  background-color: var(--bg-secondary);
  padding: 25px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  transition: all 0.3s ease;
}

.summary-card:hover {
  box-shadow: var(--shadow-md);
  border-color: var(--primary-color);
}

.summary-card h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
}

/* 部署节点列表样式优化 */
.available-nodes,
.selected-nodes-summary {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-md);
  padding: 20px;
  border: 1px solid var(--border-color);
}

.available-nodes h4,
.selected-nodes-summary h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
}

/* 节点网格样式优化 */
.nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 20px;
  margin-top: 15px;
}

/* 节点卡片样式优化 */
.node-card {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 20px;
  transition: all 0.3s ease;
  box-shadow: var(--shadow-sm);
}

.node-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
  border-color: var(--primary-color);
}

/* 部署流程状态展示 */
.deployment-status-display {
  display: flex;
  flex-direction: column;
  gap: 15px;
  background-color: var(--bg-secondary);
  border-radius: var(--radius-md);
  padding: 20px;
  border: 1px solid var(--border-color);
  margin-top: 20px;
}

.deployment-status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid var(--border-light);
}

.deployment-status-item:last-child {
  border-bottom: none;
}

/* 部署步骤导航样式 */
.step-navigation {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  padding: 20px;
  border: 1px solid var(--border-color);
  margin-top: 25px;
}

/* Join Token 区域样式 */
.join-token-section {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  margin-bottom: 25px;
}

.join-token-section h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
}

.join-token-section h4::before {
  content: '🔑';
  font-size: 1.1rem;
}

.join-token-container {
  position: relative;
}

.join-token {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-md);
  padding: 20px;
  border: 1px solid var(--border-color);
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.08);
}

.join-token pre {
  margin: 0 0 15px 0;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.9rem;
  line-height: 1.6;
  color: var(--text-primary);
  white-space: pre-wrap;
  word-wrap: break-word;
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  padding: 16px;
  overflow-x: auto;
  border-radius: var(--radius-sm);
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
}

.join-token pre:hover {
  background-color: rgba(52, 152, 219, 0.05);
  border-color: var(--primary-color);
}

.copy-token-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  font-size: 0.9rem;
  font-weight: 600;
  background: linear-gradient(135deg, var(--primary-color), var(--primary-dark));
  border: none;
  color: white;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: 0 2px 8px rgba(52, 152, 219, 0.3);
}

.copy-token-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(52, 152, 219, 0.4);
  background: linear-gradient(135deg, var(--primary-dark), var(--primary-color));
}

.copy-token-btn:active {
  transform: translateY(0);
  box-shadow: 0 2px 6px rgba(52, 152, 219, 0.3);
}

.no-token {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-secondary);
  font-style: italic;
  background-color: var(--bg-input);
  border-radius: var(--radius-md);
  border: 1px dashed var(--border-color);
}

/* 部署说明样式 */
.deployment-instructions {
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  padding: 20px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  margin-bottom: 25px;
}

.deployment-instructions h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
}

.deployment-instructions h4::before {
  content: '📋';
  font-size: 1.1rem;
}

.deployment-instructions ol {
  margin: 0;
  padding-left: 25px;
  color: var(--text-secondary);
  line-height: 1.8;
}

.deployment-instructions li {
  margin-bottom: 10px;
  background-color: var(--bg-input);
  padding: 10px 15px;
  border-radius: var(--radius-sm);
  border-left: 3px solid var(--primary-color);
  transition: all 0.3s ease;
}

.deployment-instructions li:hover {
  background-color: rgba(52, 152, 219, 0.1);
  transform: translateX(4px);
}

/* 手动部署操作样式 */
.manual-deployment-actions {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 20px;
  padding: 15px;
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-sm);
}

.manual-deployment-actions .btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  font-size: 0.9rem;
  font-weight: 600;
  background: linear-gradient(135deg, var(--primary-color), var(--primary-dark));
  border: none;
  color: white;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: 0 2px 8px rgba(52, 152, 219, 0.3);
}

.manual-deployment-actions .btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(52, 152, 219, 0.4);
  background: linear-gradient(135deg, var(--primary-dark), var(--primary-color));
}

.manual-deployment-actions .btn:active {
  transform: translateY(0);
  box-shadow: 0 2px 6px rgba(52, 152, 219, 0.3);
}

/* 每个步骤的内容卡片样式 */
.step-content > div {
  animation: fadeIn 0.5s ease;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* 步骤跳过配置样式 */
.skip-steps-config {
  background-color: var(--bg-secondary);
  border-radius: var(--radius-md);
  padding: 20px;
  border: 1px solid var(--border-color);
  margin: 20px 0;
}

.skip-steps-config h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  gap: 10px;
}

.skip-steps-config h3::before {
  content: '⚙️';
  font-size: 1.2rem;
}

.skip-steps-description {
  background-color: var(--bg-info);
  color: var(--text-info);
  padding: 10px 15px;
  border-radius: var(--radius-sm);
  margin-bottom: 20px;
  font-size: 0.9rem;
  border-left: 4px solid var(--primary-color);
  line-height: 1.5;
}

.skip-steps-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 15px;
}

.skip-step-item {
  background-color: var(--bg-card);
  border-radius: var(--radius-sm);
  padding: 15px;
  border: 1px solid var(--border-color);
  transition: all 0.3s ease;
  box-shadow: var(--shadow-sm);
}

.skip-step-item:hover {
  box-shadow: var(--shadow-md);
  border-color: var(--primary-color);
  transform: translateY(-2px);
  background-color: rgba(52, 152, 219, 0.05);
}

/* 调试信息样式 */
.debug-info {
  background-color: var(--bg-info);
  border-radius: var(--radius-md);
  padding: 20px;
  margin: 20px 0;
  border-left: 4px solid var(--primary-color);
}

.debug-info h4 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
}

.debug-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-color);
}

.debug-item:last-child {
  border-bottom: none;
}

.debug-label {
  font-weight: 500;
  color: var(--text-secondary);
}

.debug-value {
  font-weight: 600;
}

.debug-value.success {
  color: var(--success-color);
}

.debug-value.error {
  color: var(--error-color);
}

/* 必填项标记样式 */
.required {
  color: var(--error-color);
  font-weight: bold;
}

.skip-step-item .checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
  color: var(--text-primary);
  margin-bottom: 8px;
  cursor: pointer;
}

.skip-step-item .step-description {
  font-size: 0.85rem;
  color: var(--text-secondary);
  line-height: 1.5;
  margin-left: 24px;
}

/* 部署控制按钮样式 */
.deployment-controls {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 20px;
  padding: 15px;
  background-color: var(--bg-card);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-sm);
}

.deployment-controls .btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  font-size: 0.9rem;
  font-weight: 600;
  background: linear-gradient(135deg, var(--error-color), var(--error-dark));
  border: none;
  color: white;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: 0 2px 8px rgba(231, 76, 60, 0.3);
}

.deployment-controls .btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(231, 76, 60, 0.4);
  background: linear-gradient(135deg, var(--error-dark), var(--error-color));
}

.deployment-controls .btn:active {
  transform: translateY(0);
  box-shadow: 0 2px 6px rgba(231, 76, 60, 0.3);
}

/* 节点选择步骤 */
.step-node-selection .node-selection-container {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

.node-filters {
  background-color: var(--bg-secondary);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.available-nodes {
  margin-top: 20px;
}

.nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 15px;
  margin-top: 15px;
}

.node-card {
  background-color: var(--bg-card);
  border: 2px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 20px;
  transition: all 0.3s ease;
  cursor: pointer;
}

.node-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.node-card.selected {
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
}

.node-card.master {
  border-color: var(--success-color);
  background-color: rgba(39, 174, 96, 0.05);
}

.node-card.worker {
  border-color: var(--primary-color);
  background-color: rgba(52, 152, 219, 0.05);
}

.node-info h5 {
  margin: 0 0 10px 0;
  font-size: 1.1rem;
  font-weight: 600;
}

.node-meta {
  display: flex;
  flex-direction: column;
  gap: 5px;
  font-size: 0.85rem;
  color: var(--text-secondary);
}

.node-type-selector {
  display: flex;
  gap: 10px;
  margin-top: 15px;
}

.node-type-btn {
  flex: 1;
  padding: 8px 12px;
  border: 1px solid var(--border-color);
  background-color: var(--bg-secondary);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 0.85rem;
  font-weight: 500;
}

.node-type-btn:hover {
  background-color: var(--border-color);
}
/* 节点类型选择按钮 */
.node-type-btn.active {
  background-color: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

/* 高级配置切换样式 */
.advanced-toggle {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.3s ease;
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 15px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid var(--primary-color);
  display: inline-block;
}

.advanced-toggle:hover {
  color: var(--primary-light);
}

.toggle-icon {
  font-size: 0.8rem;
  transition: transform 0.3s ease;
}

.advanced-deploy-config {
  margin-top: 20px;
}

.advanced-deploy-config .skip-steps-config {
  margin-top: 15px;
  padding-top: 15px;
  border-top: 1px dashed var(--border-color);
}

/* 卡片标题折叠样式 */
.card-title-toggle {
  margin: 0;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  transition: all 0.3s ease;
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text-primary);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

/* 已选择节点摘要 */
.selected-nodes-summary {
  background-color: var(--bg-secondary);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  margin-top: 20px;
}

.selected-nodes-summary h4 {
  margin: 0 0 15px 0;
  font-size: 1rem;
  font-weight: 600;
}

.summary-info {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}

.summary-item {
  display: flex;
  gap: 8px;
  align-items: center;
}

.summary-label {
  font-weight: 500;
  color: var(--text-secondary);
}

.summary-value {
  font-weight: 600;
  color: var(--text-primary);
}

/* 部署配置步骤 */
.step-deploy-config .deploy-config-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* 部署进度步骤 */
.step-master-deployment, .step-worker-deployment {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

.deployment-progress-container {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 25px;
}

@media (max-width: 1024px) {
  .deployment-progress-container {
    grid-template-columns: 1fr;
  }
}

.master-node-list, .worker-node-list {
  background-color: var(--bg-secondary);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.deployment-node-item {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 15px;
  margin-bottom: 15px;
  transition: all 0.3s ease;
}

.deployment-node-item:last-child {
  margin-bottom: 0;
}

.deployment-node-item.deployed {
  border-color: var(--success-color);
  background-color: rgba(39, 174, 96, 0.05);
}

.deployment-node-item.failed {
  border-color: var(--error-color);
  background-color: rgba(231, 76, 60, 0.05);
}

.node-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.node-name {
  font-weight: 600;
  font-size: 0.95rem;
}

.deployment-status {
  font-size: 0.85rem;
  font-weight: 600;
  padding: 4px 12px;
  border-radius: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.deployment-status:empty {
  display: none;
}

.node-progress-bar {
  background-color: var(--bg-input);
  border-radius: var(--radius-sm);
  height: 8px;
  overflow: hidden;
  margin-top: 10px;
}

.progress-bar {
  height: 100%;
  background-color: var(--primary-color);
  border-radius: var(--radius-sm);
  transition: width 0.3s ease;
}

.progress-bar.failed {
  background-color: var(--error-color);
}

.deployment-logs {
  background-color: var(--bg-secondary);
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

.deployment-logs h4 {
  margin: 0 0 15px 0;
  font-size: 1rem;
  font-weight: 600;
}

.logs-container {
  background-color: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  max-height: 500px;
  overflow-y: auto;
  padding: 20px;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.9rem;
  line-height: 1.6;
  color: var(--text-secondary);
  margin-bottom: 15px;
  background-image: 
    linear-gradient(rgba(255, 255, 255, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.05) 1px, transparent 1px);
  background-size: 20px 20px;
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.1);
}

.logs-container pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}

.logs-container::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.logs-container::-webkit-scrollbar-track {
  background: var(--bg-secondary);
  border-radius: 4px;
  margin: 10px 0;
}

.logs-container::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 4px;
  transition: all 0.3s ease;
}

.logs-container::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
  transform: scale(1.1);
}

.logs-container::-webkit-scrollbar-corner {
  background: transparent;
}

/* 部署完成步骤样式 */
.step-completion .completion-summary {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.summary-card {
  background: linear-gradient(135deg, var(--bg-secondary) 0%, rgba(255, 255, 255, 0.05) 100%);
  padding: 25px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-sm);
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
}

.summary-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 4px;
  height: 100%;
  background: var(--primary-color);
  opacity: 0.5;
  transition: opacity 0.3s ease;
}

.summary-card:hover {
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.summary-card:hover::before {
  opacity: 1;
}

.summary-card h4 {
  margin: 0 0 20px 0;
  font-size: 1.1rem;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 10px;
  position: relative;
  padding-left: 24px;
}

.summary-card h4::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--primary-color);
  opacity: 0.2;
}

.summary-card.success {
  border-color: var(--success-color);
}

.summary-card.success::before {
  background: var(--success-color);
}

.summary-card.success h4::before {
  background: var(--success-color);
}

.summary-card.info {
  border-color: var(--primary-color);
}

.summary-card.info::before {
  background: var(--primary-color);
}

.summary-card.info h4::before {
  background: var(--primary-color);
}

.summary-card.warning {
  border-color: var(--warning-color);
}

.summary-card.warning::before {
  background: var(--warning-color);
}

.summary-card.warning h4::before {
  background: var(--warning-color);
}

/* 摘要统计样式 */
.summary-stats {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.stat-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--border-light);
  transition: all 0.3s ease;
}

.stat-item:last-child {
  border-bottom: none;
}

.stat-item:hover {
  transform: translateX(4px);
}

.stat-label {
  font-weight: 500;
  color: var(--text-secondary);
  font-size: 0.9rem;
}

.stat-value {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 0.95rem;
}

.stat-value.success {
  color: var(--success-color);
}

/* 集群信息样式 */
.cluster-info {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.info-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 12px 0;
  border-bottom: 1px solid var(--border-light);
  transition: all 0.3s ease;
}

.info-item:last-child {
  border-bottom: none;
}

.info-item:hover {
  transform: translateX(4px);
}

.info-label {
  font-weight: 500;
  color: var(--text-secondary);
  font-size: 0.9rem;
  flex: 1;
}

.info-value {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 0.9rem;
  flex: 2;
  text-align: right;
  word-break: break-all;
}

/* 后续操作建议样式 */
.next-steps {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.next-steps li {
  padding: 12px 16px;
  background: var(--bg-card);
  border-radius: var(--radius-sm);
  border-left: 4px solid var(--warning-color);
  transition: all 0.3s ease;
  position: relative;
  padding-left: 32px;
  font-size: 0.95rem;
}

.next-steps li::before {
  content: '💡';
  position: absolute;
  left: 10px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 1rem;
}

.next-steps li:hover {
  transform: translateX(4px);
  box-shadow: var(--shadow-sm);
  background: var(--bg-secondary);
}

/* 按钮样式优化 */
.step-navigation {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: linear-gradient(135deg, var(--bg-secondary) 0%, rgba(255, 255, 255, 0.05) 100%);
  border-radius: var(--radius-lg);
  padding: 20px;
  border: 1px solid var(--border-color);
  margin-top: 25px;
  box-shadow: var(--shadow-sm);
}

.btn {
  padding: 10px 24px;
  border: none;
  border-radius: var(--radius-md);
  font-size: 0.95rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
  box-shadow: var(--shadow-sm);
  display: flex;
  align-items: center;
  gap: 8px;
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

.btn-primary {
  background: linear-gradient(135deg, var(--primary-color), var(--primary-color-light));
  color: white;
}

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.btn-secondary {
  background: linear-gradient(135deg, var(--bg-input), var(--border-color));
  color: var(--text-primary);
}

.btn-secondary:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
  background: linear-gradient(135deg, var(--border-color), var(--bg-input));
}

.btn-success {
  background: linear-gradient(135deg, var(--success-color), var(--success-color-light));
  color: white;
}

.btn-success:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
}

.btn:disabled:hover {
  transform: none;
  box-shadow: none;
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

/* 表单样式优化 */
.form-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
  margin-bottom: 20px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-group label {
  font-weight: 500;
  font-size: 0.9rem;
  color: var(--text-primary);
}

.form-group select,
.form-group input {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background-color: var(--bg-input);
  color: var(--text-primary);
  font-size: 0.9rem;
  transition: all 0.3s ease;
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.05);
}

.form-group select:focus,
.form-group input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 3px rgba(52, 152, 219, 0.1);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-weight: 500;
  font-size: 0.9rem;
  color: var(--text-primary);
  transition: all 0.3s ease;
}

.checkbox-label:hover {
  color: var(--primary-color);
  transform: translateX(2px);
}

.checkbox-label input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: var(--primary-color);
}

/* 节点信息样式 */
.node-info h5 {
  margin: 0 0 10px 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.node-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 0.85rem;
  color: var(--text-secondary);
}

.node-meta span {
  display: flex;
  align-items: center;
  gap: 8px;
}

.node-meta span::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--border-color);
}

/* 节点选择操作样式 */
.node-selection-actions {
  margin-top: 15px;
}

.node-type-selector {
  display: flex;
  gap: 10px;
  margin-top: 15px;
}

.node-type-btn {
  flex: 1;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  background-color: var(--bg-secondary);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-primary);
  position: relative;
  overflow: hidden;
}

.node-type-btn:hover {
  background-color: var(--border-color);
  transform: translateY(-1px);
  box-shadow: var(--shadow-sm);
}

.node-type-btn.active {
  background: linear-gradient(135deg, var(--primary-color), var(--primary-color-light));
  color: white;
  border-color: var(--primary-color);
  box-shadow: var(--shadow-sm);
}

.node-type-btn.active::before {
  content: '';
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.2), transparent);
  transition: left 0.5s ease;
}

.node-type-btn.active:hover::before {
  left: 100%;
}

/* 现代化部署工作节点页面样式 */
.step-worker-deployment.modern {
  /* 现代化布局基础样式 */
}

/* 主要控制区样式 */
.main-control-panel {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 25px;
  flex-wrap: wrap;
  gap: 20px;
}

/* 状态概览卡片样式 */
.status-overview-card {
  background: linear-gradient(135deg, var(--bg-secondary), var(--bg-card));
  border-radius: var(--radius-lg);
  padding: 20px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
  flex: 1;
  min-width: 300px;
}

.status-overview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.status-overview-header h4 {
  margin: 0;
  font-size: 1.1rem;
  color: var(--text-primary);
}

.status-badge {
  padding: 6px 12px;
  border-radius: 20px;
  font-size: 0.85rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.status-badge.success {
  background-color: rgba(46, 204, 113, 0.2);
  color: var(--success-color);
}

.status-badge.warning {
  background-color: rgba(243, 156, 18, 0.2);
  color: var(--warning-color);
}

.status-badge.danger {
  background-color: rgba(231, 76, 60, 0.2);
  color: var(--error-color);
}

.status-badge.info {
  background-color: rgba(52, 152, 219, 0.2);
  color: var(--info-color);
}

.status-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
  gap: 15px;
}

.status-stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
}

.stat-number {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-primary);
}

.stat-label {
  font-size: 0.8rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.stat-icon {
  font-size: 1.2rem;
}

/* 核心内容区布局 */
.core-content {
  display: grid;
  grid-template-columns: 1fr 1.5fr;
  gap: 25px;
  margin-bottom: 25px;
}

/* 卡片基础样式 */
.card {
  background-color: var(--bg-card);
  border-radius: var(--radius-lg);
  padding: 0;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  overflow: hidden;
  margin-bottom: 25px;
  transition: all 0.3s ease;
}

.card:hover {
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.card-header {
  background: linear-gradient(135deg, var(--bg-secondary), var(--bg-card));
  padding: 15px 20px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h4 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.card-body {
  padding: 20px;
}

/* 左侧部署操作区样式 */
.deploy-operation-section {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

/* Join Token卡片样式 */
.join-token-card {
  /* Join Token卡片特定样式 */
}

.join-token-content {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.token-display {
  position: relative;
  background-color: var(--bg-input);
  border-radius: var(--radius-md);
  padding: 15px;
  border: 1px solid var(--border-color);
}

.token-text {
  margin: 0 0 15px 0;
  font-size: 0.85rem;
  font-family: 'Courier New', Courier, monospace;
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.copy-btn {
  align-self: flex-start;
  padding: 8px 16px;
  font-size: 0.85rem;
}

.token-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 15px;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 5px;
}

.token-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 30px 20px;
  text-align: center;
  gap: 10px;
}

.token-loading p {
  margin: 0;
  color: var(--text-muted);
}

/* 部署操作卡片样式 */
.deploy-actions-card {
  /* 部署操作卡片特定样式 */
}

.action-buttons {
  display: flex;
  gap: 15px;
  margin-bottom: 25px;
}

.deploy-mode-toggle {
  margin-bottom: 25px;
}

.deploy-mode-toggle h5 {
  margin: 0 0 15px 0;
  font-size: 0.95rem;
  color: var(--text-primary);
}

.toggle-group {
  display: flex;
  background-color: var(--bg-input);
  border-radius: var(--radius-md);
  overflow: hidden;
  border: 1px solid var(--border-color);
}

.toggle-btn {
  flex: 1;
  padding: 12px 20px;
  background-color: transparent;
  border: none;
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-muted);
  transition: all 0.3s ease;
}

.toggle-btn:hover {
  background-color: var(--border-color);
}

.toggle-btn.active {
  background-color: var(--primary-color);
  color: white;
}

/* 现代化部署指南样式 */
.deploy-guide h5 {
  margin: 0 0 15px 0;
  font-size: 0.95rem;
  color: var(--text-primary);
}

.guide-steps.modern {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.guide-steps.modern li {
  display: flex;
  gap: 15px;
  align-items: flex-start;
}

.guide-steps.modern .step-number {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background-color: var(--primary-color);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.8rem;
  font-weight: 600;
  flex-shrink: 0;
  margin-top: 2px;
}

.guide-steps.modern .step-content {
  flex: 1;
}

.guide-steps.modern .step-content strong {
  display: block;
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 5px;
}

.guide-steps.modern .step-content p {
  margin: 0 0 5px 0;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.guide-steps.modern .step-content code {
  background-color: var(--bg-input);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.8rem;
  font-family: 'Courier New', Courier, monospace;
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

/* 工作节点步骤选择样式 */
.worker-steps-card {
  /* 工作节点步骤选择卡片特定样式 */
}

.steps-selection-description {
  margin-bottom: 20px;
}

.steps-selection-description p {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.worker-steps-list {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.worker-step-item {
  background-color: var(--bg-input);
  padding: 15px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  transition: all 0.3s ease;
}

.worker-step-item:hover {
  box-shadow: var(--shadow-sm);
}

.step-selection {
  margin-bottom: 8px;
}

.step-selection .checkbox-label {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-primary);
  cursor: pointer;
}

.step-selection input[type="checkbox"] {
  width: 18px;
  height: 18px;
  accent-color: var(--primary-color);
}

.step-selection input[type="checkbox"]:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.step-description {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin-left: 28px;
  line-height: 1.4;
}

/* 右侧节点状态区样式 */
.nodes-status-section {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

/* 节点列表卡片样式 */
.nodes-list-card {
  /* 节点列表卡片特定样式 */
}

.nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 15px;
}

.node-card {
  background-color: var(--bg-input);
  border-radius: var(--radius-md);
  padding: 15px;
  border: 1px solid var(--border-color);
  transition: all 0.3s ease;
}

.node-card:hover {
  box-shadow: var(--shadow-sm);
  transform: translateY(-2px);
}

.node-card.status-completed {
  border-color: var(--success-color);
  background-color: rgba(46, 204, 113, 0.05);
}

.node-card.status-failed {
  border-color: var(--error-color);
  background-color: rgba(231, 76, 60, 0.05);
}

.node-card.status-deploying {
  border-color: var(--warning-color);
  background-color: rgba(243, 156, 18, 0.05);
}

.node-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.node-name {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-primary);
}

.node-status {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 4px 8px;
  border-radius: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.node-status.completed {
  background-color: rgba(46, 204, 113, 0.2);
  color: var(--success-color);
}

.node-status.failed {
  background-color: rgba(231, 76, 60, 0.2);
  color: var(--error-color);
}

.node-status.deploying {
  background-color: rgba(243, 156, 18, 0.2);
  color: var(--warning-color);
}

.node-info {
  display: flex;
  flex-direction: column;
  gap: 5px;
  margin-bottom: 15px;
}

.node-ip {
  font-size: 0.85rem;
  color: var(--text-muted);
}

.node-runtime {
  font-size: 0.8rem;
  background-color: var(--bg-card);
  padding: 4px 8px;
  border-radius: 12px;
  border: 1px solid var(--border-color);
  color: var(--text-muted);
  display: inline-block;
  align-self: flex-start;
}

.node-progress {
  margin-bottom: 15px;
}

.progress-bar-container {
  position: relative;
  height: 8px;
  background-color: var(--bg-card);
  border-radius: 4px;
  overflow: hidden;
  border: 1px solid var(--border-color);
}

.progress-bar {
  height: 100%;
  background-color: var(--primary-color);
  border-radius: 4px;
  transition: width 0.3s ease;
}

.progress-bar.failed {
  background-color: var(--error-color);
}

.progress-text {
  position: absolute;
  right: 10px;
  top: -2px;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-muted);
}

.node-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

/* 日志卡片样式 */
.logs-card {
  /* 日志卡片特定样式 */
}

.logs-container {
  background-color: var(--bg-input);
  border-radius: var(--radius-md);
  padding: 15px;
  border: 1px solid var(--border-color);
  height: 300px;
  overflow-y: auto;
}

.logs-content {
  margin: 0;
  font-size: 0.85rem;
  font-family: 'Courier New', Courier, monospace;
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.4;
}

/* 空状态样式 */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  text-align: center;
  gap: 10px;
  color: var(--text-muted);
}

.empty-icon {
  font-size: 2rem;
  margin-bottom: 10px;
}

.empty-state p {
  margin: 0;
  font-size: 0.9rem;
}

.empty-state .hint {
  font-size: 0.8rem;
  color: var(--text-muted);
}

/* 紧凑样式优化 */
.guide-steps.compact {
  gap: 10px;
}

.dashboard-stats.compact {
  gap: 15px;
}

.worker-node-list.compact {
  padding: 15px;
}

.manual-deployment-actions.compact {
  gap: 10px;
}

.deployment-logs-panel.compact {
  padding: 15px;
  margin-bottom: 20px;
}

.progress-text {
  position: absolute;
  right: 10px;
  top: -2px;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-muted);
}

/* 响应式设计优化 */
@media (max-width: 768px) {
  .steps-indicator {
    flex-direction: column;
    gap: 20px;
  }
  
  .steps-indicator::before,
  .steps-indicator::after {
    display: none;
  }
  
  .step-item {
    flex-direction: row;
    justify-content: flex-start;
    gap: 15px;
    width: 100%;
    text-align: left;
  }
  
  .step-number {
    margin-bottom: 0;
  }
  
  .step-title {
    min-height: auto;
    text-align: left;
  }
  
  .step-navigation {
    flex-direction: column;
    gap: 15px;
    align-items: stretch;
  }
  
  .form-row {
    grid-template-columns: 1fr;
  }
  
  .deployment-progress-container {
    grid-template-columns: 1fr;
  }
  
  .nodes-grid {
    grid-template-columns: 1fr;
  }
  
  .summary-stats,
  .cluster-info {
    gap: 12px;
  }
  
  .stat-item,
  .info-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }
  
  .info-value {
    text-align: left;
  }
}
</style>