package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type gateway struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func main() {
	log.SetOutput(os.Stderr)
	g, err := gatewayFromEnv(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	serve(os.Stdin, os.Stdout, g)
}

func gatewayFromEnv(getenv func(string) string) (*gateway, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(getenv("QIANCHUAN_MCP_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:18081/mcp"
	}
	apiKey := strings.TrimSpace(getenv("QIANCHUAN_TENANT_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("QIANCHUAN_TENANT_KEY is required")
	}
	return &gateway{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 40 * time.Second,
		},
	}, nil
}

func serve(input io.Reader, output io.Writer, g *gateway) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var request rpcRequest
		if err := json.Unmarshal(line, &request); err != nil {
			_ = encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "Parse error"},
			})
			continue
		}
		if len(request.ID) == 0 {
			// This transport is stateless; notifications need no response.
			continue
		}
		response, err := g.roundTrip(request)
		if err != nil {
			log.Printf("forward MCP request failed: %v", err)
			response = rpcResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Error:   &rpcError{Code: -32603, Message: "Qianchuan MCP upstream unavailable"},
			}
		}
		if len(response.ID) == 0 {
			response.ID = request.ID
		}
		if err := encoder.Encode(response); err != nil {
			log.Printf("write response failed: %v", err)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("read stdio failed: %v", err)
	}
}

func (g *gateway) roundTrip(request rpcRequest) (rpcResponse, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return rpcResponse{}, fmt.Errorf("encode JSON-RPC request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, g.baseURL, bytes.NewReader(encoded))
	if err != nil {
		return rpcResponse{}, fmt.Errorf("create Qianchuan MCP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	client := g.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return rpcResponse{}, fmt.Errorf("call Qianchuan MCP: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return rpcResponse{}, fmt.Errorf("read Qianchuan MCP response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return rpcResponse{}, fmt.Errorf("Qianchuan MCP HTTP %d: %s", resp.StatusCode, string(payload))
	}

	var response rpcResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return rpcResponse{}, fmt.Errorf("decode Qianchuan MCP response: %w", err)
	}
	return response, nil
}
