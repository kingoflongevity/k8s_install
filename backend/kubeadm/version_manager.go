package kubeadm

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// VersionManager 版本管理器，负责定期同步Kubernetes版本

type VersionManager struct {
	mu                sync.RWMutex
	availableVersions []string
	syncInterval      time.Duration
	running           bool
	stopChan          chan struct{}
}

// NewVersionManager 创建新的版本管理器
func NewVersionManager(syncInterval time.Duration) *VersionManager {
	vm := &VersionManager{
		availableVersions: []string{},
		syncInterval:      syncInterval,
		running:           false,
		stopChan:          make(chan struct{}),
	}
	return vm
}

// Start 启动版本同步服务
func (vm *VersionManager) Start() {
	if vm.running {
		return
	}
	vm.running = true

	// 立即执行一次同步
	vm.SyncVersions()

	// 启动定时同步
	go func() {
		ticker := time.NewTicker(vm.syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				vm.SyncVersions()
			case <-vm.stopChan:
				vm.running = false
				return
			}
		}
	}()
}

// Stop 停止版本同步服务
func (vm *VersionManager) Stop() {
	if vm.running {
		vm.stopChan <- struct{}{}
	}
}

// GetAvailableVersions 获取可用的Kubernetes版本列表
func (vm *VersionManager) GetAvailableVersions() []string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	// 如果没有同步到版本，返回默认版本列表
	if len(vm.availableVersions) == 0 {
		return []string{
			"v1.30.0",
			"v1.29.4",
			"v1.29.3",
			"v1.29.2",
			"v1.29.1",
			"v1.29.0",
			"v1.28.8",
			"v1.28.7",
			"v1.28.6",
			"v1.28.5",
			"v1.28.4",
			"v1.28.3",
			"v1.28.2",
			"v1.28.1",
			"v1.28.0",
			"v1.27.12",
			"v1.27.11",
			"v1.27.10",
			"v1.27.9",
			"v1.27.8",
			"v1.27.7",
			"v1.27.6",
			"v1.27.5",
			"v1.27.4",
			"v1.27.3",
			"v1.27.2",
			"v1.27.1",
			"v1.27.0",
		}
	}

	return vm.availableVersions
}

// SyncVersions 同步Kubernetes版本列表
func (vm *VersionManager) SyncVersions() {
	fmt.Println("开始同步Kubernetes版本列表...")

	// 从阿里云镜像源获取可用版本
	versions := vm.fetchVersionsFromAliyun()

	// 从官方源获取可用版本作为备份
	if len(versions) == 0 {
		versions = vm.fetchVersionsFromOfficial()
	}

	// 处理版本列表，去重、排序
	processedVersions := vm.processVersions(versions)

	// 更新可用版本
	vm.mu.Lock()
	vm.availableVersions = processedVersions
	vm.mu.Unlock()

	fmt.Printf("版本同步完成，共获取到 %d 个可用版本\n", len(processedVersions))
	fmt.Printf("最新可用版本: %s\n", processedVersions[0])
}

// fetchVersionsFromAliyun 从阿里云镜像源获取可用版本
func (vm *VersionManager) fetchVersionsFromAliyun() []string {
	versions := []string{}

	// 阿里云Ubuntu/Debian源
	aliyunDebURL := "https://mirrors.aliyun.com/kubernetes/apt/dists/kubernetes-xenial/main/binary-amd64/Packages"
	debVersions := vm.parseVersionsFromPackagesURL(aliyunDebURL)
	versions = append(versions, debVersions...)

	// 阿里云CentOS/RHEL源
	aliyunRpmURL := "https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64/repodata/primary.xml.gz"
	rpmVersions := vm.parseVersionsFromRepodataURL(aliyunRpmURL)
	versions = append(versions, rpmVersions...)

	return versions
}

// fetchVersionsFromOfficial 从官方源获取可用版本
func (vm *VersionManager) fetchVersionsFromOfficial() []string {
	versions := []string{}

	// 官方稳定版列表
	officialURL := "https://dl.k8s.io/release/stable.txt"
	resp, err := http.Get(officialURL)
	if err != nil {
		fmt.Printf("获取官方稳定版失败: %v\n", err)
		return versions
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取官方稳定版失败: %v\n", err)
		return versions
	}

	stableVersion := strings.TrimSpace(string(body))
	if stableVersion != "" {
		versions = append(versions, stableVersion)
	}

	return versions
}

// parseVersionsFromPackagesURL 从Packages文件中解析版本
func (vm *VersionManager) parseVersionsFromPackagesURL(url string) []string {
	versions := []string{}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("获取Packages文件失败: %v\n", err)
		return versions
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取Packages文件失败: %v\n", err)
		return versions
	}

	content := string(body)
	// 匹配kubelet包的版本
	r := regexp.MustCompile(`Package: kubelet\s+Version: ([0-9]+\.[0-9]+\.[0-9]+)`)
	matches := r.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			versions = append(versions, "v"+match[1])
		}
	}

	return versions
}

// parseVersionsFromRepodataURL 从repodata XML文件中解析版本
func (vm *VersionManager) parseVersionsFromRepodataURL(url string) []string {
	versions := []string{}

	// 简化实现，实际需要解析XML
	// 这里我们返回空，因为解析XML需要额外的依赖
	// 实际生产环境中应该使用合适的XML解析库

	return versions
}

// processVersions 处理版本列表，去重、排序
func (vm *VersionManager) processVersions(versions []string) []string {
	// 去重
	versionMap := make(map[string]bool)
	for _, v := range versions {
		versionMap[v] = true
	}

	// 转换为切片
	uniqueVersions := []string{}
	for v := range versionMap {
		uniqueVersions = append(uniqueVersions, v)
	}

	// 按版本号降序排序
	sort.Slice(uniqueVersions, func(i, j int) bool {
		return vm.compareVersions(uniqueVersions[i], uniqueVersions[j]) > 0
	})

	return uniqueVersions
}

// compareVersions 比较两个版本号，返回1（v1 > v2）, 0（v1 == v2）, -1（v1 < v2）
func (vm *VersionManager) compareVersions(v1, v2 string) int {
	// 移除v前缀
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// 分割版本号
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// 比较每个部分
	for i := 0; i < 3; i++ {
		var num1, num2 int
		fmt.Sscanf(parts1[i], "%d", &num1)
		fmt.Sscanf(parts2[i], "%d", &num2)

		if num1 > num2 {
			return 1
		} else if num1 < num2 {
			return -1
		}
	}

	return 0
}
