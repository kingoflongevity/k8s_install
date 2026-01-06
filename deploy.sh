#!/bin/bash

# K8s安装工具部署脚本
# 适用于Ubuntu/Debian/CentOS/RHEL等Linux系统

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=====================================${NC}"
echo -e "${GREEN}   K8s 安装工具部署脚本   ${NC}"
echo -e "${GREEN}=====================================${NC}"

# 检查是否为root用户
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}错误: 请以root用户运行此脚本${NC}"
    exit 1
fi

# 检测Linux发行版
detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO=$ID
        VERSION=$VERSION_ID
    elif type lsb_release >/dev/null 2>&1; then
        DISTRO=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
        VERSION=$(lsb_release -sr)
    elif [ -f /etc/lsb-release ]; then
        . /etc/lsb-release
        DISTRO=$DISTRIB_ID
        VERSION=$DISTRIB_RELEASE
    elif [ -f /etc/debian_version ]; then
        DISTRO=debian
        VERSION=$(cat /etc/debian_version)
    elif [ -f /etc/centos-release ]; then
        DISTRO=centos
        VERSION=$(rpm -q --queryformat '%{VERSION}' centos-release)
    elif [ -f /etc/fedora-release ]; then
        DISTRO=fedora
        VERSION=$(rpm -q --queryformat '%{VERSION}' fedora-release)
    else
        DISTRO=unknown
        VERSION=unknown
    fi
    echo -e "${GREEN}检测到操作系统: $DISTRO $VERSION${NC}"
}

# 安装系统依赖
install_system_deps() {
    echo -e "${YELLOW}正在安装系统依赖...${NC}"
    
    case $DISTRO in
        ubuntu|debian)
            apt-get update
            apt-get install -y curl wget git build-essential
            ;;
        centos|rhel|rocky|almalinux)
            yum install -y curl wget git gcc-c++ make
            ;;
        fedora)
            dnf install -y curl wget git gcc-c++ make
            ;;
        *)
            echo -e "${RED}不支持的发行版: $DISTRO${NC}"
            exit 1
            ;;
    esac
}

# 安装Go
install_go() {
    echo -e "${YELLOW}正在安装Go...${NC}"
    GO_VERSION="1.22.0"
    
    # 下载并安装Go
    wget -O go${GO_VERSION}.linux-amd64.tar.gz https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
    tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
    rm go${GO_VERSION}.linux-amd64.tar.gz
    
    # 设置环境变量
    cat <<EOF > /etc/profile.d/go.sh
#!/bin/bash
export GOROOT=/usr/local/go
export GOPATH=HOME/go
export PATH=GOPATH/bin:GOROOT/bin:PATH
EOF
    
    source /etc/profile.d/go.sh
    
    echo -e "${GREEN}Go安装完成: $(go version)${NC}"
}

# 安装Node.js和npm
install_nodejs() {
    echo -e "${YELLOW}正在安装Node.js和npm...${NC}"
    NODE_VERSION="18"
    
    case $DISTRO in
        ubuntu|debian)
            curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash -
            apt-get install -y nodejs
            ;;
        centos|rhel|rocky|almalinux)
            curl -fsSL https://rpm.nodesource.com/setup_${NODE_VERSION}.x | bash -
            yum install -y nodejs
            ;;
        fedora)
            curl -fsSL https://rpm.nodesource.com/setup_${NODE_VERSION}.x | bash -
            dnf install -y nodejs
            ;;
    esac
    
    echo -e "${GREEN}Node.js安装完成: $(node --version)${NC}"
    echo -e "${GREEN}npm安装完成: $(npm --version)${NC}"
}

# 安装Docker
install_docker() {
    echo -e "${YELLOW}正在安装Docker...${NC}"
    
    # 卸载旧版本
    case $DISTRO in
        ubuntu|debian)
            apt-get remove -y docker docker-engine docker.io containerd runc
            ;;
        centos|rhel|rocky|almalinux|fedora)
            yum remove -y docker docker-client docker-client-latest docker-common docker-latest docker-latest-logrotate docker-logrotate docker-engine
            ;;
    esac
    
    # 安装Docker
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    
    # 启动Docker服务
    systemctl enable docker
    systemctl start docker
    
    echo -e "${GREEN}Docker安装完成: $(docker --version)${NC}"
}

# 安装kubeadm相关工具
install_kubeadm() {
    echo -e "${YELLOW}正在安装kubeadm、kubelet和kubectl...${NC}"
    
    case $DISTRO in
        ubuntu|debian)
            apt-get update
            apt-get install -y apt-transport-https ca-certificates curl
            curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
            echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /' | tee /etc/apt/sources.list.d/kubernetes.list
            apt-get update
            apt-get install -y kubelet kubeadm kubectl
            apt-mark hold kubelet kubeadm kubectl
            ;;
        centos|rhel|rocky|almalinux)
            cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/v1.30/rpm/
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/v1.30/rpm/repodata/repomd.xml.key
enabled=1
exclude=kubelet kubeadm kubectl cri-tools kubernetes-cni
EOF
            yum install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
            systemctl enable --now kubelet
            ;;
        fedora)
            cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/v1.30/rpm/
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/v1.30/rpm/repodata/repomd.xml.key
enabled=1
exclude=kubelet kubeadm kubectl cri-tools kubernetes-cni
EOF
            dnf install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
            systemctl enable --now kubelet
            ;;
    esac
    
    echo -e "${GREEN}kubeadm安装完成: $(kubeadm version --short)${NC}"
    echo -e "${GREEN}kubelet安装完成: $(kubelet --version)${NC}"
    echo -e "${GREEN}kubectl安装完成: $(kubectl version --client --short)${NC}"
}

# 构建后端服务
build_backend() {
    echo -e "${YELLOW}正在构建后端服务...${NC}"
    
    cd "$(dirname "$0")/backend"
    
    # 设置Go环境变量
    source /etc/profile.d/go.sh
    
    # 下载依赖
    go mod tidy
    
    # 构建
    go build -o k8s-installer-backend main.go
    
    echo -e "${GREEN}后端服务构建完成${NC}"
}

# 构建前端服务
build_frontend() {
    echo -e "${YELLOW}正在构建前端服务...${NC}"
    
    cd "$(dirname "$0")/frontend"
    
    # 安装依赖
    npm install
    
    # 构建
    npm run build
    
    echo -e "${GREEN}前端服务构建完成${NC}"
}

# 创建systemd服务文件
create_systemd_service() {
    echo -e "${YELLOW}正在创建systemd服务文件...${NC}"
    
    # 创建后端服务文件
    cat <<EOF > /etc/systemd/system/k8s-installer-backend.service
[Unit]
Description=K8s Installer Backend
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
WorkingDirectory=$(dirname "$0")/backend
ExecStart=$(dirname "$0")/backend/k8s-installer-backend
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    
    # 重新加载systemd
    systemctl daemon-reload
    
    echo -e "${GREEN}systemd服务文件创建完成${NC}"
}

# 启动服务
start_services() {
    echo -e "${YELLOW}正在启动服务...${NC}"
    
    # 启动后端服务
    systemctl enable k8s-installer-backend
    systemctl start k8s-installer-backend
    
    echo -e "${GREEN}服务启动完成${NC}"
}

# 显示部署完成信息
show_completion() {
    echo -e "${GREEN}=====================================${NC}"
    echo -e "${GREEN}   部署完成!   ${NC}"
    echo -e "${GREEN}=====================================${NC}"
    echo -e "${YELLOW}后端服务地址: http://localhost:8080${NC}"
    echo -e "${YELLOW}前端访问地址: http://localhost:5173${NC}"
    echo -e "${YELLOW}API文档: http://localhost:8080/api/health${NC}"
    echo -e "${GREEN}=====================================${NC}"
    echo -e "${GREEN}使用说明:${NC}"
    echo -e "1. 打开浏览器访问 http://localhost:5173"
    echo -e "2. 输入节点IP地址和Kubernetes版本"
    echo -e "3. 点击'初始化集群'按钮开始部署"
    echo -e "4. 部署完成后，可获取工作节点加入命令"
    echo -e "${GREEN}=====================================${NC}"
}

# 主函数
main() {
    detect_distro
    install_system_deps
    install_go
    install_nodejs
    install_docker
    install_kubeadm
    build_backend
    build_frontend
    create_systemd_service
    start_services
    show_completion
}

# 执行主函数
main
