package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"k8s-installer/kubeadm"
	"k8s-installer/log"
	"k8s-installer/node"
	"k8s-installer/script"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// maskPassword 掩码密码，只显示前2个字符和后2个字符
func maskPassword(password string) string {
	if password == "" {
		return "<空>"
	}
	if len(password) <= 4 {
		return strings.Repeat("*", len(password))
	}
	return password[:2] + strings.Repeat("*", len(password)-4) + password[len(password)-2:]
}

// maskPrivateKey 掩码私钥，只显示前20个字符和后20个字符
func maskPrivateKey(privateKey string) string {
	if privateKey == "" {
		return "<空>"
	}
	if len(privateKey) <= 40 {
		return strings.Repeat("*", len(privateKey))
	}
	return privateKey[:20] + "...(省略)..." + privateKey[len(privateKey)-20:]
}

func main() {
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// 初始化版本管理器，每3小时同步一次
	versionManager := kubeadm.NewVersionManager(3 * time.Hour)
	// 启动版本同步服务
	versionManager.Start()

	// 初始化节点管理器（SQLite实现，使用纯Go驱动，支持持久化存储，不需要CGO）
	nodeManager, err := node.NewSqliteNodeManager("k8s_installer.db")
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize SQLite node manager: %v", err))
	}

	// 获取日志管理器 - 广播回调由SSE端点动态设置

	// 初始化脚本管理器
	scriptManager, err := script.NewScriptManager("./scripts")
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize script manager: %v", err))
	}

	// 设置数据库连接，确保脚本与数据库同步
	// 直接调用GetDB()方法，因为nodeManager是*node.SqliteNodeManager类型，它有GetDB()方法
	scriptManager.SetDB(nodeManager.GetDB())

	// 将脚本管理器传递给节点管理器
	if err := nodeManager.SetScriptManager(scriptManager); err != nil {
		panic(fmt.Sprintf("Failed to set script manager for node manager: %v", err))
	}

	// API routes// 健康检查路由
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Kubeadm routes
	r.GET("/kubeadm/version", func(c *gin.Context) {
		masterNodeID := c.Query("masterNodeId")
		if masterNodeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "masterNodeId is required",
			})
			return
		}

		// 获取master节点信息
		masterNode, err := nodeManager.GetNode(masterNodeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to get master node: %v", err),
			})
			return
		}

		// 创建SSH配置，首先使用IP地址连接（确保在任何hosts文件更新之前都能连接）
		sshConfig := kubeadm.SSHConfig{
			Host:       masterNode.IP,
			Port:       masterNode.Port,
			Username:   masterNode.Username,
			Password:   masterNode.Password,
			PrivateKey: masterNode.PrivateKey,
		}

		version, err := kubeadm.CheckKubeadmVersion(sshConfig)
		if err != nil {
			// 记录详细错误日志
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"version": version,
		})
	})

	// Kubeadm 系统预检路由
	r.GET("/kubeadm/preflight", func(c *gin.Context) {
		results := kubeadm.PreflightChecks()
		c.JSON(http.StatusOK, gin.H{
			"checks": results,
		})
	})

	// Kubeadm 包管理路由
	r.GET("/kubeadm/packages", func(c *gin.Context) {
		// 从版本管理器获取可用的Kubernetes版本列表
		versions := versionManager.GetAvailableVersions()
		c.JSON(http.StatusOK, gin.H{
			"versions": versions,
		})
	})

	// 获取包源列表
	r.GET("/kubeadm/sources", func(c *gin.Context) {
		sources := kubeadm.PackageSources
		c.JSON(http.StatusOK, gin.H{
			"sources": sources,
		})
	})

	// 更新包源
	r.PUT("/kubeadm/sources/:index", func(c *gin.Context) {
		indexStr := c.Param("index")
		var index int
		if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid index format",
			})
			return
		}

		var source kubeadm.PackageSource
		if err := c.ShouldBindJSON(&source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 如果设置了default为true，需要将其他源的default设置为false
		if source.Default {
			for i := range kubeadm.PackageSources {
				kubeadm.PackageSources[i].Default = false
			}
		}

		if err := kubeadm.UpdatePackageSource(index, source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "updated",
			"sources": kubeadm.PackageSources,
		})
	})

	// 添加新包源
	r.POST("/kubeadm/sources", func(c *gin.Context) {
		var source kubeadm.PackageSource
		if err := c.ShouldBindJSON(&source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 如果设置了default为true，需要将其他源的default设置为false
		if source.Default {
			for i := range kubeadm.PackageSources {
				kubeadm.PackageSources[i].Default = false
			}
		}

		kubeadm.AddPackageSource(source)
		c.JSON(http.StatusOK, gin.H{
			"status":  "added",
			"sources": kubeadm.PackageSources,
		})
	})

	// 删除包源
	r.DELETE("/kubeadm/sources/:index", func(c *gin.Context) {
		indexStr := c.Param("index")
		var index int
		if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid index format",
			})
			return
		}

		if err := kubeadm.DeletePackageSource(index); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "deleted",
			"sources": kubeadm.PackageSources,
		})
	})

	// 获取已下载的包列表
	r.GET("/kubeadm/packages/local", func(c *gin.Context) {
		packages, err := kubeadm.ListLocalPackages()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"packages": packages,
		})
	})

	// 删除本地包
	r.DELETE("/kubeadm/packages/local", func(c *gin.Context) {
		var req struct {
			Name    string `json:"name" binding:"required"`
			Version string `json:"version" binding:"required"`
			Arch    string `json:"arch" binding:"required"`
			Distro  string `json:"distro" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		if err := kubeadm.DeletePackage(req.Name, req.Version, req.Arch, req.Distro); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "deleted",
		})
	})

	r.POST("/kubeadm/packages/download", func(c *gin.Context) {
		var req struct {
			Version   string `json:"version" binding:"required"`
			Arch      string `json:"arch" binding:"required"`
			Distro    string `json:"distro" binding:"required"`
			SourceURL string `json:"sourceURL"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 下载指定版本的Kubeadm包
		log := func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		}
		packagePath, err := kubeadm.DownloadKubeadmPackage(req.Version, req.Arch, req.Distro, req.SourceURL, log)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"packagePath": packagePath,
			"version":     req.Version,
		})
	})

	r.POST("/kubeadm/packages/deploy", func(c *gin.Context) {
		var req struct {
			PackagePath string `json:"packagePath" binding:"required"`
			NodeIP      string `json:"nodeIP" binding:"required"`
			Username    string `json:"username" binding:"required"`
			Password    string `json:"password"`
			Port        int    `json:"port"`
			PrivateKey  string `json:"privateKey"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 部署Kubeadm包到远程节点
		log := func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		}
		err := kubeadm.DeployKubeadmPackage(req.PackagePath, req.NodeIP, req.Username, req.Password, req.Port, req.PrivateKey, log)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "deployed",
			"nodeIP": req.NodeIP,
		})
	})

	r.POST("/kubeadm/init", func(c *gin.Context) {
		var req struct {
			Config    kubeadm.KubeadmConfig `json:"config" binding:"required"`
			SkipSteps []string              `json:"skipSteps" binding:"omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 获取所有节点，然后选择第一个主节点
		allNodes, err := nodeManager.GetNodes()
		if err != nil {
			errorLog := fmt.Sprintf("调试信息: 获取所有节点失败: %v", err)
			fmt.Println(errorLog)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to get nodes: %v", err),
			})
			return
		}

		// 过滤出主节点
		var masterNode *node.Node
		for _, n := range allNodes {
			if n.NodeType == "master" || n.NodeType == "Master" {
				masterNode = &n
				break
			}
		}

		if masterNode == nil {
			errorLog := "调试信息: 没有找到主节点，请先添加主节点并设置为主节点类型"
			fmt.Println(errorLog)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "no master node found",
			})
			return
		}

		// 记录找到的主节点信息
		debugLog := fmt.Sprintf("调试信息: 成功找到主节点: ID=%s, Name=%s, IP=%s", masterNode.ID, masterNode.Name, masterNode.IP)
		fmt.Println(debugLog)

		// 添加详细的节点信息调试
		nodeInfoLog := fmt.Sprintf("调试信息: 成功获取节点信息:\nID: %s\nName: %s\nIP: '%s' (长度: %d)\nPort: %d\nUsername: '%s'\nPassword: %s\nPrivateKey: %s\nNodeType: %s\nStatus: %s\nOS: %s",
			masterNode.ID, masterNode.Name, masterNode.IP, len(masterNode.IP),
			masterNode.Port, masterNode.Username, maskPassword(masterNode.Password),
			maskPrivateKey(masterNode.PrivateKey), masterNode.NodeType, masterNode.Status, masterNode.OS)
		fmt.Println(nodeInfoLog)
		// 记录节点信息日志
		nodeManager.CreateLog(log.LogEntry{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			NodeID:    masterNode.ID,
			NodeName:  masterNode.Name,
			Operation: "Debug",
			Command:   "节点信息",
			Output:    nodeInfoLog,
			Status:    "success",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})

		// 验证节点信息是否完整
		if masterNode.IP == "" {
			errorLog := "错误: 节点IP地址为空"
			fmt.Println(errorLog)
			// 记录错误日志
			nodeManager.CreateLog(log.LogEntry{
				ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
				NodeID:    masterNode.ID,
				NodeName:  masterNode.Name,
				Operation: "Debug",
				Command:   "验证节点信息",
				Output:    errorLog,
				Status:    "failed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "master node IP address is empty",
			})
			return
		}

		if masterNode.Port == 0 {
			warningLog := "警告: 节点端口为0，设置为默认值22"
			fmt.Println(warningLog)
			// 记录警告日志
			nodeManager.CreateLog(log.LogEntry{
				ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
				NodeID:    masterNode.ID,
				NodeName:  masterNode.Name,
				Operation: "Debug",
				Command:   "验证节点信息",
				Output:    warningLog,
				Status:    "warning",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
			masterNode.Port = 22
		}

		if masterNode.Username == "" {
			errorLog := "错误: 节点用户名为空"
			fmt.Println(errorLog)
			// 记录错误日志
			nodeManager.CreateLog(log.LogEntry{
				ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
				NodeID:    masterNode.ID,
				NodeName:  masterNode.Name,
				Operation: "Debug",
				Command:   "验证节点信息",
				Output:    errorLog,
				Status:    "failed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "master node username is empty",
			})
			return
		}

		if masterNode.Password == "" && masterNode.PrivateKey == "" {
			errorLog := "错误: 节点既没有密码也没有私钥"
			fmt.Println(errorLog)
			// 记录错误日志
			nodeManager.CreateLog(log.LogEntry{
				ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
				NodeID:    masterNode.ID,
				NodeName:  masterNode.Name,
				Operation: "Debug",
				Command:   "验证节点信息",
				Output:    errorLog,
				Status:    "failed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "master node has neither password nor private key",
			})
			return
		}

		// 创建SSH配置，首先使用IP地址连接（确保在任何hosts文件更新之前都能连接）
		sshConfig := kubeadm.SSHConfig{
			Host:       masterNode.IP,
			Port:       masterNode.Port,
			Username:   masterNode.Username,
			Password:   masterNode.Password,
			PrivateKey: masterNode.PrivateKey,
		}

		// 添加SSH配置调试信息
		sshConfigLog := fmt.Sprintf("调试信息: 最终的SSH配置:\nHost: %s\nPort: %d\nUsername: %s\nPassword: %s\nPrivateKey: %s",
			sshConfig.Host, sshConfig.Port, sshConfig.Username,
			maskPassword(sshConfig.Password), maskPrivateKey(sshConfig.PrivateKey))
		fmt.Println(sshConfigLog)
		// 记录SSH配置日志
		nodeManager.CreateLog(log.LogEntry{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			NodeID:    masterNode.ID,
			NodeName:  masterNode.Name,
			Operation: "Debug",
			Command:   "SSH配置",
			Output:    sshConfigLog,
			Status:    "success",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})

		// 记录初始化开始日志
		initLog := log.LogEntry{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			NodeID:    masterNode.ID,
			NodeName:  masterNode.Name,
			Operation: "InitMaster",
			Command:   "初始化Master节点",
			Output:    "开始初始化Master节点...",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		nodeManager.CreateLog(initLog)

		fmt.Printf("开始初始化master节点: %s\n", masterNode.Name)
		fmt.Printf("跳过的步骤: %s\n", strings.Join(req.SkipSteps, ", "))

		result, err := kubeadm.InitMaster(sshConfig, req.Config, req.SkipSteps)
		if err != nil {
			// 记录初始化失败日志
			initLog.Output = fmt.Sprintf("初始化失败: %v\n输出: %s", err, result)
			initLog.Status = "failed"
			initLog.UpdatedAt = time.Now()
			nodeManager.CreateLog(initLog)

			fmt.Printf("初始化master节点失败: %s\n错误: %v\n输出: %s\n", masterNode.Name, err, result)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 记录初始化成功日志
		initLog.Output = fmt.Sprintf("初始化成功\n输出: %s", result)
		initLog.Status = "success"
		initLog.UpdatedAt = time.Now()
		nodeManager.CreateLog(initLog)

		fmt.Printf("初始化master节点成功: %s\n输出: %s\n", masterNode.Name, result)

		// 从输出中提取join命令并存储到数据库中
		var joinCommand string
		lines := strings.Split(result, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "kubeadm join") {
				// 开始构建join命令，处理多行情况
				var fullCommand []string
				j := i
				for j < len(lines) {
					currentLine := strings.TrimSpace(lines[j])
					// 检查是否以反斜杠结尾（表示命令换行）
					if strings.HasSuffix(currentLine, "\\") {
						// 移除反斜杠并添加到命令中
						fullCommand = append(fullCommand, strings.TrimSuffix(currentLine, "\\"))
						j++
					} else {
						// 这是命令的最后一行，添加到命令中并停止
						fullCommand = append(fullCommand, currentLine)
						break
					}
				}
				// 合并所有行到一个完整的命令中
				joinCommand = strings.TrimSpace(strings.Join(fullCommand, " "))
				break
			}
		}

		// 如果提取到join命令，将其存储到数据库中
		if joinCommand != "" {
			fmt.Printf("提取到join命令: %s\n", joinCommand)
			// 更新master节点的JoinCommand字段
			masterNode.JoinCommand = joinCommand
			_, err := nodeManager.UpdateNode(masterNode.ID, *masterNode)
			if err != nil {
				fmt.Printf("存储join命令到数据库失败: %v\n", err)
			} else {
				fmt.Println("join命令存储到数据库成功")
			}
		} else {
			fmt.Println("未从输出中提取到join命令")
			// 尝试直接获取join命令
			sshConfig := kubeadm.SSHConfig{
				Host:       masterNode.IP,
				Port:       masterNode.Port,
				Username:   masterNode.Username,
				Password:   masterNode.Password,
				PrivateKey: masterNode.PrivateKey,
			}
			joinCommand, err := kubeadm.GetJoinCommand(sshConfig)
			if err == nil && joinCommand != "" {
				fmt.Printf("直接获取到join命令: %s\n", joinCommand)
				// 更新master节点的JoinCommand字段
				masterNode.JoinCommand = joinCommand
				_, err := nodeManager.UpdateNode(masterNode.ID, *masterNode)
				if err != nil {
					fmt.Printf("存储join命令到数据库失败: %v\n", err)
				} else {
					fmt.Println("join命令存储到数据库成功")
				}
			} else {
				fmt.Printf("直接获取join命令失败: %v\n", err)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"result":      result,
			"joinCommand": joinCommand,
		})
	})

	// 拉取Kubernetes镜像到本地
	r.POST("/kubeadm/images/pull", func(c *gin.Context) {
		var req struct {
			MasterNodeID string `json:"masterNodeId" binding:"required"`
			Version      string `json:"version" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 获取master节点信息
		masterNode, err := nodeManager.GetNode(req.MasterNodeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to get master node: %v", err),
			})
			return
		}

		// 创建SSH配置，首先使用IP地址连接（确保在任何hosts文件更新之前都能连接）
		sshConfig := kubeadm.SSHConfig{
			Host:       masterNode.IP,
			Port:       masterNode.Port,
			Username:   masterNode.Username,
			Password:   masterNode.Password,
			PrivateKey: masterNode.PrivateKey,
		}

		// 记录镜像拉取开始日志
		pullLog := log.LogEntry{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			NodeID:    masterNode.ID,
			NodeName:  masterNode.Name,
			Operation: "PullKubernetesImages",
			Command:   fmt.Sprintf("拉取Kubernetes镜像，版本: %s", req.Version),
			Output:    "开始拉取Kubernetes镜像...",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		nodeManager.CreateLog(pullLog)

		fmt.Printf("开始拉取Kubernetes镜像，版本: %s\n", req.Version)

		result, err := kubeadm.PullKubernetesImages(sshConfig, req.Version)
		if err != nil {
			// 记录镜像拉取失败日志
			pullLog.Output = fmt.Sprintf("拉取失败: %v\n输出: %s", err, result)
			pullLog.Status = "failed"
			pullLog.UpdatedAt = time.Now()
			nodeManager.CreateLog(pullLog)

			fmt.Printf("拉取Kubernetes镜像失败\n版本: %s\n错误: %v\n输出: %s\n", req.Version, err, result)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 记录镜像拉取成功日志
		pullLog.Output = fmt.Sprintf("拉取成功\n输出: %s", result)
		pullLog.Status = "success"
		pullLog.UpdatedAt = time.Now()
		nodeManager.CreateLog(pullLog)

		fmt.Printf("拉取Kubernetes镜像成功\n版本: %s\n输出: %s\n", req.Version, result)

		c.JSON(http.StatusOK, gin.H{
			"result": result,
		})
	})

	r.GET("/kubeadm/join-command", func(c *gin.Context) {
		// 获取所有节点，然后选择第一个主节点
		allNodes, err := nodeManager.GetNodes()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to get nodes: %v", err),
			})
			return
		}

		// 过滤出主节点
		var masterNode *node.Node
		for _, n := range allNodes {
			if n.NodeType == "master" || n.NodeType == "Master" {
				masterNode = &n
				break
			}
		}

		if masterNode == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "no master node found",
			})
			return
		}

		// 首先尝试从节点的JoinCommand字段中获取join命令
		if masterNode.JoinCommand != "" {
			c.JSON(http.StatusOK, gin.H{
				"command": masterNode.JoinCommand,
			})
			return
		}

		// 如果JoinCommand字段为空，则通过SSH连接到master节点获取join命令
		// 创建SSH配置，首先使用IP地址连接（确保在任何hosts文件更新之前都能连接）
		sshConfig := kubeadm.SSHConfig{
			Host:       masterNode.IP,
			Port:       masterNode.Port,
			Username:   masterNode.Username,
			Password:   masterNode.Password,
			PrivateKey: masterNode.PrivateKey,
		}

		cmd, err := kubeadm.GetJoinCommand(sshConfig)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 将获取到的join命令存储到master节点的JoinCommand字段中
		masterNode.JoinCommand = cmd
		_, err = nodeManager.UpdateNode(masterNode.ID, *masterNode)
		if err != nil {
			// 存储失败不影响返回结果，只记录错误
			fmt.Printf("存储join命令到数据库失败: %v\n", err)
		}

		c.JSON(http.StatusOK, gin.H{
			"command": cmd,
		})
	})

	r.POST("/kubeadm/reset", func(c *gin.Context) {
		var req struct {
			MasterNodeID string `json:"masterNodeId" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 获取master节点信息
		masterNode, err := nodeManager.GetNode(req.MasterNodeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to get master node: %v", err),
			})
			return
		}

		// 创建SSH配置
		sshConfig := kubeadm.SSHConfig{
			Host:       masterNode.Name,
			Port:       masterNode.Port,
			Username:   masterNode.Username,
			Password:   masterNode.Password,
			PrivateKey: masterNode.PrivateKey,
		}

		// 记录集群重置开始日志
		resetLog := log.LogEntry{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			NodeID:    masterNode.ID,
			NodeName:  masterNode.Name,
			Operation: "ResetCluster",
			Command:   "重置Kubernetes集群",
			Output:    "开始重置Kubernetes集群...",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		nodeManager.CreateLog(resetLog)

		fmt.Printf("开始重置Kubernetes集群\n")

		result, err := kubeadm.ResetCluster(sshConfig)
		if err != nil {
			// 记录集群重置失败日志
			resetLog.Output = fmt.Sprintf("重置失败: %v\n输出: %s", err, result)
			resetLog.Status = "failed"
			resetLog.UpdatedAt = time.Now()
			nodeManager.CreateLog(resetLog)

			fmt.Printf("重置Kubernetes集群失败\n错误: %v\n输出: %s\n", err, result)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 记录集群重置成功日志
		resetLog.Output = fmt.Sprintf("重置成功\n输出: %s", result)
		resetLog.Status = "success"
		resetLog.UpdatedAt = time.Now()
		nodeManager.CreateLog(resetLog)

		fmt.Printf("重置Kubernetes集群成功\n输出: %s\n", result)

		c.JSON(http.StatusOK, gin.H{
			"result": result,
		})
	})

	r.POST("/kubeadm/join", func(c *gin.Context) {
		var req struct {
			WorkerNodeID         string `json:"workerNodeId" binding:"required"`
			Token                string `json:"token" binding:"required"`
			CACertHash           string `json:"caCertHash" binding:"required"`
			ControlPlaneEndpoint string `json:"controlPlaneEndpoint" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 获取工作节点信息
		workerNode, err := nodeManager.GetNode(req.WorkerNodeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to get worker node: %v", err),
			})
			return
		}

		// 创建SSH配置，首先使用IP地址连接（确保在任何hosts文件更新之前都能连接）
		sshConfig := kubeadm.SSHConfig{
			Host:       workerNode.IP,
			Port:       workerNode.Port,
			Username:   workerNode.Username,
			Password:   workerNode.Password,
			PrivateKey: workerNode.PrivateKey,
		}

		// 记录工作节点加入开始日志
		joinLog := log.LogEntry{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			NodeID:    workerNode.ID,
			NodeName:  workerNode.Name,
			Operation: "JoinWorker",
			Command:   fmt.Sprintf("将工作节点加入集群，控制平面端点: %s", req.ControlPlaneEndpoint),
			Output:    "开始将工作节点加入集群...",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		nodeManager.CreateLog(joinLog)

		fmt.Printf("开始将工作节点加入集群: %s\n", workerNode.Name)

		result, err := kubeadm.JoinWorker(sshConfig, req.Token, req.CACertHash, req.ControlPlaneEndpoint)
		if err != nil {
			// 记录工作节点加入失败日志
			joinLog.Output = fmt.Sprintf("加入失败: %v\n输出: %s", err, result)
			joinLog.Status = "failed"
			joinLog.UpdatedAt = time.Now()
			nodeManager.CreateLog(joinLog)

			fmt.Printf("工作节点加入集群失败: %s\n错误: %v\n输出: %s\n", workerNode.Name, err, result)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 记录工作节点加入成功日志
		joinLog.Output = fmt.Sprintf("加入成功\n输出: %s", result)
		joinLog.Status = "success"
		joinLog.UpdatedAt = time.Now()
		nodeManager.CreateLog(joinLog)

		fmt.Printf("工作节点加入集群成功: %s\n输出: %s\n", workerNode.Name, result)

		c.JSON(http.StatusOK, gin.H{
			"result": result,
		})
	})

	// K8s Deployment routes
	r.POST("/k8s/deploy", func(c *gin.Context) {
		var req struct {
			KubeVersion          string   `json:"kubeVersion" binding:"required"`
			Arch                 string   `json:"arch" binding:"required"`
			Distro               string   `json:"distro" binding:"required"`
			NodeIds              []string `json:"nodeIds" binding:"required"`
			SkipSteps            []string `json:"skipSteps" binding:"omitempty"`
			JoinToken            string   `json:"joinToken" binding:"omitempty"`
			CACertHash           string   `json:"caCertHash" binding:"omitempty"`
			ControlPlaneEndpoint string   `json:"controlPlaneEndpoint" binding:"omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 记录部署开始日志
		deployLog := log.LogEntry{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			NodeID:    "cluster",
			NodeName:  "Kubernetes Cluster",
			Operation: "DeployK8sCluster",
			Command:   fmt.Sprintf("部署Kubernetes集群，版本: %s，架构: %s，发行版: %s", req.KubeVersion, req.Arch, req.Distro),
			Output:    "开始部署Kubernetes集群...",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		nodeManager.CreateLog(deployLog)

		fmt.Printf("开始部署Kubernetes集群\n节点ID列表: %s\n版本: %s\n架构: %s\n发行版: %s\n", strings.Join(req.NodeIds, ", "), req.KubeVersion, req.Arch, req.Distro)

		// 获取所有指定的节点
		var nodes []node.Node
		var nodeNames []string
		for _, id := range req.NodeIds {
			n, err := nodeManager.GetNode(id)
			if err != nil {
				// 记录部署失败日志
				deployLog.Output = fmt.Sprintf("部署失败: 获取节点 %s 失败\n错误: %v\n", id, err)
				deployLog.Status = "failed"
				deployLog.UpdatedAt = time.Now()
				nodeManager.CreateLog(deployLog)

				fmt.Printf("部署失败: 获取节点 %s 失败\n错误: %v\n", id, err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("获取节点 %s 失败: %v", id, err),
				})
				return
			}
			nodes = append(nodes, *n)
			nodeNames = append(nodeNames, n.Name)
		}

		// 更新部署日志，添加节点信息
		deployLog.Output = fmt.Sprintf("节点列表: %s\n开始部署...", strings.Join(nodeNames, ", "))
		deployLog.UpdatedAt = time.Now()
		nodeManager.CreateLog(deployLog)

		fmt.Printf("节点列表: %s\n", strings.Join(nodeNames, ", "))

		// 创建一个上下文，支持取消部署
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 设置join token相关环境变量，供DeployK8sCluster函数使用
		if req.JoinToken != "" && req.CACertHash != "" && req.ControlPlaneEndpoint != "" {
			// 构建完整的join命令
			joinCmd := fmt.Sprintf("kubeadm join %s --token %s --discovery-token-ca-cert-hash %s", req.ControlPlaneEndpoint, req.JoinToken, req.CACertHash)
			os.Setenv("KUBEADM_JOIN_COMMAND", joinCmd)
			fmt.Printf("设置KUBEADM_JOIN_COMMAND环境变量: %s\n", joinCmd)
		} else if req.JoinToken != "" {
			// 如果只提供了部分信息，分别设置环境变量
			os.Setenv("KUBEADM_TOKEN", req.JoinToken)
			os.Setenv("KUBEADM_CA_CERT_HASH", req.CACertHash)
			os.Setenv("KUBEADM_CONTROL_PLANE_ENDPOINT", req.ControlPlaneEndpoint)
			fmt.Printf("设置join token相关环境变量: Token=%s, CACertHash=%s, ControlPlaneEndpoint=%s\n", req.JoinToken, req.CACertHash, req.ControlPlaneEndpoint)
		}

		// 调用DeployK8sCluster函数进行部署，传递scriptManager和skipSteps
		// 实时日志回调函数，支持按节点记录日志
		logCallback := func(logMsg, nodeID, nodeName string) {
			// 确定日志的节点ID和节点名
			logNodeID := nodeID
			logNodeName := nodeName

			// 如果是集群级别的日志，使用原始日志回调中的固定值
			if logNodeID == "cluster" {
				logNodeName = "Kubernetes Cluster"
			}

			// 创建日志条目
			logEntry := log.LogEntry{
				ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
				NodeID:    logNodeID,
				NodeName:  logNodeName,
				Operation: "DeployK8sCluster",
				Command:   fmt.Sprintf("部署Kubernetes集群，版本: %s，架构: %s，发行版: %s", req.KubeVersion, req.Arch, req.Distro),
				Output:    logMsg,
				Status:    "running",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			nodeManager.CreateLog(logEntry)
		}

		result, err := kubeadm.DeployK8sCluster(ctx, nodes, req.KubeVersion, req.Arch, req.Distro, scriptManager, req.SkipSteps, logCallback)
		if err != nil {
			// 记录部署失败日志
			deployLog.Output = fmt.Sprintf("部署失败: %v\n详细错误: %s\n", err, result)
			deployLog.Status = "failed"
			deployLog.UpdatedAt = time.Now()
			nodeManager.CreateLog(deployLog)

			fmt.Printf("部署失败: %v\n详细错误: %s\n", err, result)

			// 返回详细的错误信息
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("部署Kubernetes集群失败: %v\n详细信息: %s", err, result),
			})
			return
		}

		// 记录部署成功日志
		deployLog.Output = fmt.Sprintf("部署成功!\n结果: %s\n", result)
		deployLog.Status = "success"
		deployLog.UpdatedAt = time.Now()
		nodeManager.CreateLog(deployLog)

		fmt.Printf("部署成功!\n结果: %s\n", result)

		// 返回部署成功结果
		c.JSON(http.StatusOK, gin.H{
			"result":  result,
			"message": "Kubernetes集群部署成功",
			"nodes":   nodeNames,
			"version": req.KubeVersion,
		})
	})

	// Node management routes
	// 获取所有节点
	r.GET("/nodes", func(c *gin.Context) {
		nodes, err := nodeManager.GetNodes()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		// 确保返回的是数组类型，而不是null
		// 显式创建一个切片，确保Gin将其序列化为数组
		responseNodes := []node.Node{}
		if nodes != nil {
			responseNodes = nodes
		}
		c.JSON(http.StatusOK, responseNodes)
	})

	// 获取单个节点
	r.GET("/nodes/:id", func(c *gin.Context) {
		id := c.Param("id")
		node, err := nodeManager.GetNode(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, node)
	})

	// 创建节点
	r.POST("/nodes", func(c *gin.Context) {
		var node node.Node
		if err := c.ShouldBindJSON(&node); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		createdNode, err := nodeManager.CreateNode(node)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusCreated, createdNode)
	})

	// 更新节点
	r.PUT("/nodes/:id", func(c *gin.Context) {
		id := c.Param("id")
		var node node.Node
		if err := c.ShouldBindJSON(&node); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		updatedNode, err := nodeManager.UpdateNode(id, node)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, updatedNode)
	})

	// 删除节点
	r.DELETE("/nodes/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := nodeManager.DeleteNode(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	})

	// 测试节点连接
	r.POST("/nodes/:id/test-connection", func(c *gin.Context) {
		id := c.Param("id")
		connected, err := nodeManager.TestConnection(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"connected": connected,
		})
	})

	// 容器运行时相关API端点 - 暂时注释，因为节点管理器没有实现这些方法
	/*
		// 安装容器运行时
		r.POST("/nodes/:id/runtime/install", func(c *gin.Context) {
			id := c.Param("id")

			var req struct {
				RuntimeType string `json:"runtimeType"`
				Version     string `json:"version"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			if err := nodeManager.InstallContainerRuntime(id, req.RuntimeType, req.Version); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status": "container runtime installed successfully",
			})
		})

		// 配置容器运行时
		r.POST("/nodes/:id/runtime/configure", func(c *gin.Context) {
			id := c.Param("id")

			var config node.ContainerRuntimeConfig
			if err := c.ShouldBindJSON(&config); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			if err := nodeManager.ConfigureContainerRuntime(id, config); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status": "container runtime configured successfully",
			})
		})

		// 启动容器运行时
		r.POST("/nodes/:id/runtime/start", func(c *gin.Context) {
			id := c.Param("id")

			var req struct {
				RuntimeType string `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			if err := nodeManager.StartContainerRuntime(id, req.RuntimeType); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status": "container runtime started successfully",
			})
		})

		// 停止容器运行时
		r.POST("/nodes/:id/runtime/stop", func(c *gin.Context) {
			id := c.Param("id")

			var req struct {
				RuntimeType string `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			if err := nodeManager.StopContainerRuntime(id, req.RuntimeType); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status": "container runtime stopped successfully",
			})
		})

		// 移除容器运行时
		r.POST("/nodes/:id/runtime/remove", func(c *gin.Context) {
			id := c.Param("id")

			var req struct {
				RuntimeType string `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			if err := nodeManager.RemoveContainerRuntime(id, req.RuntimeType); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status": "container runtime removed successfully",
			})
		})

		// 启用容器运行时开机自启
		r.POST("/nodes/:id/runtime/enable", func(c *gin.Context) {
			id := c.Param("id")

			var req struct {
				RuntimeType string `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			if err := nodeManager.EnableContainerRuntime(id, req.RuntimeType); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status": "container runtime enabled successfully",
			})
		})

		// 禁用容器运行时开机自启
		r.POST("/nodes/:id/runtime/disable", func(c *gin.Context) {
			id := c.Param("id")

			var req struct {
				RuntimeType string `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			if err := nodeManager.DisableContainerRuntime(id, req.RuntimeType); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status": "container runtime disabled successfully",
			})
		})

		// 检查容器运行时状态
		r.GET("/nodes/:id/runtime/status", func(c *gin.Context) {
			id := c.Param("id")

			runtimeType := c.Query("runtimeType")
			if runtimeType == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "runtimeType is required",
				})
				return
			}

			status, err := nodeManager.CheckContainerRuntimeStatus(id, runtimeType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status": status,
			})
		})

		// 批量安装容器运行时
		r.POST("/nodes/runtime/batch-install", func(c *gin.Context) {
			var req struct {
				NodeIds     []string `json:"nodeIds"`
				RuntimeType string   `json:"runtimeType"`
				Version     string   `json:"version"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			result, err := nodeManager.BatchInstallContainerRuntime(req.NodeIds, req.RuntimeType, req.Version)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"result": result,
			})
		})

		// 批量配置容器运行时
		r.POST("/nodes/runtime/batch-configure", func(c *gin.Context) {
			var req struct {
				NodeIds []string                    `json:"nodeIds"`
				Config  node.ContainerRuntimeConfig `json:"config"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			result, err := nodeManager.BatchConfigureContainerRuntime(req.NodeIds, req.Config)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"result": result,
			})
		})

		// 批量启动容器运行时
		r.POST("/nodes/runtime/batch-start", func(c *gin.Context) {
			var req struct {
				NodeIds     []string `json:"nodeIds"`
				RuntimeType string   `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			result, err := nodeManager.BatchStartContainerRuntime(req.NodeIds, req.RuntimeType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"result": result,
			})
		})

		// 批量停止容器运行时
		r.POST("/nodes/runtime/batch-stop", func(c *gin.Context) {
			var req struct {
				NodeIds     []string `json:"nodeIds"`
				RuntimeType string   `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			result, err := nodeManager.BatchStopContainerRuntime(req.NodeIds, req.RuntimeType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"result": result,
			})
		})

		// 批量移除容器运行时
		r.POST("/nodes/runtime/batch-remove", func(c *gin.Context) {
			var req struct {
				NodeIds     []string `json:"nodeIds"`
				RuntimeType string   `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			result, err := nodeManager.BatchRemoveContainerRuntime(req.NodeIds, req.RuntimeType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"result": result,
			})
		})

		// 批量启用容器运行时开机自启
		r.POST("/nodes/runtime/batch-enable", func(c *gin.Context) {
			var req struct {
				NodeIds     []string `json:"nodeIds"`
				RuntimeType string   `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			result, err := nodeManager.BatchEnableContainerRuntime(req.NodeIds, req.RuntimeType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"result": result,
			})
		})

		// 批量禁用容器运行时开机自启
		r.POST("/nodes/runtime/batch-disable", func(c *gin.Context) {
			var req struct {
				NodeIds     []string `json:"nodeIds"`
				RuntimeType string   `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			result, err := nodeManager.BatchDisableContainerRuntime(req.NodeIds, req.RuntimeType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"result": result,
			})
		})

		// 批量检查容器运行时状态
		r.POST("/nodes/runtime/batch-status", func(c *gin.Context) {
			var req struct {
				NodeIds     []string `json:"nodeIds"`
				RuntimeType string   `json:"runtimeType"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}

			statusMap, err := nodeManager.BatchCheckContainerRuntimeStatus(req.NodeIds, req.RuntimeType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, statusMap)
		})
	*/

	// 安装Kubernetes组件
	r.POST("/nodes/:id/kubernetes/install", func(c *gin.Context) {
		id := c.Param("id")

		var req struct {
			KubeadmVersion string `json:"kubeadmVersion" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":     "请求参数错误: " + err.Error(),
				"details":   err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			})
			return
		}

		// 获取节点信息
		node, err := nodeManager.GetNode(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":          "获取节点信息失败",
				"details":        fmt.Sprintf("failed to get node: %v", err),
				"timestamp":      time.Now().Format(time.RFC3339),
				"nodeId":         id,
				"kubeadmVersion": req.KubeadmVersion,
			})
			return
		}

		// 记录安装开始日志
		installLog := log.LogEntry{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			NodeID:    node.ID,
			NodeName:  node.Name,
			Operation: "InstallKubernetesComponents",
			Command:   fmt.Sprintf("安装Kubernetes组件，版本: %s", req.KubeadmVersion),
			Output:    "开始安装Kubernetes组件...",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		nodeManager.CreateLog(installLog)

		// 记录安装请求
		fmt.Printf("开始为节点 %s 安装Kubernetes组件，版本: %s\n", id, req.KubeadmVersion)

		if err := nodeManager.InstallKubernetesComponents(id, req.KubeadmVersion); err != nil {
			// 记录详细错误日志
			fmt.Printf("节点 %s 安装Kubernetes组件失败: %v\n", id, err)

			// 记录安装失败日志
			installLog.Output = fmt.Sprintf("安装失败: %v", err)
			installLog.Status = "failed"
			installLog.UpdatedAt = time.Now()
			nodeManager.CreateLog(installLog)

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":          "安装Kubernetes组件失败",
				"details":        err.Error(),
				"timestamp":      time.Now().Format(time.RFC3339),
				"nodeId":         id,
				"kubeadmVersion": req.KubeadmVersion,
			})
			return
		}

		// 记录安装成功日志
		installLog.Output = "安装成功"
		installLog.Status = "success"
		installLog.UpdatedAt = time.Now()
		nodeManager.CreateLog(installLog)

		// 记录成功日志
		fmt.Printf("节点 %s 成功安装Kubernetes组件，版本: %s\n", id, req.KubeadmVersion)

		c.JSON(http.StatusOK, gin.H{
			"status":         "success",
			"message":        "Kubernetes组件安装成功",
			"result":         "Kubernetes组件安装成功", // 添加result字段，兼容前端期望
			"timestamp":      time.Now().Format(time.RFC3339),
			"nodeId":         id,
			"kubeadmVersion": req.KubeadmVersion,
		})
	})

	// SSH相关API端点
	// 配置节点SSH设置
	r.POST("/nodes/:id/ssh/configure", func(c *gin.Context) {
		id := c.Param("id")
		if err := nodeManager.ConfigureSSHSettings(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "SSH settings configured successfully",
		})
	})

	// 配置所有节点之间的SSH免密互通
	r.POST("/nodes/ssh/passwdless", func(c *gin.Context) {
		if err := nodeManager.ConfigureSSHPasswdless(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "SSH passwdless configuration completed successfully",
		})
	})

	// 日志相关API端点
	// 获取所有日志
	r.GET("/logs", func(c *gin.Context) {
		logs, err := nodeManager.GetLogs()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"logs": logs,
		})
	})

	// 获取指定节点的日志
	r.GET("/logs/node/:id", func(c *gin.Context) {
		id := c.Param("id")
		logs, err := nodeManager.GetLogsByNode(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"logs": logs,
		})
	})

	// 清除所有日志
	r.DELETE("/logs", func(c *gin.Context) {
		if err := nodeManager.ClearLogs(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "logs cleared successfully",
		})
	})

	// 实时日志流API
	r.GET("/logs/stream", func(c *gin.Context) {
		// 设置响应头，支持SSE
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "*")

		// 立即发送一个心跳事件，确认连接建立
		fmt.Fprintf(c.Writer, "data: {\"type\": \"heartbeat\", \"message\": \"连接已建立\"}\n\n")
		c.Writer.(http.Flusher).Flush()

		// 获取日志管理器
		logManager := nodeManager.GetLogManager()

		// 创建日志通道
		var logChan <-chan log.LogEntry
		var subscription log.LogSubscription

		// 检查日志管理器是否支持订阅功能
		if lm, ok := logManager.(interface {
			SubscribeLogs() log.LogSubscription
			UnsubscribeLogs(sub log.LogSubscription)
		}); ok {
			// 订阅日志事件
			subscription = lm.SubscribeLogs()
			logChan = subscription.Ch

			// 客户端断开连接时取消订阅
			defer func() {
				lm.UnsubscribeLogs(subscription)
			}()
		} else {
			// 如果不支持订阅功能，创建一个新的通道并定期发送心跳
			ch := make(chan log.LogEntry, 100)
			logChan = ch

			// 定期发送心跳
			go func() {
				for {
					select {
					case <-time.After(30 * time.Second):
						select {
						case ch <- log.LogEntry{
							ID:        fmt.Sprintf("heartbeat-%d", time.Now().UnixNano()),
							Operation: "Heartbeat",
							NodeName:  "系统",
							CreatedAt: time.Now(),
						}:
							// 心跳发送成功
						default:
							// 通道已满，跳过此心跳
						}
					case <-c.Request.Context().Done():
						close(ch)
						return
					}
				}
			}()
		}

		// 客户端断开连接时关闭通道
		c.Stream(func(w io.Writer) bool {
			select {
			case logEntry := <-logChan:
				// 将日志格式化为SSE格式
				logJSON, err := json.Marshal(logEntry)
				if err != nil {
					return true
				}
				fmt.Fprintf(w, "data: %s\n\n", logJSON)
				c.Writer.(http.Flusher).Flush()
				return true
			case <-c.Request.Context().Done():
				// 客户端断开连接
				return false
			case <-time.After(60 * time.Second):
				// 60秒内没有日志，发送一个心跳事件，保持连接活跃
				fmt.Fprintf(w, "data: {\"type\": \"heartbeat\", \"message\": \"连接保持活跃\"}\n\n")
				c.Writer.(http.Flusher).Flush()
				return true
			}
		})
	})

	// 系统脚本管理API端点
	// 获取系统脚本
	r.GET("/scripts", func(c *gin.Context) {
		// 使用脚本管理器获取脚本
		c.JSON(http.StatusOK, gin.H{
			"scripts": scriptManager.GetScripts(),
		})
	})

	// 保存自定义系统脚本
	r.POST("/scripts", func(c *gin.Context) {
		var scripts map[string]string
		if err := c.ShouldBindJSON(&scripts); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 使用脚本管理器更新并保存脚本
		scriptManager.UpdateScripts(scripts)
		if err := scriptManager.SaveScripts(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "scripts saved successfully",
		})
	})

	// 部署流程脚本管理API端点
	// 获取部署流程脚本
	r.GET("/deployment-process/scripts", func(c *gin.Context) {
		// 获取所有部署流程脚本
		c.JSON(http.StatusOK, gin.H{
			"scripts": scriptManager.GetScripts(),
		})
	})

	// 保存部署流程脚本
	r.POST("/deployment-process/scripts", func(c *gin.Context) {
		var scripts map[string]string
		if err := c.ShouldBindJSON(&scripts); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 更新脚本
		scriptManager.UpdateScripts(scripts)

		// 保存到文件
		if err := scriptManager.SaveScripts(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "scripts saved successfully",
		})
	})

	// 重置部署流程脚本到默认脚本
	r.POST("/deployment-process/scripts/reset", func(c *gin.Context) {
		// 获取默认脚本
		defaultScripts := scriptManager.GetDefaultScripts()

		// 更新脚本管理器
		scriptManager.UpdateScripts(defaultScripts)

		// 保存到文件
		if err := scriptManager.SaveScripts(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":       "scripts reset to default",
			"message":      "所有脚本已重置为默认脚本",
			"scriptsCount": len(defaultScripts),
		})
	})

	// 获取单个脚本的默认值
	r.GET("/deployment-process/scripts/:name/default", func(c *gin.Context) {
		scriptName := c.Param("name")
		// 获取所有默认脚本
		defaultScripts := scriptManager.GetDefaultScripts()
		// 查找指定脚本
		if scriptContent, exists := defaultScripts[scriptName]; exists {
			c.JSON(http.StatusOK, gin.H{
				"status":        "success",
				"message":       "获取默认脚本成功",
				"scriptName":    scriptName,
				"scriptContent": scriptContent,
			})
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"error":      "script not found",
				"message":    "未找到指定的默认脚本",
				"scriptName": scriptName,
			})
		}
	})

	// Start server
	r.Run(":8080")
}
