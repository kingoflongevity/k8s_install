# 3节点Kubernetes部署测试方案

## 一、测试环境

| 节点角色 | 节点名称 | IP地址 | 操作系统 | 配置要求 |
|---------|---------|--------|---------|----------|
| Master | master-node | 192.168.31.206 | Rocky Linux 10 | 2CPU, 4GB RAM, 20GB Disk |
| Worker | worker-node1 | 192.168.31.207 | Rocky Linux 10 | 2CPU, 4GB RAM, 20GB Disk |
| Worker | worker-node2 | 192.168.31.208 | Rocky Linux 10 | 2CPU, 4GB RAM, 20GB Disk |

## 二、部署前准备

### 1. 环境检查
- [ ] 所有节点网络互通
- [ ] 所有节点时间同步
- [ ] 所有节点禁用swap
- [ ] 所有节点防火墙已关闭
- [ ] 所有节点SELINUX已禁用

### 2. 节点添加
通过前端界面或API添加3个节点：
- Master节点：192.168.31.206
- Worker节点1：192.168.31.207
- Worker节点2：192.168.31.208

## 三、部署步骤

### 1. 单节点Master部署

#### 步骤1：安装containerd（所有节点）
- 接口：POST `/nodes/:id/runtime/install`
- 参数：`{"runtimeType": "containerd"}`
- 验证：containerd服务运行正常，socket文件存在

#### 步骤2：安装Kubernetes组件（所有节点）
- 接口：POST `/nodes/:id/kubernetes/install`
- 参数：`{"kubeadmVersion": "1.30.0"}`
- 验证：kubelet、kubeadm、kubectl命令可用

#### 步骤3：初始化Master节点
- 接口：POST `/kubeadm/init`
- 参数：
  ```json
  {
    "nodeId": "master-node-id",
    "podNetworkCidr": "10.244.0.0/16",
    "serviceCidr": "10.96.0.0/12"
  }
  ```
- 验证：
  - Master节点状态Ready
  - 控制平面组件运行正常
  - 获取join命令

### 2. 添加Worker节点

#### 步骤4：将Worker节点加入集群
- 接口：POST `/kubeadm/join`
- 参数：
  ```json
  {
    "nodeIds": ["worker1-node-id", "worker2-node-id"],
    "joinCommand": "kubeadm join ..."
  }
  ```
- 验证：
  - 所有Worker节点状态Ready
  - 集群节点数量为3

### 3. 部署网络插件

#### 步骤5：部署Calico网络插件
- 接口：POST `/k8s/deploy`
- 参数：
  ```json
  {
    "nodeId": "master-node-id",
    "manifestUrl": "https://raw.githubusercontent.com/projectcalico/calico/v3.26.1/manifests/calico.yaml"
  }
  ```
- 验证：
  - Calico pods运行正常
  - 节点间网络连通

## 四、验证测试

### 1. 集群状态验证
- 命令：`kubectl get nodes`
- 预期：所有3个节点状态Ready

### 2. 控制平面组件验证
- 命令：`kubectl get pods -n kube-system`
- 预期：所有control plane pods状态Running

### 3. 网络连通性测试
- 部署测试Pod：`kubectl run test-pod --image=nginx`
- 测试Pod间通信：`kubectl exec -it test-pod -- ping -c 3 kubernetes.default.svc`
- 预期：通信成功

### 4. 服务发现测试
- 创建测试服务：`kubectl expose pod test-pod --port=80 --name=test-service`
- 测试服务访问：`kubectl run -it --rm test-client --image=busybox -- wget -qO- test-service`
- 预期：成功获取服务响应

## 五、错误处理

### 1. 常见错误及解决方案

| 错误类型 | 可能原因 | 解决方案 |
|---------|---------|---------|
| containerd安装失败 | 网络问题或包管理器配置错误 | 检查网络连接，手动安装containerd |
| Kubernetes组件安装失败 | repo配置错误或版本不兼容 | 检查repo配置，使用兼容版本 |
| Master初始化失败 | 端口占用或资源不足 | 检查端口占用，确保资源满足要求 |
| Worker加入失败 | join命令过期或网络问题 | 重新生成join命令，检查网络 |
| 节点NotReady | 网络插件未部署或配置错误 | 检查网络插件状态，重新部署 |

### 2. 回滚机制
- Master初始化失败：执行`kubeadm reset`清理环境
- Worker加入失败：执行`kubeadm reset`清理环境
- 集群部署失败：所有节点执行`kubeadm reset`，清理containerd数据

## 六、测试计划

| 测试阶段 | 测试时间 | 负责人 | 测试结果 |
|---------|---------|-------|----------|
| 环境准备 | 2026-01-07 | 系统 | ✅ |
| 单节点Master部署 | 2026-01-07 | 系统 | ⏳ |
| 2节点集群扩展 | 2026-01-07 | 系统 | ⏳ |
| 3节点集群完整测试 | 2026-01-07 | 系统 | ⏳ |
| 功能验证 | 2026-01-07 | 系统 | ⏳ |

## 七、测试成功标准

1. ✅ 3个节点成功加入集群
2. ✅ 所有节点状态Ready
3. ✅ 控制平面组件运行正常
4. ✅ 网络插件部署成功
5. ✅ Pod间通信正常
6. ✅ 服务发现功能正常
7. ✅ 可以成功部署和访问应用

## 八、后续优化

1. 实现自动化部署脚本
2. 增加部署进度监控
3. 实现一键回滚功能
4. 支持多版本Kubernetes部署
5. 增加HA Master部署支持
