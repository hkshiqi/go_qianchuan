package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := cookie.NewStore([]byte("test-secret-key-32-bytes-long!!"))
	router.Use(sessions.Sessions("test_session", store))
	return router
}

func setupTestService() *service.QianchuanService {
	client := sdk.NewOceanengineClient(123456789, "test-secret")
	return service.NewQianchuanService(client, 123456789, "test-secret")
}

func TestLegacyOAuthExchangeIsNotExposed(t *testing.T) {
	router := setupTestRouter()

	request, _ := http.NewRequest(http.MethodPost, "/oauth/exchange", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestGetUserInfoRequiresSession(t *testing.T) {
	router := setupTestRouter()
	authHandler := NewAuthHandler(setupTestService())
	router.GET("/user/info", authHandler.GetUserInfo)

	request, _ := http.NewRequest(http.MethodGet, "/user/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	var response map[string]any
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, float64(http.StatusUnauthorized), response["code"])
}

func TestLogoutClearsSession(t *testing.T) {
	router := setupTestRouter()
	authHandler := NewAuthHandler(setupTestService())
	router.POST("/logout", authHandler.Logout)

	request, _ := http.NewRequest(http.MethodPost, "/logout", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response map[string]any
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, float64(0), response["code"])
	assert.Equal(t, "登出成功", response["message"])
}

func TestRefreshSessionRequiresSession(t *testing.T) {
	router := setupTestRouter()
	authHandler := NewAuthHandler(setupTestService())
	router.POST("/refresh", authHandler.RefreshSession)

	request, _ := http.NewRequest(http.MethodPost, "/refresh", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
