package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/CriarBrand/qianchuan-backend/internal/tokenmanager"
	"github.com/CriarBrand/qianchuan-backend/pkg/session"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
	"github.com/CriarBrand/qianchuan-backend/pkg/tokenstore"
)

var ErrTenantAccountMismatch = errors.New("OAuth account does not match tenant binding")

type TenantRuntime struct {
	service *service.QianchuanService
	tools   *Tools
}

type TenantFactory struct {
	client             sdk.QianchuanClient
	appID              int64
	appSecret          string
	tokenRoot          string
	tokenEncryptionKey string
	writeEnabled       bool
	oauthAuthorization string
	states             func(tenantID string) (string, error)
	mu                 sync.Mutex
	runtimes           map[string]*TenantRuntime
}

type TenantFactoryOptions struct {
	Client              sdk.QianchuanClient
	AppID               int64
	AppSecret           string
	TokenRoot           string
	TokenEncryptionKey  string
	WriteEnabled        bool
	OAuthAuthorizationURL string
	IssueAuthState      func(tenantID string) (string, error)
}

func NewTenantFactory(options TenantFactoryOptions) (*TenantFactory, error) {
	if options.Client == nil {
		return nil, errors.New("Qianchuan client is required")
	}
	if options.TokenRoot == "" {
		return nil, errors.New("tenant token root is required")
	}
	if options.TokenEncryptionKey == "" {
		return nil, errors.New("token encryption key is required")
	}
	if options.IssueAuthState == nil {
		return nil, errors.New("OAuth state issuer is required")
	}
	return &TenantFactory{
		client:             options.Client,
		appID:              options.AppID,
		appSecret:          options.AppSecret,
		tokenRoot:          filepath.Clean(options.TokenRoot),
		tokenEncryptionKey: options.TokenEncryptionKey,
		writeEnabled:       options.WriteEnabled,
		oauthAuthorization: options.OAuthAuthorizationURL,
		states:             options.IssueAuthState,
		runtimes:           make(map[string]*TenantRuntime),
	}, nil
}

func (f *TenantFactory) ToolsFor(_ context.Context, tenant *tenantstore.Tenant) (ToolCaller, error) {
	if tenant == nil {
		return nil, errors.New("tenant context is required")
	}
	runtime, err := f.runtimeFor(*tenant)
	if err != nil {
		return nil, err
	}
	tools := *runtime.tools
	tools.tenant = TenantIdentity{ID: tenant.ID, Name: tenant.Name}
	tools.authURLBuilder = f.authorizationURL
	return &tools, nil
}

func (f *TenantFactory) SaveOAuthSession(
	ctx context.Context,
	tenant tenantstore.Tenant,
	oauthData *sdk.OauthAccessTokenResData,
) error {
	if oauthData == nil {
		return errors.New("OAuth response is required")
	}
	if !containsAdvertiser(oauthData, tenant.AdvertiserID) {
		return ErrTenantAccountMismatch
	}
	runtime, err := f.runtimeFor(tenant)
	if err != nil {
		return err
	}
	if strings.TrimSpace(oauthData.AccessToken) == "" || strings.TrimSpace(oauthData.RefreshToken) == "" {
		return errors.New("OAuth response is missing access or refresh token")
	}
	value := session.NewSessionFromTokenResponse(&session.TokenResponse{
		AccessToken:           oauthData.AccessToken,
		RefreshToken:          oauthData.RefreshToken,
		ExpiresIn:             oauthData.ExpiresIn,
		RefreshTokenExpiresIn: oauthData.RefreshTokenExpiresIn,
	}, tenant.AdvertiserID)
	if err := runtime.service.TokenManager.SaveToken(tenant.AdvertiserID, value); err != nil {
		return fmt.Errorf("persist tenant OAuth session: %w", err)
	}
	return nil
}

func (f *TenantFactory) TokenStorePath(tenantID string) (string, error) {
	if err := validateTenantPath(tenantID); err != nil {
		return "", err
	}
	return filepath.Join(f.tokenRoot, tenantID, "tokens.json"), nil
}

func (f *TenantFactory) authorizationURL(tenantID string) (string, error) {
	state, err := f.states(tenantID)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(f.oauthAuthorization)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (f *TenantFactory) runtimeFor(tenant tenantstore.Tenant) (*TenantRuntime, error) {
	if err := validateTenantPath(tenant.ID); err != nil {
		return nil, err
	}
	tenantID := tenant.ID
	f.mu.Lock()
	defer f.mu.Unlock()
	if runtime, ok := f.runtimes[tenantID]; ok {
		return runtime, nil
	}
	path, err := f.TokenStorePath(tenantID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create tenant token directory: %w", err)
	}
	store, err := tokenstore.New(path, f.tokenEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("initialize tenant token store: %w", err)
	}
	manager, err := tokenmanager.New(f.client, store)
	if err != nil {
		return nil, fmt.Errorf("initialize tenant token manager: %w", err)
	}
	runtimeService := service.NewQianchuanService(f.client, f.appID, f.appSecret)
	runtimeService.TokenManager = manager
	runtimeService.DefaultAdvertiserID = tenant.AdvertiserID
	runtimeService.DefaultAwemeID = tenant.AwemeID
	runtimeService.DefaultLiveRoomID = tenant.LiveRoomID

	tools := NewTools(runtimeService, f.writeEnabled, "")
	tools.tenant = TenantIdentity{ID: tenantID}
	tools.authURLBuilder = f.authorizationURL
	tools.tokenStorePath = path
	runtime := &TenantRuntime{service: runtimeService, tools: tools}
	f.runtimes[tenantID] = runtime
	return runtime, nil
}

func validateTenantPath(tenantID string) error {
	if tenantID == "" || tenantID == "." || tenantID == ".." {
		return errors.New("invalid tenant id")
	}
	for _, char := range tenantID {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' {
			continue
		}
		return errors.New("invalid tenant id")
	}
	return nil
}

func containsAdvertiser(data *sdk.OauthAccessTokenResData, expected int64) bool {
	if expected <= 0 {
		return false
	}
	if data.AdvertiserId == expected {
		return true
	}
	for _, id := range data.AdvertiserIds {
		if id == expected {
			return true
		}
	}
	return false
}

func TenantTokenRoot(tokenStorePath string) string {
	if configured := filepath.Clean(tokenStorePath); configured != "" && configured != "." {
		return filepath.Join(filepath.Dir(configured), "tenants")
	}
	return filepath.Join("data", "tenants")
}

func tenantStoreExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
