# Kubernetes One-Click Deployment Tool

## Project Introduction

The Kubernetes One-Click Deployment Tool is a powerful automated deployment platform designed to simplify the installation and management of Kubernetes clusters. Whether you're a Kubernetes novice or an experienced administrator, our tool helps you quickly and reliably deploy and manage production-grade Kubernetes clusters.

## 🌟 Core Features

- **One-Click Deployment**: Complete the entire process from node configuration to cluster deployment in just a few steps
- **Visual Management**: Intuitive web interface, no complex command-line operations required
- **Multi-Version Support**: Supports multiple stable Kubernetes versions from v1.27 to v1.30
- **Cross-Platform Compatibility**: Supports mainstream Linux distributions like Ubuntu and CentOS
- **Automated Operations**: Built-in node management, log monitoring, script management, and more
- **Secure and Reliable**: Supports SSH key authentication to ensure deployment security

## 🚀 Quick Start

### Environment Requirements

- Backend Server: Supports Windows/Linux/macOS
- Node Requirements:
  - Minimum 2GB RAM
  - Minimum 2 CPU cores
  - 20GB available disk space
  - Linux distribution (Ubuntu 20.04+/CentOS 7+)
  - SSH service enabled

### Deployment Steps

1. **Start Backend Service**
   ```bash
   cd backend
   ./k8s-installer  # Linux/macOS
   .\k8s-installer.exe  # Windows
   ```

2. **Start Frontend Service**
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

3. **Access Web Interface**
   Open your browser and visit `http://localhost:5173`

4. **Configure Nodes**
   - Add your server nodes in the "Node Management" page
   - Test node connections to ensure SSH configuration is correct

5. **Deploy Cluster**
   - Download the required Kubernetes component versions in the "Kubeadm Management" page
   - Select nodes and version in the "Deployment Management" page
   - Click "Start Deployment" and wait for completion

## 📋 Feature Overview

### 1. Node Management
- Add, edit, and delete nodes
- Test node connection status
- Batch configure node SSH
- Manage node credentials

### 2. Kubeadm Management
- View available Kubernetes versions
- Download and manage Kubeadm packages
- Configure package sources
- View locally downloaded packages

### 3. Cluster Deployment
- One-click Master node initialization
- Automatic Kubernetes image pulling
- Generate Worker node join commands
- One-click Worker node cluster joining
- Cluster reset functionality

### 4. Log Management
- View all operation logs
- Filter logs by node
- Real-time deployment progress monitoring
- Export logs

### 5. Script Management
- Custom deployment scripts
- Manage system scripts
- Save and load script configurations

## 📁 Project Structure

```
k8s_install/
├── backend/           # Go backend service
│   ├── kubeadm/       # Kubeadm-related functionality
│   ├── node/          # Node management
│   ├── log/           # Log management
│   ├── script/        # Script management
│   └── ssh/           # SSH connection management
├── frontend/          # Vue 3 frontend application
│   ├── src/           # Frontend source code
│   │   ├── components/ # Vue components
│   │   ├── App.vue    # Main application component
│   │   └── main.js    # Application entry
│   └── public/        # Static resources
└── deploy.sh          # Deployment script
```

## 🛠️ Technology Stack

### Backend
- **Language**: Go 1.21+
- **Framework**: Gin
- **Database**: SQLite
- **API**: RESTful API

### Frontend
- **Framework**: Vue 3
- **Build Tool**: Vite
- **UI Components**: Custom component library
- **HTTP Client**: Axios

## 🔒 Security Features

- SSH key authentication support
- Password encryption storage
- Permission control
- Secure API communication

## 📊 Monitoring and Logging

- Real-time deployment progress monitoring
- Detailed operation logs
- Error log analysis
- Deployment history records

## 🤝 Contribution Guide

We welcome community contributions! If you have any ideas or suggestions, please submit an Issue or Pull Request.

### Development Environment Setup

1. Clone the repository
   ```bash
   git clone https://github.com/your-repo/k8s-installer.git
   ```

2. Backend development
   ```bash
   cd backend
   go mod tidy
   go run main.go
   ```

3. Frontend development
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

## 📄 License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.

## 📞 Support and Feedback

- **GitHub Issues**: [Submit Issue](https://github.com/your-repo/k8s-installer/issues)
- **Email**: support@k8s-installer.com
- **Documentation**: [Project Documentation](https://docs.k8s-installer.com)

## 📈 Version Updates

### v1.0.0 (2024-01-10)
- Initial version release
- Support for Kubernetes v1.27-v1.30
- One-click deployment functionality
- Visual web interface
- Node management and log monitoring

---

**Make Kubernetes deployment simple!** 🎉