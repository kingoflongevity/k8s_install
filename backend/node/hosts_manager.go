package node

import (
	"sync"
)

// HostsManager 主机名映射管理器
// 用于管理节点名称到IP地址的映射关系
// 从数据库中获取节点信息，提供快速查询服务
// 避免依赖本地hosts文件，提高SSH连接的可靠性

type HostsManager struct {
	nodeManager NodeManager
	cache       map[string]string // 缓存：节点名称 -> IP地址
	mutex       sync.RWMutex
}

// NewHostsManager 创建新的主机名映射管理器
func NewHostsManager(nodeManager NodeManager) *HostsManager {
	return &HostsManager{
		nodeManager: nodeManager,
		cache:       make(map[string]string),
	}
}

// GetIPByHostname 根据节点名称获取IP地址
// 如果缓存中存在，直接返回
// 如果缓存中不存在，从数据库中查询并更新缓存
func (hm *HostsManager) GetIPByHostname(hostname string) (string, bool) {
	// 先检查缓存
	hm.mutex.RLock()
	ip, exists := hm.cache[hostname]
	hm.mutex.RUnlock()
	if exists {
		return ip, true
	}

	// 缓存中不存在，从数据库中查询
	allNodes, err := hm.nodeManager.GetNodes()
	if err != nil {
		return "", false
	}

	// 更新缓存并查找目标节点
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	// 遍历所有节点，更新缓存
	for _, node := range allNodes {
		hm.cache[node.Name] = node.IP
	}

	// 再次查找目标节点
	ip, exists = hm.cache[hostname]
	return ip, exists
}

// RefreshCache 刷新缓存
func (hm *HostsManager) RefreshCache() error {
	allNodes, err := hm.nodeManager.GetNodes()
	if err != nil {
		return err
	}

	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	// 清空缓存
	hm.cache = make(map[string]string)
	
	// 重新填充缓存
	for _, node := range allNodes {
		hm.cache[node.Name] = node.IP
	}

	return nil
}

// ClearCache 清空缓存
func (hm *HostsManager) ClearCache() {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	hm.cache = make(map[string]string)
}
