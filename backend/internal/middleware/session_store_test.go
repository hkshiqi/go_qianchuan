package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	backendSession "github.com/CriarBrand/qianchuan-backend/pkg/session"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
)

func TestMemstorePersistsQianchuanSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := memstore.NewStore([]byte("01234567890123456789012345678901"))
	router := gin.New()
	router.Use(sessions.Sessions("qianchuan_session", store))
	router.GET("/set", func(c *gin.Context) {
		sess := sessions.Default(c)
		sess.Set("user", &backendSession.UserSession{
			AdvertiserID:   123,
			AccessToken:    "access-token",
			RefreshToken:   "refresh-token",
			ExpiresAt:      time.Now().Add(time.Hour).Unix(),
			RefreshExpires: time.Now().Add(24 * time.Hour).Unix(),
		})
		if err := sess.Save(); err != nil {
			t.Fatalf("save session: %v", err)
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/get", func(c *gin.Context) {
		value := sessions.Default(c).Get("user")
		user, ok := value.(*backendSession.UserSession)
		if !ok {
			t.Fatalf("session type = %T, want *session.UserSession", value)
		}
		if user.AdvertiserID != 123 || user.AccessToken != "access-token" {
			t.Fatalf("unexpected session: %+v", user)
		}
		c.Status(http.StatusNoContent)
	})

	setRequest := httptest.NewRequest(http.MethodGet, "/set", nil)
	setResponse := httptest.NewRecorder()
	router.ServeHTTP(setResponse, setRequest)
	if setResponse.Code != http.StatusNoContent {
		t.Fatalf("set status = %d", setResponse.Code)
	}
	cookies := setResponse.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("session cookie was not set")
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/get", nil)
	for _, cookie := range cookies {
		getRequest.AddCookie(cookie)
	}
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusNoContent {
		t.Fatalf("get status = %d: %s", getResponse.Code, getResponse.Body.String())
	}
}
