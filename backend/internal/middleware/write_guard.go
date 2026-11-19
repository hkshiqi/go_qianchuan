package middleware

import (
	"net/http"
	"strings"

	"github.com/CriarBrand/qianchuan-backend/internal/util"
	"github.com/gin-gonic/gin"
)

// WriteGuard blocks state-changing HTTP methods while operations are paused.
// GET requests and explicitly declared read-only POST endpoints continue
// through unchanged.
func WriteGuard(enabled bool, readOnlyPOSTPaths ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(readOnlyPOSTPaths))
	for _, path := range readOnlyPOSTPaths {
		if path = strings.TrimSpace(path); path != "" {
			allowed[path] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		if enabled || !isWriteMethod(c.Request.Method) ||
			(c.Request.Method == http.MethodPost && isAllowedRead(c.Request.URL.Path, allowed)) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, util.ApiResponse{
			Code:    util.CodeForbidden,
			Message: "投放写操作已停用",
			Data: gin.H{
				"write_enabled": false,
			},
		})
	}
}

func isAllowedRead(path string, allowed map[string]struct{}) bool {
	_, ok := allowed[path]
	return ok
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
