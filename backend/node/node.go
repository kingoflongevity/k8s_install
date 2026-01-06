package node

import (
	"time"
)

// Node 定义节点信息
type Node struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	Username  string    `json:"username"`
	Password  string    `json:"password,omitempty"`
	PrivateKey string   `json:"privateKey,omitempty"`
	NodeType  string    `json:"nodeType"` // master 或 worker
	Status    string    `json:"status"`   // online, offline, ready, deploying
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
}

// SSHConfig SSH连接配置
type SSHConfig struct {
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"`
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
