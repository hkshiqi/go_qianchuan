package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWriteGuardBlocksWritesWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(WriteGuard(false))
	router.POST("/write", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/read", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	write := httptest.NewRecorder()
	router.ServeHTTP(write, httptest.NewRequest(http.MethodPost, "/write", nil))
	assert.Equal(t, http.StatusForbidden, write.Code)

	read := httptest.NewRecorder()
	router.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/read", nil))
	assert.Equal(t, http.StatusNoContent, read.Code)
}

func TestWriteGuardAllowsWritesWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(WriteGuard(true))
	router.POST("/write", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	write := httptest.NewRecorder()
	router.ServeHTTP(write, httptest.NewRequest(http.MethodPost, "/write", nil))
	assert.Equal(t, http.StatusNoContent, write.Code)
}

func TestWriteGuardAllowsDeclaredReadOnlyPOST(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(WriteGuard(false, "/api/report/read"))
	router.POST("/api/report/read", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/api/report/write", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	read := httptest.NewRecorder()
	router.ServeHTTP(read, httptest.NewRequest(http.MethodPost, "/api/report/read", nil))
	assert.Equal(t, http.StatusNoContent, read.Code)

	write := httptest.NewRecorder()
	router.ServeHTTP(write, httptest.NewRequest(http.MethodPost, "/api/report/write", nil))
	assert.Equal(t, http.StatusForbidden, write.Code)
}
