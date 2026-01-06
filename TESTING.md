# Linux环境测试指南

## 环境要求

- Linux发行版：Ubuntu 20.04+、Debian 11+、CentOS 7+、RHEL 7+、Rocky Linux 8+、AlmaLinux 8+、Fedora 36+
- 硬件要求：至少2GB RAM，2核CPU，20GB磁盘空间
- 网络要求：能够访问互联网，用于下载依赖和Kubernetes组件

## 测试步骤

### 1. 克隆项目

```bash
git clone <项目仓库地址>
cd k8s_install
```

### 2. 运行部署脚本

部署脚本会自动安装所有依赖，包括Go、Node.js、Docker、kubeadm等，并构建和启动服务。

```bash
chmod +x deploy.sh
sudo ./deploy.sh
```

### 3. 验证服务状态

#### 3.1 检查后端服务

```bash
systemctl status k8s-installer-backend
```

预期输出：
```
● k8s-installer-backend.service - K8s Installer Backend
   Loaded: loaded (/etc/systemd/system/k8s-installer-backend.service; enabled; vendor preset: enabled)
   Active: active (running) since Wed 2026-01-06 17:00:00 UTC; 1min ago
 Main PID: 12345 (k8s-installer-back)
    Tasks: 10 (limit: 4915)
   Memory: 50.0M
   CPU: 1.234s
   CGroup: /system.slice/k8s-installer-backend.service
           └─12345 /path/to/k8s_install/backend/k8s-installer-backend
```

#### 3.2 检查API接口

```bash
curl http://localhost:8080/api/health
```

预期输出：
```json
{"status":"ok"}
```

#### 3.3 检查kubeadm版本

```bash
curl http://localhost:8080/api/kubeadm/version
```

预期输出：
```json
{"version":"v1.30.0"}
```

### 4. 前端访问

使用浏览器访问：
```
http://<服务器IP>:5173
```

### 5. 测试K8s集群部署

1. 在前端界面输入节点IP地址（例如：192.168.1.100）
2. 选择Kubernetes版本（默认：v1.30.0）
3. 配置Pod子网和Service子网（使用默认值即可）
4. 点击"初始化集群"按钮
5. 等待部署完成，查看部署日志
6. 部署完成后，获取工作节点加入命令

### 6. 测试工作节点加入

在另一台Linux机器上，执行获取到的加入命令：

```bash
sudo kubeadm join <control-plane-endpoint>:6443 --token <token> --discovery-token-ca-cert-hash <hash>
```

### 7. 验证集群状态

在主节点上执行：

```bash
kubectl get nodes
```

预期输出：
```
NAME     STATUS   ROLES           AGE     VERSION
master   Ready    control-plane   10m     v1.30.0
worker1  Ready    <none>          5m      v1.30.0
```

### 8. 测试集群重置

在前端界面点击"重置集群"按钮，确认重置操作。重置完成后，验证集群已被清理。

## 手动测试（可选）

如果不想使用部署脚本，可以手动执行以下步骤：

### 1. 安装依赖

#### 1.1 安装Go

```bash
wget -O go1.22.0.linux-amd64.tar.gz https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

#### 1.2 安装Node.js和npm

```bash
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs
```

#### 1.3 安装Docker

```bash
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo systemctl enable docker
sudo systemctl start docker
```

#### 1.4 安装kubeadm、kubelet和kubectl

```bash
sudo apt-get update
sudo apt-get install -y apt-transport-https ca-certificates curl
sudo curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
sudo apt-get update
sudo apt-get install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl
```

### 2. 构建和启动服务

#### 2.1 构建后端

```bash
cd backend
go mod tidy
go build -o k8s-installer-backend main.go
```

#### 2.2 构建前端

```bash
cd ../frontend
npm install
npm run build
```

#### 2.3 启动后端服务

```bash
cd ../backend
./k8s-installer-backend
```

## 故障排查

### 1. 后端服务启动失败

查看服务日志：

```bash
journalctl -u k8s-installer-backend -f
```

### 2. kubeadm版本获取失败

确保kubeadm已正确安装：

```bash
kubeadm version --short
```

如果kubeadm未安装，执行：

```bash
sudo apt-get install -y kubeadm
```

### 3. 前端无法访问

检查Node.js服务是否运行：

```bash
ps aux | grep node
```

如果未运行，手动启动：

```bash
cd frontend
npm run dev
```

### 4. 集群部署失败

查看部署日志，检查具体错误信息。常见问题包括：

- 网络问题：确保节点可以访问互联网
- 硬件资源不足：至少需要2GB RAM和2核CPU
- Docker服务未运行：检查Docker状态
- kubelet服务未运行：检查kubelet状态

## 清理测试环境

如果需要清理测试环境，执行以下命令：

```bash
# 停止服务
sudo systemctl stop k8s-installer-backend

# 重置Kubernetes集群
sudo kubeadm reset --force

# 清理Docker容器和镜像
sudo docker system prune -a -f

# 卸载kubeadm、kubelet和kubectl
sudo apt-get remove -y kubelet kubeadm kubectl

# 卸载Docker
sudo apt-get remove -y docker-ce docker-ce-cli containerd.io
```

## 测试结果记录

| 测试项 | 预期结果 | 实际结果 | 状态 |
|-------|---------|---------|------|
| 部署脚本执行 | 成功安装所有依赖 | | |
| 后端服务启动 | 服务运行正常 | | |
| 健康检查接口 | 返回{"status":"ok"} | | |
| kubeadm版本接口 | 返回kubeadm版本 | | |
| 前端访问 | 可以正常打开 | | |
| 集群初始化 | 成功初始化K8s集群 | | |
| 工作节点加入 | 工作节点成功加入集群 | | |
| 集群重置 | 成功清理集群 | | |

## 注意事项

1. 测试环境建议使用虚拟机或云服务器，避免影响生产环境
2. 部署过程中会下载大量依赖和Kubernetes组件，建议在网络良好的环境下进行
3. 首次部署可能需要较长时间（10-30分钟），请耐心等待
4. 确保测试机器有足够的磁盘空间（至少20GB）
5. 测试完成后，建议及时清理环境，避免资源浪费

## 联系信息

如果在测试过程中遇到问题，请联系：

- 项目维护者：<维护者邮箱>
- 项目仓库：<项目仓库地址>
- Issue跟踪：<Issue地址>
