package mcp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

type tenantFactoryTestClient struct {
	sdk.QianchuanClient
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
	status := tools.(*Tools).status(context.Background())
	if status.TenantName != "second" || status.AdvertiserID != "1002" {
		t.Fatalf("recreated tenant context = %#v", status)
	}
}
