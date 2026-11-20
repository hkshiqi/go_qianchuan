package handler

import (
	"bytes"
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
	advertiserID  int64
	advertiserIDs []int64
	roots         []sdk.AdvertiserListItem
	workspaceIDs  []int64
	workspaceCode int64
	workspaceErr  error
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
			AdvertiserIds:         c.advertiserIDs,
		},
	}, nil
}

func (c oauthTestClient) AdvertiserList(context.Context, sdk.AdvertiserListReq) (*sdk.AdvertiserListRes, error) {
	return &sdk.AdvertiserListRes{
		Code: 0,
		Data: struct {
			List []sdk.AdvertiserListItem `json:"list"`
		}{List: c.roots},
	}, nil
}

func (c oauthTestClient) CustomerCenterAdvertiserList(context.Context, sdk.CustomerCenterAdvertiserListReq) (*sdk.CustomerCenterAdvertiserListRes, error) {
	if c.workspaceErr != nil {
		return nil, c.workspaceErr
	}
	response := &sdk.CustomerCenterAdvertiserListRes{Code: c.workspaceCode, Message: "test"}
	if response.Code != 0 {
		return response, nil
	}
	response.Message = "success"
	for _, id := range c.workspaceIDs {
		response.Data.List = append(response.Data.List, sdk.CustomerCenterAdvertiserItem{AdvertiserId: id})
	}
	response.Data.PageInfo.TotalPage = 1
	return response, nil
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

func TestOAuthCallbackResolvesWorkspaceRootToBoundAdvertiser(t *testing.T) {
	handler, factory, tenant := buildOAuthTestHandler(t, oauthTestClient{
		advertiserID: 9001,
		roots: []sdk.AdvertiserListItem{{
			AdvertiserId: 9001,
			AccountRole:  "CUSTOMER_ADMIN",
		}},
		workspaceIDs: []int64{1001, 9999},
	})
	state, err := handler.states.Issue(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	handler.Callback(oauthContext(t, state))

	path, err := factory.TokenStorePath(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytesContains(raw, []byte(`"1001"`)) {
		t.Fatal("bound workspace advertiser 1001 was not persisted")
	}
	if bytesContains(raw, []byte(`"9999"`)) {
		t.Fatal("workspace advertiser outside tenant binding was persisted")
	}
}

func TestOAuthCallbackPersistsSelectedAccountWhenWorkspaceListFails(t *testing.T) {
	handler, factory, tenant := buildOAuthTestHandler(t, oauthTestClient{
		advertiserID: 9001,
		roots: []sdk.AdvertiserListItem{{
			AdvertiserId: 9001,
			AccountRole:  "CUSTOMER_ADMIN",
		}},
		workspaceCode: 50000,
	})
	state, err := handler.states.IssueForAccount(tenant.ID, 1002)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&auth_code=test-code", nil)
	handler.Callback(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	caller, err := factory.ToolsFor(context, tenant)
	if err != nil {
		t.Fatal(err)
	}
	value, err := caller.Call(context, "qianchuan_get_status", map[string]any{"account_id": "1002"})
	if err != nil {
		t.Fatal(err)
	}
	status, ok := value.(mcp.Status)
	if !ok {
		t.Fatalf("status type = %T", value)
	}
	if !status.Authenticated || !status.TokenStorePresent {
		t.Fatalf("selected account was not persisted: %#v", status)
	}
}

func TestAccountScopedOAuthPersistsOnlySelectedAccount(t *testing.T) {
	handler, state, factory, tenant := testOAuthMultiAccountHandler(t)
	handler.Callback(oauthContext(t, state))

	path, err := factory.TokenStorePath(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytesContains(raw, []byte(`"1001"`)) {
		t.Fatal("unselected account 1001 was persisted")
	}
	if !bytesContains(raw, []byte(`"1002"`)) {
		t.Fatal("selected account 1002 was not persisted")
	}
}

func TestOAuthStateForForeignAccountIsRejected(t *testing.T) {
	handler, state, factory, tenant := testOAuthForeignAccountHandler(t)
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
		t.Fatalf("foreign account persisted a token: %v", err)
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

func testOAuthMultiAccountHandler(t *testing.T) (*OAuthHandler, string, *mcp.TenantFactory, *tenantstore.Tenant) {
	t.Helper()
	handler, factory, tenant := buildOAuthTestHandler(t, oauthTestClient{
		advertiserID:  1001,
		advertiserIDs: []int64{1001, 1002},
	})
	state, err := handler.states.IssueForAccount(tenant.ID, 1002)
	if err != nil {
		t.Fatal(err)
	}
	return handler, state, factory, tenant
}

func testOAuthForeignAccountHandler(t *testing.T) (*OAuthHandler, string, *mcp.TenantFactory, *tenantstore.Tenant) {
	t.Helper()
	handler, factory, tenant := buildOAuthTestHandler(t, oauthTestClient{
		advertiserID:  1001,
		advertiserIDs: []int64{1001, 9999},
	})
	state, err := handler.states.IssueForAccount(tenant.ID, 9999)
	if err != nil {
		t.Fatal(err)
	}
	return handler, state, factory, tenant
}

func buildOAuthTestHandler(
	t *testing.T,
	client oauthTestClient,
) (*OAuthHandler, *mcp.TenantFactory, *tenantstore.Tenant) {
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
		AdvertiserID: 1001,
		Accounts: []tenantstore.AccountBinding{
			{AdvertiserID: 1001},
			{AdvertiserID: 1002},
		},
	}
	if err := tenants.Register(*tenant, "owner-key-012345678901234567890123456"); err != nil {
		t.Fatal(err)
	}
	states := authstate.New(0)
	factory, err := mcp.NewTenantFactory(mcp.TenantFactoryOptions{
		Client:                client,
		AppID:                 123,
		AppSecret:             "test-secret",
		TokenRoot:             filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey:    "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState:        states.Issue,
		IssueAccountAuthState: states.IssueForAccount,
		OAuthAuthorizationURL: "https://example.test/oauth/authorize",
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewOAuthHandler(
		client,
		123,
		"https://example.test/auth/callback",
		tenants,
		states,
		factory,
	)
	return handler, factory, tenant
}

func bytesContains(haystack, needle []byte) bool {
	return bytes.Index(haystack, needle) >= 0
}

func oauthContext(t *testing.T, state string) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&auth_code=test-code", nil)
	return context
}
