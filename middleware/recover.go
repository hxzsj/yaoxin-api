package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 始终记录完整 panic 信息和堆栈到服务端日志
				common.SysLog(fmt.Sprintf("panic detected: %v", err))
				common.SysLog(fmt.Sprintf("stacktrace from panic: %s", string(debug.Stack())))
				// 生产环境隐藏错误详情，仅返回通用消息
				var errMsg string
				if gin.Mode() == gin.DebugMode {
					errMsg = fmt.Sprintf("Panic detected, error: %v. Please submit a issue here: https://github.com/Calcium-Ion/new-api", err)
				} else {
					errMsg = "Internal server error"
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"message": errMsg,
						"type":    "new_api_panic",
					},
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
