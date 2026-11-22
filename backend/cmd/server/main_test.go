package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNormalizePublicPathPrefix(t *testing.T) {
	tests := map[string]string{
		"":                    "",
		"/":                   "",
		"qianchuan-app":       "/qianchuan-app",
		"/qianchuan-app/":     "/qianchuan-app",
		"  /qianchuan-app/  ": "/qianchuan-app",
	}
	for input, expected := range tests {
		if actual := normalizePublicPathPrefix(input); actual != expected {
			t.Fatalf("normalizePublicPathPrefix(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestOperationReadOnlyPathsAddsPublicPrefix(t *testing.T) {
	input := operationReadOnlyPOSTPaths[0]
	paths := operationReadOnlyPaths("/qianchuan-app")
	if len(paths) != len(operationReadOnlyPOSTPaths) {
		t.Fatalf("unexpected path count: %d", len(paths))
	}
	if paths[0] != "/qianchuan-app"+input {
		t.Fatalf("prefixed path = %q", paths[0])
	}
}

func TestPublicURLUsesExplicitOAuthOverride(t *testing.T) {
	actual := publicURL(
		"https://qianchuan.example/oauth/authorize",
		"https://qianchuan.example",
		"/qianchuan-app",
		"/oauth/authorize",
	)
	expected := "https://qianchuan.example/oauth/authorize"
	if actual != expected {
		t.Fatalf("publicURL override = %q, want %q", actual, expected)
	}
}

func TestPublicURLKeepsLegacyPrefixWithoutOverride(t *testing.T) {
	actual := publicURL("", "https://qianchuan.example/", "qianchuan-app/", "/oauth/authorize")
	expected := "https://qianchuan.example/qianchuan-app/oauth/authorize"
	if actual != expected {
		t.Fatalf("publicURL default = %q, want %q", actual, expected)
	}
}

func TestDedicatedHostUsesRootRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	register := func(group *gin.RouterGroup) {
		group.GET("/health", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})
		group.GET("/api/access/status", func(c *gin.Context) {
			c.String(http.StatusOK, "status")
		})
	}
	register(router.Group("/qianchuan-app"))
	register(&router.RouterGroup)

	for _, path := range []string{
		"/qianchuan-app/health",
		"/qianchuan-app/api/access/status",
		"/health",
		"/api/access/status",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s returned %d, want 200", path, recorder.Code)
		}
	}
}
