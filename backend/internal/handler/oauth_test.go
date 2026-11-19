package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/CriarBrand/qianchuan-backend/internal/authstate"
	"github.com/CriarBrand/qianchuan-backend/internal/mcp"
	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
	"github.com/gin-gonic/gin"
)

type oauthTestClient struct {
	sdk.QianchuanClient
	advertiserID int64
}

func (c oauthTestClient) OauthAccessToken(context.Context, sdk.OauthAccessTokenReq) (*sdk.OauthAccessTokenRes, error) {
	return &sdk.OauthAccessTokenRes{
		Code: 0,
		Data: &sdk.OauthAccessTokenResData{
			AccessToken:           "access-token",
			RefreshToken:          "refresh-token",
			ExpiresIn:             3600,
			RefreshTokenExpiresIn: 3600,
			AdvertiserId:          c.advertiserID,
		},
	}, nil
}

func TestOAuthCallbackRejectsMismatchedAdvertiser(t *testing.T) {
	handler, state, factory, tenant := testOAuthHandler(t, 1001, 2002)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&auth_code=test-code", nil)
	handler.Callback(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	path, err := factory.TokenStorePath(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mismatched advertiser persisted a token: %v", err)
	}
}

func TestOAuthCallbackStateIsSingleUse(t *testing.T) {
	handler, state, _, _ := testOAuthHandler(t, 1001, 1001)
	handler.Callback(oauthContext(t, state))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&auth_code=second", nil)
	handler.Callback(context)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("second callback status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestOAuthCallbackRejectsInvalidState(t *testing.T) {
	handler, _, factory, tenant := testOAuthHandler(t, 1001, 1001)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/auth/callback?state=invalid&auth_code=test-code", nil)
	handler.Callback(context)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	path, err := factory.TokenStorePath(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid state persisted a token: %v", err)
	}
}

func testOAuthHandler(
	t *testing.T,
	boundAdvertiserID,
	returnedAdvertiserID int64,
) (*OAuthHandler, string, *mcp.TenantFactory, *tenantstore.Tenant) {
	t.Helper()
	dataDir := t.TempDir()
	tenants, err := tenantstore.New(
		filepath.Join(dataDir, "tenants.json"),
		"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
	)
	if err != nil {
		t.Fatal(err)
	}
	tenant := &tenantstore.Tenant{
		ID:           "owner",
		Name:         "owner",
		AdvertiserID: boundAdvertiserID,
		AwemeID:      63307887035,
		LiveRoomID:   7709635958206991154,
	}
	if err := tenants.Register(*tenant, "owner-key-012345678901234567890123456"); err != nil {
		t.Fatal(err)
	}
	states := authstate.New(0)
	factory, err := mcp.NewTenantFactory(mcp.TenantFactoryOptions{
		Client:                oauthTestClient{advertiserID: returnedAdvertiserID},
		AppID:                 123,
		AppSecret:             "test-secret",
		TokenRoot:             filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey:    "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState:        states.Issue,
		OAuthAuthorizationURL: "https://example.test/oauth/authorize",
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := states.Issue(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	return NewOAuthHandler(
		oauthTestClient{advertiserID: returnedAdvertiserID},
		123,
		"https://example.test/auth/callback",
		tenants,
		states,
		factory,
	), state, factory, tenant
}

func oauthContext(t *testing.T, state string) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&auth_code=test-code", nil)
	return context
}
