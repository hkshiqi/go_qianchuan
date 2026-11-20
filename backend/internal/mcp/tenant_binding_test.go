package mcp

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/pkg/session"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

type tenantFactoryTestClient struct {
	sdk.QianchuanClient
	roots            []sdk.AdvertiserListItem
	workspaceIDs     []int64
	shopIDs          []int64
	agentIDs         []int64
	rootListCode     int64
	rootListErr      error
	customerListCode int64
	customerListErr  error
}

func (c tenantFactoryTestClient) AdvertiserList(context.Context, sdk.AdvertiserListReq) (*sdk.AdvertiserListRes, error) {
	if c.rootListErr != nil {
		return nil, c.rootListErr
	}
	response := &sdk.AdvertiserListRes{Code: c.rootListCode, Message: "test"}
	if response.Code == 0 {
		response.Data.List = c.roots
	}
	return response, nil
}

func (c tenantFactoryTestClient) CustomerCenterAdvertiserList(context.Context, sdk.CustomerCenterAdvertiserListReq) (*sdk.CustomerCenterAdvertiserListRes, error) {
	if c.customerListErr != nil {
		return nil, c.customerListErr
	}
	response := &sdk.CustomerCenterAdvertiserListRes{Code: c.customerListCode, Message: "test"}
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

func (c tenantFactoryTestClient) ShopAdvertiserList(context.Context, sdk.ShopAdvertiserListReq) (*sdk.ShopAdvertiserListRes, error) {
	response := &sdk.ShopAdvertiserListRes{Code: 0, Message: "success"}
	for _, id := range c.shopIDs {
		response.Data.List = append(response.Data.List, sdk.ShopAdvertiserItem{AdvertiserId: id})
	}
	response.Data.PageInfo.TotalPage = 1
	return response, nil
}

func (c tenantFactoryTestClient) AgentAdvertiserList(context.Context, sdk.AgentAdvertiserListReq) (*sdk.AgentAdvertiserListRes, error) {
	response := &sdk.AgentAdvertiserListRes{Code: 0, Message: "success"}
	for _, id := range c.agentIDs {
		response.Data.List = append(response.Data.List, sdk.AgentAdvertiserItem{AdvertiserId: id})
	}
	response.Data.PageInfo.TotalPage = 1
	return response, nil
}

func TestOAuthAdvertiserMustMatchTenantBinding(t *testing.T) {
	tenant := tenantstore.Tenant{ID: "owner", AdvertiserID: 1001}
	if containsAdvertiser(&sdk.OauthAccessTokenResData{
		AdvertiserId:  2002,
		AdvertiserIds: []int64{2002},
	}, tenant.AdvertiserID) {
		t.Fatal("OAuth response for another advertiser was accepted")
	}
	if !containsAdvertiser(&sdk.OauthAccessTokenResData{
		AdvertiserIds: []int64{2002, 1001},
	}, tenant.AdvertiserID) {
		t.Fatal("bound advertiser in OAuth account list was rejected")
	}
}

func TestTokenPathIsPartitionedByTenant(t *testing.T) {
	factory := &TenantFactory{tokenRoot: "/srv/qianchuan/data/tenants"}
	pathA, err := factory.TokenStorePath("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	pathB, err := factory.TokenStorePath("tenant-b")
	if err != nil {
		t.Fatal(err)
	}
	if pathA == pathB {
		t.Fatalf("token paths overlap: %s", pathA)
	}
	if _, err := factory.TokenStorePath("../tenant-b"); err == nil {
		t.Fatal("path traversal tenant id was accepted")
	}
}

func TestCachedRuntimeRefreshesAfterTenantRebind(t *testing.T) {
	dataDir := t.TempDir()
	tenants, err := tenantstore.New(
		filepath.Join(dataDir, "tenants.json"),
		"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
	)
	if err != nil {
		t.Fatal(err)
	}
	factory, err := NewTenantFactory(TenantFactoryOptions{
		Client:             tenantFactoryTestClient{},
		AppID:              123,
		AppSecret:          "test-secret",
		TokenRoot:          filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState: func(string) (string, error) {
			return "state", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	first := tenantstore.Tenant{
		ID:           "reused",
		Name:         "first",
		AdvertiserID: 1001,
	}
	if err := tenants.Register(first, "first-key-012345678901234567890123456"); err != nil {
		t.Fatal(err)
	}
	firstContext, err := tenants.Get(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := factory.ToolsFor(context.Background(), firstContext); err != nil {
		t.Fatal(err)
	}
	if err := tenants.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Name = "second"
	second.AdvertiserID = 1002
	second.LiveRoomID = 9002
	if err := tenants.Register(second, "second-key-012345678901234567890123456"); err != nil {
		t.Fatal(err)
	}
	secondContext, err := tenants.Get(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := factory.ToolsFor(context.Background(), secondContext)
	if err != nil {
		t.Fatal(err)
	}
	status, err := tools.(*Tools).status(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if status.TenantName != "second" || status.AdvertiserID != "1002" {
		t.Fatalf("recreated tenant context = %#v", status)
	}
}

func TestMultipleAccountsResolvePerRequest(t *testing.T) {
	dataDir := t.TempDir()
	factory, err := NewTenantFactory(TenantFactoryOptions{
		Client:             tenantFactoryTestClient{},
		AppID:              123,
		AppSecret:          "test-secret",
		TokenRoot:          filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState: func(string) (string, error) {
			return "tenant-state", nil
		},
		IssueAccountAuthState: func(_ string, accountID int64) (string, error) {
			return "account-" + formatID(accountID), nil
		},
		OAuthAuthorizationURL: "https://example.test/oauth/authorize",
	})
	if err != nil {
		t.Fatal(err)
	}
	tenant := tenantstore.Tenant{
		ID:           "multi",
		Name:         "multi",
		AdvertiserID: 1001,
		Accounts: []tenantstore.AccountBinding{
			{AdvertiserID: 1001, AwemeID: 2001, LiveRoomID: 3001},
			{AdvertiserID: 1002, AwemeID: 2002, LiveRoomID: 3002},
		},
	}
	caller, err := factory.ToolsFor(context.Background(), &tenant)
	if err != nil {
		t.Fatal(err)
	}
	tools := caller.(*Tools)
	for _, advertiserID := range []int64{1001, 1002} {
		if err := tools.service.TokenManager.SaveToken(advertiserID, session.NewSessionFromTokenResponse(
			&session.TokenResponse{
				AccessToken:           "access-" + formatID(advertiserID),
				RefreshToken:          "refresh-" + formatID(advertiserID),
				ExpiresIn:             3600,
				RefreshTokenExpiresIn: 3600,
			},
			advertiserID,
		)); err != nil {
			t.Fatal(err)
		}
	}
	for _, account := range tenant.AccountList() {
		account := account
		status, err := tools.status(context.Background(), map[string]any{"account_id": formatID(account.AdvertiserID)})
		if err != nil {
			t.Fatal(err)
		}
		if status.AdvertiserID != formatID(account.AdvertiserID) {
			t.Fatalf("account %d status advertiser = %q", account.AdvertiserID, status.AdvertiserID)
		}
		if status.AwemeID != formatID(account.AwemeID) || status.LiveRoomID != formatID(account.LiveRoomID) {
			t.Fatalf("account %d status crossed aweme/live room: %#v", account.AdvertiserID, status)
		}
		if !status.Authenticated {
			t.Fatalf("account %d should be authenticated: %#v", account.AdvertiserID, status)
		}
	}
	unselected, err := tools.status(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !unselected.AccountSelectionRequired || unselected.AdvertiserID != "" {
		t.Fatalf("unselected multi-account status = %#v", unselected)
	}
	if _, err := tools.status(context.Background(), map[string]any{"account_id": "9999"}); !errors.Is(err, tenantstore.ErrAccountNotFound) {
		t.Fatalf("foreign account status err = %v, want ErrAccountNotFound", err)
	}
	listed := tools.listAccounts(context.Background())
	accounts, ok := listed["accounts"].([]map[string]any)
	if !ok || len(accounts) != 2 {
		t.Fatalf("listed accounts = %#v", listed["accounts"])
	}
	if listed["account_selection_required"] != true {
		t.Fatalf("selection flag = %#v", listed["account_selection_required"])
	}
}

func TestOAuthSessionOnlyPersistsSelectedAccount(t *testing.T) {
	dataDir := t.TempDir()
	factory, err := NewTenantFactory(TenantFactoryOptions{
		Client:             tenantFactoryTestClient{},
		AppID:              123,
		AppSecret:          "test-secret",
		TokenRoot:          filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState: func(string) (string, error) {
			return "state", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tenant := tenantstore.Tenant{
		ID:           "multi",
		Name:         "multi",
		AdvertiserID: 1001,
		Accounts: []tenantstore.AccountBinding{
			{AdvertiserID: 1001},
			{AdvertiserID: 1002},
		},
	}
	if err := factory.SaveOAuthSession(context.Background(), tenant, &sdk.OauthAccessTokenResData{
		AccessToken:           "access-token",
		RefreshToken:          "refresh-token",
		ExpiresIn:             3600,
		RefreshTokenExpiresIn: 3600,
		AdvertiserId:          1001,
		AdvertiserIds:         []int64{1001, 1002},
	}, 1002); err != nil {
		t.Fatal(err)
	}
	caller, err := factory.ToolsFor(context.Background(), &tenant)
	if err != nil {
		t.Fatal(err)
	}
	tools := caller.(*Tools)
	if _, err := tools.service.TokenManager.AccessToken(context.Background(), 1001); err == nil {
		t.Fatal("unselected account in the OAuth response was persisted")
	}
	if _, err := tools.service.TokenManager.AccessToken(context.Background(), 1002); err != nil {
		t.Fatalf("selected account was not persisted: %v", err)
	}
}

func TestWorkspaceRootResolvesToBoundAdvertiser(t *testing.T) {
	dataDir := t.TempDir()
	factory, err := NewTenantFactory(TenantFactoryOptions{
		Client: tenantFactoryTestClient{
			roots: []sdk.AdvertiserListItem{{
				AdvertiserId: 9001,
				AccountRole:  "CUSTOMER_ADMIN",
			}},
			workspaceIDs: []int64{1001, 2002},
		},
		AppID:              123,
		AppSecret:          "test-secret",
		TokenRoot:          filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState: func(string) (string, error) {
			return "state", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tenant := tenantstore.Tenant{
		ID:           "owner",
		Name:         "owner",
		AdvertiserID: 1001,
	}
	if err := factory.SaveOAuthSession(context.Background(), tenant, &sdk.OauthAccessTokenResData{
		AccessToken:           "access-token",
		RefreshToken:          "refresh-token",
		ExpiresIn:             3600,
		RefreshTokenExpiresIn: 3600,
		AdvertiserId:          9001,
	}, 0); err != nil {
		t.Fatal(err)
	}
	caller, err := factory.ToolsFor(context.Background(), &tenant)
	if err != nil {
		t.Fatal(err)
	}
	tools := caller.(*Tools)
	if _, err := tools.service.TokenManager.AccessToken(context.Background(), 1001); err != nil {
		t.Fatalf("bound advertiser token was not persisted: %v", err)
	}
	if _, err := tools.service.TokenManager.AccessToken(context.Background(), 2002); err == nil {
		t.Fatal("workspace child outside tenant binding was persisted")
	}
}

func TestWorkspaceRootResolvesOnlySelectedAccount(t *testing.T) {
	dataDir := t.TempDir()
	factory, err := NewTenantFactory(TenantFactoryOptions{
		Client: tenantFactoryTestClient{
			roots: []sdk.AdvertiserListItem{{
				AdvertiserId: 9001,
				AccountRole:  "CUSTOMER_OPERATOR",
			}},
			workspaceIDs: []int64{1001, 1002},
		},
		AppID:              123,
		AppSecret:          "test-secret",
		TokenRoot:          filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState: func(string) (string, error) {
			return "state", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tenant := tenantstore.Tenant{
		ID:           "owner",
		Name:         "owner",
		AdvertiserID: 1001,
		Accounts: []tenantstore.AccountBinding{
			{AdvertiserID: 1001},
			{AdvertiserID: 1002},
		},
	}
	if err := factory.SaveOAuthSession(context.Background(), tenant, &sdk.OauthAccessTokenResData{
		AccessToken:           "access-token",
		RefreshToken:          "refresh-token",
		ExpiresIn:             3600,
		RefreshTokenExpiresIn: 3600,
		AdvertiserId:          9001,
	}, 1002); err != nil {
		t.Fatal(err)
	}
	caller, err := factory.ToolsFor(context.Background(), &tenant)
	if err != nil {
		t.Fatal(err)
	}
	tools := caller.(*Tools)
	if _, err := tools.service.TokenManager.AccessToken(context.Background(), 1001); err == nil {
		t.Fatal("unselected workspace account was persisted")
	}
	if _, err := tools.service.TokenManager.AccessToken(context.Background(), 1002); err != nil {
		t.Fatalf("selected workspace account was not persisted: %v", err)
	}
}

func TestWorkspaceWithoutBoundAdvertiserIsRejected(t *testing.T) {
	dataDir := t.TempDir()
	factory, err := NewTenantFactory(TenantFactoryOptions{
		Client: tenantFactoryTestClient{
			roots: []sdk.AdvertiserListItem{{
				AdvertiserId: 9001,
				AccountRole:  "CUSTOMER_ADMIN",
			}},
			workspaceIDs: []int64{2002, 2003},
		},
		AppID:              123,
		AppSecret:          "test-secret",
		TokenRoot:          filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState: func(string) (string, error) {
			return "state", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tenant := tenantstore.Tenant{ID: "owner", Name: "owner", AdvertiserID: 1001}
	err = factory.SaveOAuthSession(context.Background(), tenant, &sdk.OauthAccessTokenResData{
		AccessToken:           "access-token",
		RefreshToken:          "refresh-token",
		ExpiresIn:             3600,
		RefreshTokenExpiresIn: 3600,
		AdvertiserId:          9001,
	}, 0)
	if !errors.Is(err, ErrTenantAccountMismatch) {
		t.Fatalf("SaveOAuthSession error = %v, want ErrTenantAccountMismatch", err)
	}
}

func TestWorkspaceListFailureFallsBackOnlyToSelectedAccount(t *testing.T) {
	dataDir := t.TempDir()
	factory, err := NewTenantFactory(TenantFactoryOptions{
		Client: tenantFactoryTestClient{
			roots: []sdk.AdvertiserListItem{{
				AdvertiserId: 9001,
				AccountRole:  "CUSTOMER_ADMIN",
			}},
			customerListCode: 50000,
		},
		AppID:              123,
		AppSecret:          "test-secret",
		TokenRoot:          filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState: func(string) (string, error) {
			return "state", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tenant := tenantstore.Tenant{
		ID:           "owner",
		Name:         "owner",
		AdvertiserID: 1001,
		Accounts: []tenantstore.AccountBinding{
			{AdvertiserID: 1001},
			{AdvertiserID: 1002},
		},
	}
	if err := factory.SaveOAuthSession(context.Background(), tenant, &sdk.OauthAccessTokenResData{
		AccessToken:           "access-token",
		RefreshToken:          "refresh-token",
		ExpiresIn:             3600,
		RefreshTokenExpiresIn: 3600,
		AdvertiserId:          9001,
	}, 1002); err != nil {
		t.Fatal(err)
	}
	caller, err := factory.ToolsFor(context.Background(), &tenant)
	if err != nil {
		t.Fatal(err)
	}
	tools := caller.(*Tools)
	if _, err := tools.service.TokenManager.AccessToken(context.Background(), 1001); err == nil {
		t.Fatal("unselected account was persisted during degraded discovery")
	}
	if _, err := tools.service.TokenManager.AccessToken(context.Background(), 1002); err != nil {
		t.Fatalf("selected account was not persisted: %v", err)
	}
}

func TestWorkspaceListFailureRejectsAmbiguousMultiAccountTenant(t *testing.T) {
	dataDir := t.TempDir()
	factory, err := NewTenantFactory(TenantFactoryOptions{
		Client: tenantFactoryTestClient{
			roots: []sdk.AdvertiserListItem{{
				AdvertiserId: 9001,
				AccountRole:  "CUSTOMER_ADMIN",
			}},
			customerListCode: 50000,
		},
		AppID:              123,
		AppSecret:          "test-secret",
		TokenRoot:          filepath.Join(dataDir, "tokens"),
		TokenEncryptionKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
		IssueAuthState: func(string) (string, error) {
			return "state", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tenant := tenantstore.Tenant{
		ID:           "owner",
		Name:         "owner",
		AdvertiserID: 1001,
		Accounts: []tenantstore.AccountBinding{
			{AdvertiserID: 1001},
			{AdvertiserID: 1002},
		},
	}
	err = factory.SaveOAuthSession(context.Background(), tenant, &sdk.OauthAccessTokenResData{
		AccessToken:           "access-token",
		RefreshToken:          "refresh-token",
		ExpiresIn:             3600,
		RefreshTokenExpiresIn: 3600,
		AdvertiserId:          9001,
	}, 0)
	if err == nil {
		t.Fatal("ambiguous multi-account tenant was accepted after discovery failed")
	}
}
