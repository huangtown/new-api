package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stackTrace := string(debug.Stack())
				common.SysLog(fmt.Sprintf("panic detected: %v", err))
				common.SysLog(fmt.Sprintf("stacktrace from panic: %s", stackTrace))
				method, path := "", ""
				if c.Request != nil {
					method, path = c.Request.Method, c.Request.URL.String()
				}
				gopool.Go(func() {
					service.NotifyError("Relay Panic", fmt.Sprintf("Panic: %v\n\nStack:\n%s\n\nRequest: %s %s",
						err, stackTrace, method, path))
				})
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"message": fmt.Sprintf("Panic detected, error: %v. Please submit a issue here: https://github.com/Calcium-Ion/new-api", err),
						"type":    "new_api_panic",
					},
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
