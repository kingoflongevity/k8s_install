package node

import (
	"k8s-installer/log"
	"time"
)

// Node 定义节点信息
type Node struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	IP         string    `json:"ip"`
	Port       int       `json:"port"`
	Username   string    `json:"username"`
	Password   string    `json:"password,omitempty"`
	PrivateKey string    `json:"privateKey,omitempty"`
	NodeType   string    `json:"nodeType"` // master 或 worker
	Status     string    `json:"status"`   // online, offline, ready, deploying
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
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
	// Docker相关方法
	InstallDocker(id string, version string) error
	ConfigureDocker(id string, config DockerConfig) error
	StartDocker(id string) error
	StopDocker(id string) error
	RemoveDocker(id string) error
	EnableDocker(id string) error
	DisableDocker(id string) error
	CheckDockerStatus(id string) (string, error)
	// 批量Docker操作方法
	BatchInstallDocker(nodeIds []string, version string) (string, error)
	BatchConfigureDocker(nodeIds []string, config DockerConfig) (string, error)
	BatchStartDocker(nodeIds []string) (string, error)
	BatchStopDocker(nodeIds []string) (string, error)
	BatchRemoveDocker(nodeIds []string) (string, error)
	BatchEnableDocker(nodeIds []string) (string, error)
	BatchDisableDocker(nodeIds []string) (string, error)
	BatchCheckDockerStatus(nodeIds []string) (map[string]string, error)
	// 日志相关方法
	GetLogs() ([]log.LogEntry, error)
	GetLogsByNode(nodeID string) ([]log.LogEntry, error)
	ClearLogs() error
	CreateLog(log log.LogEntry) error
}

// DockerConfig Docker配置结构体
type DockerConfig struct {
	RegistryMirrors []string `json:"registryMirrors"` // 镜像加速地址
	DataRoot        string   `json:"dataRoot"`        // 数据存储目录
	StorageDriver   string   `json:"storageDriver"`   // 存储驱动
	LogDriver       string   `json:"logDriver"`       // 日志驱动
	LogMaxSize      string   `json:"logMaxSize"`      // 日志文件大小限制
	LogMaxFile      int      `json:"logMaxFile"`      // 日志文件数量限制
	CgroupDriver    string   `json:"cgroupDriver"`    // Cgroup驱动
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
