package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

func TestCreateTenantWithKeyFileRegistersUsableCredential(t *testing.T) {
	dataDir := t.TempDir()
	store, err := tenantstore.New(
		filepath.Join(dataDir, "tenants.json"),
		"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
	)
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(dataDir, "owner.key")
	err = createTenant(store, []string{
		"--id", "owner",
		"--name", "Owner",
		"--advertiser-id", "1855810750424170",
		"--aweme-id", "63307887035",
		"--live-room-id", "7709635958206991154",
		"--key-file", keyFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode = %o, want 600", info.Mode().Perm())
	}
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	tenant, err := store.Authenticate(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if tenant.ID != "owner" || tenant.AdvertiserID != 1855810750424170 {
		t.Fatalf("unexpected tenant: %#v", tenant)
	}
}
