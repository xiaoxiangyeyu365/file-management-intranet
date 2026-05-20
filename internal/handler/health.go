package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck godoc
// @Summary 健康检查
// @Description 服务健康状态检查，用于监控和负载均衡探测
// @Tags system
// @Produce json
// @Success 200 {object} map[string]interface{} "服务正常"
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "cloudbox",
	})
}