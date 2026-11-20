package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

func TestServerRequiresBearerKey(t *testing.T) {
	store := testTenantStore(t)
	server := NewServer(store, tenantTestTools{})
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
	))
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestInitializeAndToolsList(t *testing.T) {
	store := testTenantStore(t)
	registerTestTenant(t, store, "tenant-a", 1001, "tenant-a-key-012345678901234567890123")
	server := NewServer(store, tenantTestTools{})
	step := func(payload string) rpcResponse {
		request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(payload))
		request.Header.Set("Authorization", "Bearer tenant-a-key-012345678901234567890123")
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var response rpcResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	initialize := step(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	if initialize.Error != nil {
		t.Fatalf("initialize error: %#v", initialize.Error)
	}
	list := step(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if list.Error != nil {
		t.Fatalf("tools/list error: %#v", list.Error)
	}
	encoded, err := json.Marshal(list.Result)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"qianchuan_get_status",
		"qianchuan_list_accounts",
		"qianchuan_list_plans",
		"qianchuan_live_overview",
		"qianchuan_evaluate_rules",
		"qianchuan_rotate_materials",
		"qianchuan_set_plan_budget",
		"qianchuan_list_available_products",
		"qianchuan_create_test_product_plan",
		"qianchuan_rename_test_product_plan",
		"qianchuan_delete_test_product_plan",
	} {
		if !bytes.Contains(encoded, []byte(`"`+name+`"`)) {
			t.Errorf("missing tool %s", name)
		}
	}
}

func TestBearerKeysResolveToIsolatedTenantContext(t *testing.T) {
	store := testTenantStore(t)
	registerTestTenant(t, store, "tenant-a", 1001, "tenant-a-key-012345678901234567890123")
	registerTestTenant(t, store, "tenant-b", 1002, "tenant-b-key-012345678901234567890123")
	server := NewServer(store, tenantTestTools{})

	callStatus := func(key string) Status {
		request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"qianchuan_get_status","arguments":{}}}`,
		))
		request.Header.Set("Authorization", "Bearer "+key)
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var envelope struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope.Result.Content) != 1 {
			t.Fatalf("unexpected result: %s", recorder.Body.String())
		}
		var status Status
		if err := json.Unmarshal([]byte(envelope.Result.Content[0].Text), &status); err != nil {
			t.Fatal(err)
		}
		return status
	}

	statusA := callStatus("tenant-a-key-012345678901234567890123")
	statusB := callStatus("tenant-b-key-012345678901234567890123")
	if statusA.TenantID != "tenant-a" || statusA.AdvertiserID != "1001" {
		t.Fatalf("tenant A context = %#v", statusA)
	}
	if statusB.TenantID != "tenant-b" || statusB.AdvertiserID != "1002" {
		t.Fatalf("tenant B context = %#v", statusB)
	}
	if statusA.AdvertiserID == statusB.AdvertiserID {
		t.Fatal("tenant contexts leaked into each other")
	}
}

func TestToolArgumentsCannotSwitchAccountContext(t *testing.T) {
	store := testTenantStore(t)
	registerTestTenant(t, store, "tenant-a", 1001, "tenant-a-key-012345678901234567890123")
	registerTestTenant(t, store, "tenant-b", 1002, "tenant-b-key-012345678901234567890123")
	server := NewServer(store, tenantTestTools{})

	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"qianchuan_get_status","arguments":{"advertiser_id":"1002"}}}`,
	))
	request.Header.Set("Authorization", "Bearer tenant-a-key-012345678901234567890123")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Result.IsError {
		t.Fatalf("account-switching argument was accepted: %s", recorder.Body.String())
	}
}

func TestToolArgumentsCannotSwitchWithSecretAliases(t *testing.T) {
	store := testTenantStore(t)
	registerTestTenant(t, store, "tenant-a", 1001, "tenant-a-key-012345678901234567890123")
	server := NewServer(store, tenantTestTools{})

	for _, arguments := range []string{
		`{"api_key":"another-caller-key"}`,
		`{"filtering":{"apiKey":"another-caller-key"}}`,
		`{"filtering":[{"tenantId":"tenant-b"}]}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"qianchuan_get_status","arguments":`+arguments+`}}`,
		))
		request.Header.Set("Authorization", "Bearer tenant-a-key-012345678901234567890123")
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte(`"isError":true`)) {
			t.Fatalf("credential alias was accepted for %s: status=%d body=%s", arguments, recorder.Code, recorder.Body.String())
		}
	}
}

func TestAccountSelectionIsAllowedButForeignTenantContextIsNot(t *testing.T) {
	store := testTenantStore(t)
	registerTestTenant(t, store, "tenant-a", 1001, "tenant-a-key-012345678901234567890123")
	server := NewServer(store, tenantTestTools{})

	call := func(arguments string) bool {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"qianchuan_get_status","arguments":`+arguments+`}}`,
		))
		request.Header.Set("Authorization", "Bearer tenant-a-key-012345678901234567890123")
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var envelope struct {
			Result struct {
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Result.IsError
	}
	if call(`{"account_id":"1001"}`) {
		t.Fatal("account_id within the bearer tenant was rejected")
	}
	if !call(`{"tenant_id":"tenant-b"}`) {
		t.Fatal("tenant_id argument was accepted")
	}
	if !call(`{"advertiser_id":"1001"}`) {
		t.Fatal("advertiser_id argument was accepted")
	}
}

func TestNestedToolArgumentsCannotSwitchAccountContext(t *testing.T) {
	store := testTenantStore(t)
	registerTestTenant(t, store, "tenant-a", 1001, "tenant-a-key-012345678901234567890123")
	registerTestTenant(t, store, "tenant-b", 1002, "tenant-b-key-012345678901234567890123")
	server := NewServer(store, tenantTestTools{})

	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"qianchuan_get_status","arguments":{"filtering":{"advertiser_id":"1002"}}}}`,
	))
	request.Header.Set("Authorization", "Bearer tenant-a-key-012345678901234567890123")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Result.IsError {
		t.Fatalf("nested account-switching argument was accepted: %s", recorder.Body.String())
	}
}

func TestProtectedMaterialSelection(t *testing.T) {
	materials := []MaterialMetrics{
		{MaterialID: "3", Cost: 300},
		{MaterialID: "1", Cost: 100},
		{MaterialID: "2", Cost: 200},
		{MaterialID: "4", Cost: 50},
	}
	for index := range materials {
		materials[index].CostRank = index + 1
		materials[index].ProtectedTop3 = index < 3
	}
	protected := protectedTop3(materials)
	if len(protected) != 3 {
		t.Fatalf("protected = %d, want 3", len(protected))
	}
	for _, item := range protected {
		if item.MaterialID == "4" {
			t.Fatal("material 4 must be eligible for replacement")
		}
	}
}

type tenantTestTools struct{}

func (tenantTestTools) ToolsFor(_ context.Context, tenant *tenantstore.Tenant) (ToolCaller, error) {
	return statusCaller{tenant: *tenant}, nil
}

type statusCaller struct {
	tenant tenantstore.Tenant
}

func (c statusCaller) Call(_ context.Context, _ string, _ map[string]any) (any, error) {
	return Status{
		TenantID:       c.tenant.ID,
		TenantName:     c.tenant.Name,
		AdvertiserID:   formatID(c.tenant.AdvertiserID),
		AwemeID:        formatID(c.tenant.AwemeID),
		LiveRoomID:     formatID(c.tenant.LiveRoomID),
		ReadEnabled:    true,
		WriteEnabled:   false,
		TokenEncrypted: true,
	}, nil
}

func testTenantStore(t *testing.T) *tenantstore.Store {
	t.Helper()
	store, err := tenantstore.New(
		filepath.Join(t.TempDir(), "tenants.json"),
		"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY",
	)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func registerTestTenant(t *testing.T, store *tenantstore.Store, id string, advertiserID int64, key string) {
	t.Helper()
	if err := store.Register(tenantstore.Tenant{
		ID:           id,
		Name:         id,
		AdvertiserID: advertiserID,
		AwemeID:      63307887035,
		LiveRoomID:   7709635958206991154,
	}, key); err != nil {
		t.Fatal(err)
	}
}

func TestStringIDListPreservesLargeIDs(t *testing.T) {
	ids, err := stringIDList([]any{"1855810750424170", float64(7709635958206991154)})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "1855810750424170" {
		t.Fatalf("unexpected IDs: %#v", ids)
	}
}
