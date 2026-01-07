package kubeadm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PackageSource 包源配置
type PackageSource struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Default bool   `json:"default"`
}

// PackageInfo 包信息
type PackageInfo struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Arch      string    `json:"arch"`
	Distro    string    `json:"distro"`
	FilePath  string    `json:"filePath"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

// 包源列表
var PackageSources = []PackageSource{
	{
		Name:    "官方源",
		URL:     "https://dl.k8s.io",
		Default: true,
	},
	{
		Name:    "阿里云",
		URL:     "https://mirrors.aliyun.com/kubernetes",
		Default: false,
	},
	{
		Name:    "华为源",
		URL:     "https://github.com/hwclouds/kubernetes/releases/download",
		Default: false,
	},
	{
		Name:    "案例源",
		URL:     "https://example.com/kubernetes",
		Default: false,
	},
}

// GetDefaultSource 获取默认包源
func GetDefaultSource() PackageSource {
	for _, source := range PackageSources {
		if source.Default {
			return source
		}
	}
	return PackageSources[0]
}

// GetPackagePath 获取包的本地存储路径
func GetPackagePath(packageName, version, arch, distro string) string {
	// 创建packages目录（如果不存在）
	packageDir := "packages"
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		fmt.Printf("Failed to create package directory: %v\n", err)
		return ""
	}

	// 生成文件名
	fileName := fmt.Sprintf("%s-%s-%s-%s", packageName, version, arch, distro)
	return filepath.Join(packageDir, fileName)
}

// ListLocalPackages 列出本地已下载的包
func ListLocalPackages() ([]PackageInfo, error) {
	packageDir := "packages"

	// 检查目录是否存在
	if _, err := os.Stat(packageDir); os.IsNotExist(err) {
		return []PackageInfo{}, nil
	}

	// 读取目录内容
	files, err := os.ReadDir(packageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read package directory: %v", err)
	}

	var packages []PackageInfo

	// 解析每个文件
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// 获取文件信息
		fileInfo, err := file.Info()
		if err != nil {
			continue
		}

		// 解析文件名
		name := file.Name()
		parts := strings.Split(name, "-")
		if len(parts) < 4 {
			continue
		}

		packageName := parts[0]
		version := parts[1]
		arch := parts[2]
		distro := parts[3]

		packages = append(packages, PackageInfo{
			Name:      packageName,
			Version:   version,
			Arch:      arch,
			Distro:    distro,
			FilePath:  filepath.Join(packageDir, name),
			Size:      fileInfo.Size(),
			CreatedAt: fileInfo.ModTime(),
		})
	}

	return packages, nil
}

// CheckPackageExists 检查包是否已存在
func CheckPackageExists(packageName, version, arch, distro string) bool {
	path := GetPackagePath(packageName, version, arch, distro)
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// DeletePackage 删除本地包
func DeletePackage(packageName, version, arch, distro string) error {
	path := GetPackagePath(packageName, version, arch, distro)
	return os.Remove(path)
}

// UpdatePackageSource 更新包源
func UpdatePackageSource(index int, source PackageSource) error {
	if index < 0 || index >= len(PackageSources) {
		return fmt.Errorf("invalid source index: %d", index)
	}
	PackageSources[index] = source
	return nil
}

// AddPackageSource 添加新包源
func AddPackageSource(source PackageSource) {
	PackageSources = append(PackageSources, source)
}

// DeletePackageSource 删除包源
func DeletePackageSource(index int) error {
	if index < 0 || index >= len(PackageSources) {
		return fmt.Errorf("invalid source index: %d", index)
	}
	PackageSources = append(PackageSources[:index], PackageSources[index+1:]...)
	return nil
}
