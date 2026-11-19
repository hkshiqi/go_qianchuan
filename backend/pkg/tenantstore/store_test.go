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
