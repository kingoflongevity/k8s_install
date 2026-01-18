package node

import (
	"k8s-installer/log"
	"time"
)

// Node 定义节点信息
type Node struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	IP               string    `json:"ip"`
	Port             int       `json:"port"`
	Username         string    `json:"username"`
	Password         string    `json:"password,omitempty"`
	PrivateKey       string    `json:"privateKey,omitempty"`
	NodeType         string    `json:"nodeType"`         // master 或 worker
	Status           string    `json:"status"`           // online, offline, ready, deploying
	ContainerRuntime string    `json:"containerRuntime"` // 容器运行时类型：containerd, cri-o
	OS               string    `json:"os"`               // 操作系统类型：ubuntu, centos, debian, rocky等
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// ContainerRuntimeConfig 容器运行时配置结构体
type ContainerRuntimeConfig struct {
	RuntimeType     string   `json:"runtimeType"`     // containerd, cri-o
	RegistryMirrors []string `json:"registryMirrors"` // 镜像加速地址
	CgroupDriver    string   `json:"cgroupDriver"`    // cgroup驱动
	LogDriver       string   `json:"logDriver"`       // 日志驱动
	LogMaxSize      string   `json:"logMaxSize"`      // 日志文件大小限制
	LogMaxFile      int      `json:"logMaxFile"`      // 日志文件数量限制
}

// NodeManager 节点管理器接口
type NodeManager interface {
	GetNodes() ([]Node, error)
	GetNode(id string) (*Node, error)
	CreateNode(node Node) (*Node, error)
	UpdateNode(id string, node Node) (*Node, error)
	DeleteNode(id string) error
	TestConnection(id string) (bool, error)
	DeployNode(id string) error
	// SSH免密互通配置
	ConfigureSSHSettings(id string) error
	ConfigureSSHPasswdless() error
	// 容器运行时相关方法
	InstallContainerRuntime(id string, runtimeType string, version string) error
	ConfigureContainerRuntime(id string, config ContainerRuntimeConfig) error
	StartContainerRuntime(id string, runtimeType string) error
	StopContainerRuntime(id string, runtimeType string) error
	RemoveContainerRuntime(id string, runtimeType string) error
	EnableContainerRuntime(id string, runtimeType string) error
	DisableContainerRuntime(id string, runtimeType string) error
	CheckContainerRuntimeStatus(id string, runtimeType string) (string, error)
	// 批量容器运行时操作方法
	BatchInstallContainerRuntime(nodeIds []string, runtimeType string, version string) (string, error)
	BatchConfigureContainerRuntime(nodeIds []string, config ContainerRuntimeConfig) (string, error)
	BatchStartContainerRuntime(nodeIds []string, runtimeType string) (string, error)
	BatchStopContainerRuntime(nodeIds []string, runtimeType string) (string, error)
	BatchRemoveContainerRuntime(nodeIds []string, runtimeType string) (string, error)
	BatchEnableContainerRuntime(nodeIds []string, runtimeType string) (string, error)
	BatchDisableContainerRuntime(nodeIds []string, runtimeType string) (string, error)
	BatchCheckContainerRuntimeStatus(nodeIds []string, runtimeType string) (map[string]string, error)
	// 日志相关方法
	GetLogs() ([]log.LogEntry, error)
	GetLogsByNode(nodeID string) ([]log.LogEntry, error)
	ClearLogs() error
	CreateLog(log log.LogEntry) error
	// Kubernetes组件安装
	InstallKubernetesComponents(id string, kubeadmVersion string) error
	// 设置脚本管理器
	SetScriptManager(scriptManager interface{}) error
}

// SSHConfig SSH连接配置
type SSHConfig struct {
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
}

// NodeStatus 节点状态常量
const (
	NodeStatusOnline    = "online"
	NodeStatusOffline   = "offline"
	NodeStatusReady     = "ready"
	NodeStatusDeploying = "deploying"
	NodeStatusError     = "error"
)

// NodeType 节点类型常量
const (
	NodeTypeMaster = "master"
	NodeTypeWorker = "worker"
)
