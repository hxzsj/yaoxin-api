package middleware

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	if len(common.AllowedWebOrigins) > 0 {
		// 白名单模式：仅允许配置的 Origin 跨域访问
		config.AllowOrigins = common.AllowedWebOrigins
	} else {
		// 回退模式：未配置 ALLOWED_ORIGINS 时允许所有来源（开发环境兼容）
		config.AllowAllOrigins = true
	}
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	return cors.New(config)
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅 Debug 模式暴露版本号信息，生产环境隐藏版本指纹
		if gin.Mode() == gin.DebugMode {
			c.Header("X-New-Api-Version", common.Version)
		}
		c.Next()
	}
}
