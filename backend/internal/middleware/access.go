package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const AccessSessionKey = "access_authenticated"

func AccessRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authenticated, _ := sessions.Default(c).Get(AccessSessionKey).(bool)
		if !authenticated {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "需要访问登录",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
