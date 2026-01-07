package main

import (
	"fmt"
	"k8s-installer/kubeadm"
	"k8s-installer/node"
	"net/http"

	"github.com/gin-gonic/gin"
)

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

	// 初始化节点管理器（SQLite实现）
	nodeManager, err := node.NewSqliteNodeManager("k8s_installer.db")
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize SQLite node manager: %v", err))
	}

	// API routes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Kubeadm routes
	r.GET("/kubeadm/version", func(c *gin.Context) {
		version, err := kubeadm.CheckKubeadmVersion()
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

	// Kubeadm 包管理路由
	r.GET("/kubeadm/packages", func(c *gin.Context) {
		// 返回可用的Kubeadm版本列表
		versions := []string{
			"v1.30.0",
			"v1.29.4",
			"v1.28.8",
			"v1.27.12",
		}
		c.JSON(http.StatusOK, gin.H{
			"versions": versions,
		})
	})

	// Docker 包管理路由
	r.GET("/docker/packages", func(c *gin.Context) {
		// 返回可用的Docker版本列表
		versions := []string{
			"27.0.3",
			"26.1.4",
			"25.0.5",
			"24.0.7",
		}
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
		packagePath, err := kubeadm.DownloadKubeadmPackage(req.Version, req.Arch, req.Distro, req.SourceURL)
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
		err := kubeadm.DeployKubeadmPackage(req.PackagePath, req.NodeIP, req.Username, req.Password, req.Port, req.PrivateKey)
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
		var config kubeadm.KubeadmConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		result, err := kubeadm.InitMaster(config)
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

	// 拉取Kubernetes镜像到本地
	r.POST("/kubeadm/images/pull", func(c *gin.Context) {
		var req struct {
			Version string `json:"version" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		result, err := kubeadm.PullKubernetesImages(req.Version)
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

	// 将Kubernetes镜像推送到Harbor仓库
	r.POST("/kubeadm/images/push-to-harbor", func(c *gin.Context) {
		var req struct {
			HarborConfig      kubeadm.HarborConfig `json:"harborConfig" binding:"required"`
			KubernetesVersion string               `json:"kubernetesVersion" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		result, err := kubeadm.PushImagesToHarbor(req.HarborConfig, req.KubernetesVersion)
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

	r.GET("/kubeadm/join-command", func(c *gin.Context) {
		cmd, err := kubeadm.GetJoinCommand()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"command": cmd,
		})
	})

	r.POST("/kubeadm/reset", func(c *gin.Context) {
		result, err := kubeadm.ResetCluster()
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

	r.POST("/kubeadm/join", func(c *gin.Context) {
		var req struct {
			Token                string `json:"token"`
			CACertHash           string `json:"caCertHash"`
			ControlPlaneEndpoint string `json:"controlPlaneEndpoint"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		result, err := kubeadm.JoinWorker(req.Token, req.CACertHash, req.ControlPlaneEndpoint)
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
		c.JSON(http.StatusOK, nodes)
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

	// Docker相关API端点
	// 安装Docker
	r.POST("/nodes/:id/docker/install", func(c *gin.Context) {
		id := c.Param("id")

		// 解析请求体，获取版本参数
		var req struct {
			Version string `json:"version"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			// 如果没有提供版本参数，使用默认版本
			req.Version = ""
		}

		if err := nodeManager.InstallDocker(id, req.Version); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "docker installed successfully",
		})
	})

	// 配置Docker
	r.POST("/nodes/:id/docker/configure", func(c *gin.Context) {
		id := c.Param("id")
		var config node.DockerConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		if err := nodeManager.ConfigureDocker(id, config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "docker configured successfully",
		})
	})

	// 启动Docker
	r.POST("/nodes/:id/docker/start", func(c *gin.Context) {
		id := c.Param("id")
		if err := nodeManager.StartDocker(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "docker started successfully",
		})
	})

	// 停止Docker
	r.POST("/nodes/:id/docker/stop", func(c *gin.Context) {
		id := c.Param("id")
		if err := nodeManager.StopDocker(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "docker stopped successfully",
		})
	})

	// 检查Docker状态
	r.GET("/nodes/:id/docker/status", func(c *gin.Context) {
		id := c.Param("id")
		status, err := nodeManager.CheckDockerStatus(id)
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

	// 批量安装Docker
	r.POST("/nodes/docker/batch-install", func(c *gin.Context) {
		var req struct {
			NodeIds []string `json:"nodeIds" binding:"required"`
			Version string   `json:"version"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		results, err := nodeManager.BatchInstallDocker(req.NodeIds, req.Version)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"results": results,
		})
	})

	// 批量配置Docker
	r.POST("/nodes/docker/batch-configure", func(c *gin.Context) {
		var req struct {
			NodeIds []string          `json:"nodeIds" binding:"required"`
			Config  node.DockerConfig `json:"config" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		results, err := nodeManager.BatchConfigureDocker(req.NodeIds, req.Config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"results": results,
		})
	})

	// 批量启动Docker
	r.POST("/nodes/docker/batch-start", func(c *gin.Context) {
		var req struct {
			NodeIds []string `json:"nodeIds" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		results, err := nodeManager.BatchStartDocker(req.NodeIds)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"results": results,
		})
	})

	// 批量停止Docker
	r.POST("/nodes/docker/batch-stop", func(c *gin.Context) {
		var req struct {
			NodeIds []string `json:"nodeIds" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		results, err := nodeManager.BatchStopDocker(req.NodeIds)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"results": results,
		})
	})

	// 批量检查Docker状态
	r.POST("/nodes/docker/batch-status", func(c *gin.Context) {
		var req struct {
			NodeIds []string `json:"nodeIds" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		statusMap, err := nodeManager.BatchCheckDockerStatus(req.NodeIds)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": statusMap,
		})
	})

	// 删除Docker
	r.POST("/nodes/:id/docker/remove", func(c *gin.Context) {
		id := c.Param("id")
		if err := nodeManager.RemoveDocker(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "docker removed successfully",
		})
	})

	// 启用Docker开机自启
	r.POST("/nodes/:id/docker/enable", func(c *gin.Context) {
		id := c.Param("id")
		if err := nodeManager.EnableDocker(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "docker auto-start enabled successfully",
		})
	})

	// 禁用Docker开机自启
	r.POST("/nodes/:id/docker/disable", func(c *gin.Context) {
		id := c.Param("id")
		if err := nodeManager.DisableDocker(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "docker auto-start disabled successfully",
		})
	})

	// 批量删除Docker
	r.POST("/nodes/docker/batch-remove", func(c *gin.Context) {
		var req struct {
			NodeIds []string `json:"nodeIds" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		results, err := nodeManager.BatchRemoveDocker(req.NodeIds)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"results": results,
		})
	})

	// 批量启用Docker开机自启
	r.POST("/nodes/docker/batch-enable", func(c *gin.Context) {
		var req struct {
			NodeIds []string `json:"nodeIds" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		results, err := nodeManager.BatchEnableDocker(req.NodeIds)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"results": results,
		})
	})

	// 批量禁用Docker开机自启
	r.POST("/nodes/docker/batch-disable", func(c *gin.Context) {
		var req struct {
			NodeIds []string `json:"nodeIds" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		results, err := nodeManager.BatchDisableDocker(req.NodeIds)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"results": results,
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

	// Start server
	r.Run(":8080")
}
