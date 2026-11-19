package mcp

import (
	"testing"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

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
