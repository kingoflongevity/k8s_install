package main

import (
	"github.com/gin-gonic/gin"
	"k8s-installer/kubeadm"
	"k8s-installer/node"
	"net/http"
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

	// 初始化节点管理器
	nodeManager := node.NewMemoryNodeManager()

	// API routes
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Kubeadm routes
	r.GET("/api/kubeadm/version", func(c *gin.Context) {
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
	r.GET("/api/kubeadm/packages", func(c *gin.Context) {
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

	r.POST("/api/kubeadm/packages/download", func(c *gin.Context) {
		var req struct {
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
		
		// 下载指定版本的Kubeadm包
		packagePath, err := kubeadm.DownloadKubeadmPackage(req.Version, req.Arch, req.Distro)
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

	r.POST("/api/kubeadm/packages/deploy", func(c *gin.Context) {
		var req struct {
			PackagePath    string `json:"packagePath" binding:"required"`
			NodeIP         string `json:"nodeIP" binding:"required"`
			Username       string `json:"username" binding:"required"`
			Password       string `json:"password"`
			Port           int    `json:"port"`
			PrivateKey     string `json:"privateKey"`
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

	r.POST("/api/kubeadm/init", func(c *gin.Context) {
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

	r.GET("/api/kubeadm/join-command", func(c *gin.Context) {
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

	r.POST("/api/kubeadm/reset", func(c *gin.Context) {
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

	r.POST("/api/kubeadm/join", func(c *gin.Context) {
		var req struct {
			Token               string `json:"token"`
			CACertHash          string `json:"caCertHash"`
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
	r.GET("/api/nodes", func(c *gin.Context) {
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
	r.GET("/api/nodes/:id", func(c *gin.Context) {
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
	r.POST("/api/nodes", func(c *gin.Context) {
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
	r.PUT("/api/nodes/:id", func(c *gin.Context) {
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
	r.DELETE("/api/nodes/:id", func(c *gin.Context) {
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
	r.POST("/api/nodes/:id/test-connection", func(c *gin.Context) {
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

	// 部署节点
	r.POST("/api/nodes/:id/deploy", func(c *gin.Context) {
		id := c.Param("id")
		if err := nodeManager.DeployNode(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "deploying",
		})
	})

	// Start server
	r.Run(":8080")
}