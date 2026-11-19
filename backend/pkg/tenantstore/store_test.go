package tenantstore

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	key := base64.RawStdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	store, err := New(filepath.Join(t.TempDir(), "tenants.json"), key)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func testTenant(id string, advertiserID int64) Tenant {
	return Tenant{
		ID:           id,
		Name:         id,
		AdvertiserID: advertiserID,
		AwemeID:      63307887035,
		LiveRoomID:   7709635958206991154,
	}
}

func TestAuthenticateResolvesDifferentTenants(t *testing.T) {
	store := testStore(t)
	if err := store.Register(testTenant("tenant-a", 1001), "tenant-a-key-012345678901234567890123"); err != nil {
		t.Fatal(err)
	}
	if err := store.Register(testTenant("tenant-b", 1002), "tenant-b-key-012345678901234567890123"); err != nil {
		t.Fatal(err)
	}

	tenantA, err := store.Authenticate("tenant-a-key-012345678901234567890123")
	if err != nil {
		t.Fatal(err)
	}
	tenantB, err := store.Authenticate("tenant-b-key-012345678901234567890123")
	if err != nil {
		t.Fatal(err)
	}
	if tenantA.ID != "tenant-a" || tenantB.ID != "tenant-b" {
		t.Fatalf("resolved tenants = %q, %q", tenantA.ID, tenantB.ID)
	}
	if tenantA.AdvertiserID == tenantB.AdvertiserID {
		t.Fatal("different tenant keys resolved to the same account")
	}
}

func TestAdvertiserCannotBeBoundToTwoTenants(t *testing.T) {
	store := testStore(t)
	if err := store.Register(testTenant("tenant-a", 1001), "tenant-a-key-012345678901234567890123"); err != nil {
		t.Fatal(err)
	}
	err := store.Register(testTenant("tenant-b", 1001), "tenant-b-key-012345678901234567890123")
	if !errors.Is(err, ErrBindingInUse) {
		t.Fatalf("err = %v, want ErrBindingInUse", err)
	}
}

func TestDeleteRevokesKeyAndReleasesAdvertiser(t *testing.T) {
	store := testStore(t)
	rawKey := "tenant-a-key-012345678901234567890123"
	if err := store.Register(testTenant("tenant-a", 1001), rawKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("tenant-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(rawKey); err == nil {
		t.Fatal("deleted tenant API key still authenticated")
	}
	if err := store.Register(testTenant("tenant-b", 1001), "tenant-b-key-012345678901234567890123"); err != nil {
		t.Fatalf("released advertiser could not be rebound: %v", err)
	}
}

func TestStoreDoesNotContainPlaintextAPIKey(t *testing.T) {
	store := testStore(t)
	rawKey := "tenant-a-key-012345678901234567890123"
	if err := store.Register(testTenant("tenant-a", 1001), rawKey); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" {
		t.Fatal("empty tenant store")
	}
	if strings.Contains(string(raw), rawKey) {
		t.Fatalf("tenant store contains plaintext API key")
	}
}

func TestTenantSupportsMultipleAccountsWithLegacyFields(t *testing.T) {
	store := testStore(t)
	tenant := Tenant{
		ID:           "multi",
		Name:         "multi",
		AdvertiserID: 1001,
		AwemeID:      2001,
		LiveRoomID:   3001,
		Accounts: []AccountBinding{
			{AdvertiserID: 1001, AwemeID: 2001, LiveRoomID: 3001},
			{AdvertiserID: 1002, AwemeID: 2002, LiveRoomID: 3002},
		},
	}
	if err := store.Register(tenant, "multi-key-012345678901234567890123456"); err != nil {
		t.Fatal(err)
	}
	resolved, err := store.Authenticate("multi-key-012345678901234567890123456")
	if err != nil {
		t.Fatal(err)
	}
	accounts := resolved.AccountList()
	if len(accounts) != 2 || accounts[1].AdvertiserID != 1002 {
		t.Fatalf("accounts = %#v", accounts)
	}
	if _, err := resolved.ResolveAccount(""); !errors.Is(err, ErrAccountRequired) {
		t.Fatalf("empty account selection err = %v, want ErrAccountRequired", err)
	}
	selected, err := resolved.ResolveAccount("1002")
	if err != nil {
		t.Fatal(err)
	}
	if selected.AwemeID != 2002 || selected.LiveRoomID != 3002 {
		t.Fatalf("selected account = %#v", selected)
	}
	if _, err := resolved.ResolveAccount("9999"); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("foreign account err = %v, want ErrAccountNotFound", err)
	}
}

func TestAnyAccountCannotBeBoundToTwoTenants(t *testing.T) {
	store := testStore(t)
	first := Tenant{
		ID:           "tenant-a",
		Name:         "tenant-a",
		AdvertiserID: 1001,
		Accounts: []AccountBinding{
			{AdvertiserID: 1001},
			{AdvertiserID: 1002},
		},
	}
	if err := store.Register(first, "tenant-a-key-012345678901234567890123"); err != nil {
		t.Fatal(err)
	}
	second := Tenant{
		ID:           "tenant-b",
		Name:         "tenant-b",
		AdvertiserID: 1003,
		Accounts: []AccountBinding{
			{AdvertiserID: 1003},
			{AdvertiserID: 1002},
		},
	}
	if err := store.Register(second, "tenant-b-key-012345678901234567890123"); !errors.Is(err, ErrBindingInUse) {
		t.Fatalf("err = %v, want ErrBindingInUse", err)
	}
}

func TestRemoveAccountReleasesOnlySelectedBinding(t *testing.T) {
	store := testStore(t)
	tenant := Tenant{
		ID:           "multi",
		Name:         "multi",
		AdvertiserID: 1001,
		Accounts: []AccountBinding{
			{AdvertiserID: 1001},
			{AdvertiserID: 1002},
		},
	}
	if err := store.Register(tenant, "multi-key-012345678901234567890123456"); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveAccount("multi", 1002); err != nil {
		t.Fatal(err)
	}
	current, err := store.Get("multi")
	if err != nil {
		t.Fatal(err)
	}
	if len(current.AccountList()) != 1 || current.AdvertiserID != 1001 {
		t.Fatalf("remaining accounts = %#v", current.AccountList())
	}
	if err := store.RemoveAccount("multi", 1001); !errors.Is(err, ErrLastAccount) {
		t.Fatalf("last account err = %v, want ErrLastAccount", err)
	}
	replacement := testTenant("replacement", 1002)
	if err := store.Register(replacement, "replacement-key-01234567890123456"); err != nil {
		t.Fatalf("removed account was not released: %v", err)
	}
}
