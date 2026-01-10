# Kubernetes 一键部署工具

## 项目介绍

Kubernetes 一键部署工具是一个强大的自动化部署平台，专为简化 Kubernetes 集群的安装和管理而设计。无论您是 Kubernetes 新手还是经验丰富的管理员，我们的工具都能帮助您快速、可靠地部署和管理生产级 Kubernetes 集群。

## 🌟 核心亮点

- **一键部署**：只需几步操作，即可完成从节点配置到集群部署的全过程
- **可视化管理**：直观的 Web 界面，无需复杂的命令行操作
- **多版本支持**：支持 Kubernetes v1.27 至 v1.30 等多个稳定版本
- **跨平台兼容**：支持 Ubuntu 和 CentOS 等主流 Linux 发行版
- **自动化运维**：内置节点管理、日志监控、脚本管理等功能
- **安全可靠**：支持 SSH 密钥认证，确保部署过程的安全性

## 🚀 快速开始

### 环境要求

- 后端服务器：支持 Windows/Linux/macOS
- 节点要求：
  - 至少 2GB RAM
  - 至少 2 核 CPU
  - 20GB 可用磁盘空间
  - Linux 发行版（Ubuntu 20.04+/CentOS 7+）
  - SSH 服务已启用

### 部署步骤

1. **启动后端服务**
   ```bash
   cd backend
   ./k8s-installer  # Linux/macOS
   .\k8s-installer.exe  # Windows
   ```

2. **启动前端服务**
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

3. **访问 Web 界面**
   打开浏览器访问 `http://localhost:5173`

4. **配置节点**
   - 在 "节点管理" 页面添加您的服务器节点
   - 测试节点连接，确保 SSH 配置正确

5. **部署集群**
   - 在 "Kubeadm 管理" 页面下载所需版本的 Kubernetes 组件
   - 在 "部署管理" 页面选择要部署的节点和版本
   - 点击 "开始部署"，等待部署完成

## 📋 功能特性

### 1. 节点管理
- 添加、编辑、删除节点
- 测试节点连接状态
- 批量配置节点 SSH
- 管理节点凭证

### 2. Kubeadm 管理
- 查看可用的 Kubernetes 版本
- 下载和管理 Kubeadm 包
- 配置包源
- 查看本地已下载的包

### 3. 集群部署
- 一键初始化 Master 节点
- 自动拉取 Kubernetes 镜像
- 生成 Worker 节点加入命令
- 一键将 Worker 节点加入集群
- 集群重置功能

### 4. 日志管理
- 查看所有操作日志
- 按节点筛选日志
- 实时监控部署进度
- 导出日志

### 5. 脚本管理
- 自定义部署脚本
- 管理系统脚本
- 保存和加载脚本配置

## 📁 项目结构

```
k8s_install/
├── backend/           # Go 后端服务
│   ├── kubeadm/       # Kubeadm 相关功能
│   ├── node/          # 节点管理
│   ├── log/           # 日志管理
│   ├── script/        # 脚本管理
│   └── ssh/           # SSH 连接管理
├── frontend/          # Vue 3 前端应用
│   ├── src/           # 前端源代码
│   │   ├── components/ # Vue 组件
│   │   ├── App.vue    # 主应用组件
│   │   └── main.js    # 应用入口
│   └── public/        # 静态资源
└── deploy.sh          # 部署脚本
```

## 🛠️ 技术栈

### 后端
- **语言**：Go 1.21+
- **框架**：Gin
- **数据库**：SQLite
- **API**：RESTful API

### 前端
- **框架**：Vue 3
- **构建工具**：Vite
- **UI 组件**：自定义组件库
- **HTTP 客户端**：Axios

## 🔒 安全特性

- SSH 密钥认证支持
- 密码加密存储
- 权限控制
- 安全的 API 通信

## 📊 监控与日志

- 实时部署进度监控
- 详细的操作日志
- 错误日志分析
- 部署历史记录

## 🤝 贡献指南

我们欢迎社区贡献！如果您有任何想法或建议，欢迎提交 Issue 或 Pull Request。

### 开发环境设置

1. 克隆仓库
   ```bash
   git clone https://github.com/your-repo/k8s-installer.git
   ```

2. 后端开发
   ```bash
   cd backend
   go mod tidy
   go run main.go
   ```

3. 前端开发
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

## 📄 许可证

本项目采用 Apache 2.0 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 📞 支持与反馈

- **GitHub Issues**：[提交 Issue](https://github.com/your-repo/k8s-installer/issues)
- **邮件**：support@k8s-installer.com
- **文档**：[项目文档](https://docs.k8s-installer.com)

## 📈 版本更新

### v1.0.0 (2024-01-10)
- 初始版本发布
- 支持 Kubernetes v1.27-v1.30
- 实现一键部署功能
- 可视化 Web 界面
- 节点管理和日志监控

---

**让 Kubernetes 部署变得简单！** 🎉