package tenantstore

import (
	"errors"
	"testing"
)

func TestRegisterRejectsKeyAlreadyMappedToAnotherTenant(t *testing.T) {
	store := testStore(t)
	rawKey := "tenant-a-key-012345678901234567890123"

	if err := store.Register(testTenant("tenant-a", 1001), rawKey); err != nil {
		t.Fatal(err)
	}
	err := store.Register(testTenant("tenant-b", 1002), rawKey)
	if !errors.Is(err, ErrTenantExists) {
		t.Fatalf("err = %v, want ErrTenantExists", err)
	}
	if _, err := store.Get("tenant-b"); err == nil {
		t.Fatal("tenant-b was partially registered after key collision failure")
	}
}
