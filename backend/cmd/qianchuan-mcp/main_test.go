package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayFromEnvRequiresCallerKey(t *testing.T) {
	_, err := gatewayFromEnv(func(key string) string {
		if key == "QIANCHUAN_MCP_BASE_URL" {
			return "https://example.test/mcp"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "QIANCHUAN_TENANT_KEY") {
		t.Fatalf("err = %v, want missing caller key error", err)
	}
}

func TestGatewayFromEnvUsesOnlyCallerCredentials(t *testing.T) {
	g, err := gatewayFromEnv(func(key string) string {
		switch key {
		case "QIANCHUAN_MCP_BASE_URL":
			return "https://example.test/mcp/"
		case "QIANCHUAN_TENANT_KEY":
			return "caller-private-key"
		case "TENANT_ID":
			return "someone-else"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.baseURL != "https://example.test/mcp" || g.apiKey != "caller-private-key" {
		t.Fatalf("unexpected gateway config: %#v", g)
	}
}

func TestServeForwardsInitializeAndToolList(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected API key: %q", got)
		}
		var request rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","serverInfo":{"name":"upstream"}}}`)
		case "tools/list":
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"qianchuan_get_status","description":"status","inputSchema":{"type":"object"}}]}}`)
		default:
			t.Fatalf("unexpected method: %s", request.Method)
		}
	}))
	defer upstream.Close()

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	serve(strings.NewReader(input), &output, &gateway{
		baseURL:    upstream.URL,
		apiKey:     "test-key",
		httpClient: upstream.Client(),
	})

	decoder := json.NewDecoder(&output)
	var initialize rpcResponse
	if err := decoder.Decode(&initialize); err != nil {
		t.Fatal(err)
	}
	if initialize.Error != nil {
		t.Fatalf("initialize failed: %#v", initialize.Error)
	}
	var list rpcResponse
	if err := decoder.Decode(&list); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(list.Result)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`qianchuan_get_status`)) {
		t.Fatalf("tool list was not forwarded: %s", encoded)
	}
}

func TestServeForwardsRemoteToolFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`)
	}))
	defer upstream.Close()

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"missing","arguments":{}}}` + "\n"
	var output bytes.Buffer
	serve(strings.NewReader(input), &output, &gateway{
		baseURL:    upstream.URL,
		apiKey:     "test-key",
		httpClient: upstream.Client(),
	})

	var response rpcResponse
	if err := json.NewDecoder(&output).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error == nil || response.Error.Code != -32601 {
		t.Fatalf("remote error was not preserved: %#v", response)
	}
}
