package kubeadm

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PreflightCheckResult 定义预检检查结果
type PreflightCheckResult struct {
	CheckName      string `json:"checkName"`
	Status         string `json:"status"` // "pass" or "fail"
	Message        string `json:"message"`
	Recommendation string `json:"recommendation,omitempty"`
}

// PreflightChecks 执行所有系统预检检查
func PreflightChecks() []PreflightCheckResult {
	results := []PreflightCheckResult{}

	// 检查CPU核心数
	results = append(results, checkCPU())

	// 检查内存
	results = append(results, checkMemory())

	// 检查内核版本
	results = append(results, checkKernelVersion())

	// 检查交换分区
	results = append(results, checkSwap())

	// 检查主机名唯一性
	results = append(results, checkHostname())

	// 检查MAC地址唯一性
	results = append(results, checkMACAddress())

	// 检查product_uuid唯一性
	results = append(results, checkProductUUID())

	return results
}

// checkCPU 检查CPU核心数
func checkCPU() PreflightCheckResult {
	cmd := exec.Command("nproc")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return PreflightCheckResult{
			CheckName:      "CPU Cores",
			Status:         "fail",
			Message:        "Failed to check CPU cores: " + err.Error(),
			Recommendation: "Ensure 'nproc' command is available",
		}
	}

	cores, err := strconv.Atoi(strings.TrimSpace(out.String()))
	if err != nil {
		return PreflightCheckResult{
			CheckName:      "CPU Cores",
			Status:         "fail",
			Message:        "Failed to parse CPU cores: " + err.Error(),
			Recommendation: "Check system configuration",
		}
	}

	if cores < 2 {
		return PreflightCheckResult{
			CheckName:      "CPU Cores",
			Status:         "fail",
			Message:        fmt.Sprintf("Found %d CPU cores, recommended: 2+", cores),
			Recommendation: "Add more CPU cores to the system",
		}
	}

	return PreflightCheckResult{
		CheckName: "CPU Cores",
		Status:    "pass",
		Message:   fmt.Sprintf("Found %d CPU cores", cores),
	}
}

// checkMemory 检查内存大小
func checkMemory() PreflightCheckResult {
	cmd := exec.Command("cat", "/proc/meminfo")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return PreflightCheckResult{
			CheckName:      "Memory",
			Status:         "fail",
			Message:        "Failed to check memory: " + err.Error(),
			Recommendation: "Ensure '/proc/meminfo' is accessible",
		}
	}

	memInfo := out.String()
	memTotalLine := ""
	for _, line := range strings.Split(memInfo, "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			memTotalLine = line
			break
		}
	}

	if memTotalLine == "" {
		return PreflightCheckResult{
			CheckName:      "Memory",
			Status:         "fail",
			Message:        "Failed to find MemTotal in /proc/meminfo",
			Recommendation: "Check system configuration",
		}
	}

	memTotalStr := strings.Fields(memTotalLine)[1]
	memTotalKB, err := strconv.Atoi(memTotalStr)
	if err != nil {
		return PreflightCheckResult{
			CheckName:      "Memory",
			Status:         "fail",
			Message:        "Failed to parse memory: " + err.Error(),
			Recommendation: "Check system configuration",
		}
	}

	// 转换为GB
	memTotalGB := float64(memTotalKB) / (1024 * 1024)

	if memTotalGB < 2.0 {
		return PreflightCheckResult{
			CheckName:      "Memory",
			Status:         "fail",
			Message:        fmt.Sprintf("Found %.1f GB memory, recommended: 2+ GB", memTotalGB),
			Recommendation: "Add more memory to the system",
		}
	}

	return PreflightCheckResult{
		CheckName: "Memory",
		Status:    "pass",
		Message:   fmt.Sprintf("Found %.1f GB memory", memTotalGB),
	}
}

// checkKernelVersion 检查内核版本
func checkKernelVersion() PreflightCheckResult {
	cmd := exec.Command("uname", "-r")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return PreflightCheckResult{
			CheckName:      "Kernel Version",
			Status:         "fail",
			Message:        "Failed to check kernel version: " + err.Error(),
			Recommendation: "Ensure 'uname' command is available",
		}
	}

	kernelVersion := strings.TrimSpace(out.String())

	// 解析内核版本，检查主版本号和次版本号
	// 示例：5.4.0-100-generic -> 主版本5，次版本4
	parts := strings.Split(kernelVersion, ".")
	if len(parts) < 2 {
		return PreflightCheckResult{
			CheckName:      "Kernel Version",
			Status:         "fail",
			Message:        fmt.Sprintf("Invalid kernel version format: %s", kernelVersion),
			Recommendation: "Check system configuration",
		}
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return PreflightCheckResult{
			CheckName:      "Kernel Version",
			Status:         "fail",
			Message:        fmt.Sprintf("Failed to parse kernel major version: %s", parts[0]),
			Recommendation: "Check system configuration",
		}
	}

	minor, err := strconv.Atoi(strings.Split(parts[1], "-")[0])
	if err != nil {
		return PreflightCheckResult{
			CheckName:      "Kernel Version",
			Status:         "fail",
			Message:        fmt.Sprintf("Failed to parse kernel minor version: %s", parts[1]),
			Recommendation: "Check system configuration",
		}
	}

	// Kubernetes 1.24+ 要求内核版本至少为 5.4
	if major < 5 || (major == 5 && minor < 4) {
		return PreflightCheckResult{
			CheckName:      "Kernel Version",
			Status:         "fail",
			Message:        fmt.Sprintf("Kernel version %s is too old, recommended: 5.4+", kernelVersion),
			Recommendation: "Upgrade kernel to version 5.4 or higher",
		}
	}

	return PreflightCheckResult{
		CheckName: "Kernel Version",
		Status:    "pass",
		Message:   fmt.Sprintf("Kernel version %s is compatible", kernelVersion),
	}
}

// checkSwap 检查交换分区
func checkSwap() PreflightCheckResult {
	cmd := exec.Command("swapon", "--show")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		// 如果命令失败，可能是因为交换分区未启用，这是符合要求的
		return PreflightCheckResult{
			CheckName: "Swap",
			Status:    "pass",
			Message:   "Swap is not enabled",
		}
	}

	swapOutput := strings.TrimSpace(out.String())
	if swapOutput != "" {
		return PreflightCheckResult{
			CheckName:      "Swap",
			Status:         "fail",
			Message:        "Swap is enabled",
			Recommendation: "Disable swap with 'swapoff -a' and update /etc/fstab to keep it disabled after reboot",
		}
	}

	return PreflightCheckResult{
		CheckName: "Swap",
		Status:    "pass",
		Message:   "Swap is not enabled",
	}
}

// checkHostname 检查主机名
func checkHostname() PreflightCheckResult {
	cmd := exec.Command("hostname")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return PreflightCheckResult{
			CheckName:      "Hostname",
			Status:         "fail",
			Message:        "Failed to check hostname: " + err.Error(),
			Recommendation: "Ensure 'hostname' command is available",
		}
	}

	hostname := strings.TrimSpace(out.String())
	if hostname == "" {
		return PreflightCheckResult{
			CheckName:      "Hostname",
			Status:         "fail",
			Message:        "Hostname is empty",
			Recommendation: "Set a valid hostname using 'hostnamectl set-hostname <hostname>'",
		}
	}

	return PreflightCheckResult{
		CheckName: "Hostname",
		Status:    "pass",
		Message:   fmt.Sprintf("Hostname: %s", hostname),
	}
}

// checkMACAddress 检查MAC地址
func checkMACAddress() PreflightCheckResult {
	cmd := exec.Command("ip", "link")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return PreflightCheckResult{
			CheckName:      "MAC Address",
			Status:         "fail",
			Message:        "Failed to check MAC address: " + err.Error(),
			Recommendation: "Ensure 'ip' command is available",
		}
	}

	macAddresses := []string{}
	output := out.String()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "link/ether") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				mac := fields[1]
				// 跳过环回接口
				if mac != "00:00:00:00:00:00" {
					macAddresses = append(macAddresses, mac)
				}
			}
		}
	}

	if len(macAddresses) == 0 {
		return PreflightCheckResult{
			CheckName:      "MAC Address",
			Status:         "fail",
			Message:        "No valid MAC address found",
			Recommendation: "Check network interface configuration",
		}
	}

	return PreflightCheckResult{
		CheckName: "MAC Address",
		Status:    "pass",
		Message:   fmt.Sprintf("Found %d network interfaces with unique MAC addresses", len(macAddresses)),
	}
}

// checkProductUUID 检查product_uuid
func checkProductUUID() PreflightCheckResult {
	cmd := exec.Command("cat", "/sys/class/dmi/id/product_uuid")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return PreflightCheckResult{
			CheckName:      "Product UUID",
			Status:         "fail",
			Message:        "Failed to check product UUID: " + err.Error(),
			Recommendation: "Ensure /sys/class/dmi/id/product_uuid is accessible",
		}
	}

	productUUID := strings.TrimSpace(out.String())
	if productUUID == "" {
		return PreflightCheckResult{
			CheckName:      "Product UUID",
			Status:         "fail",
			Message:        "Product UUID is empty",
			Recommendation: "Check system configuration",
		}
	}

	return PreflightCheckResult{
		CheckName: "Product UUID",
		Status:    "pass",
		Message:   fmt.Sprintf("Product UUID: %s", productUUID),
	}
}
