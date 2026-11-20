package mcp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	service  *service.QianchuanService
	tools    *Tools
	accounts []tenantstore.AccountBinding
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
	accountStates      func(tenantID string, accountID int64) (string, error)
	mu                 sync.Mutex
	runtimes           map[string]*TenantRuntime
}

type TenantFactoryOptions struct {
	Client                sdk.QianchuanClient
	AppID                 int64
	AppSecret             string
	TokenRoot             string
	TokenEncryptionKey    string
	WriteEnabled          bool
	OAuthAuthorizationURL string
	IssueAuthState        func(tenantID string) (string, error)
	IssueAccountAuthState func(tenantID string, accountID int64) (string, error)
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
	if options.IssueAuthState == nil && options.IssueAccountAuthState == nil {
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
		accountStates:      options.IssueAccountAuthState,
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
	tools.accounts = tenant.AccountList()
	tools.authURLBuilder = f.authorizationURL
	return &tools, nil
}

func (f *TenantFactory) SaveOAuthSession(
	ctx context.Context,
	tenant tenantstore.Tenant,
	oauthData *sdk.OauthAccessTokenResData,
	accountID int64,
) error {
	if oauthData == nil {
		return errors.New("OAuth response is required")
	}
	if strings.TrimSpace(oauthData.AccessToken) == "" || strings.TrimSpace(oauthData.RefreshToken) == "" {
		return errors.New("OAuth response is missing access or refresh token")
	}
	matchedAccounts := matchingAdvertisers(oauthData, tenant.AccountList(), accountID)
	if len(matchedAccounts) == 0 {
		var err error
		matchedAccounts, err = f.resolveOAuthAdvertisers(
			ctx,
			oauthData.AccessToken,
			tenant.AccountList(),
			accountID,
		)
		if err != nil {
			fallbackAccounts := explicitOAuthAccounts(tenant.AccountList(), accountID)
			if len(fallbackAccounts) == 0 {
				return err
			}
			log.Printf(
				"MCP OAuth account discovery warning: %v; persisting explicitly bound advertiser %d",
				err,
				fallbackAccounts[0],
			)
			matchedAccounts = fallbackAccounts
		}
	}
	if len(matchedAccounts) == 0 {
		return ErrTenantAccountMismatch
	}
	runtime, err := f.runtimeFor(tenant)
	if err != nil {
		return err
	}
	for _, advertiserID := range matchedAccounts {
		value := session.NewSessionFromTokenResponse(&session.TokenResponse{
			AccessToken:           oauthData.AccessToken,
			RefreshToken:          oauthData.RefreshToken,
			ExpiresIn:             oauthData.ExpiresIn,
			RefreshTokenExpiresIn: oauthData.RefreshTokenExpiresIn,
		}, advertiserID)
		if err := runtime.service.TokenManager.SaveToken(advertiserID, value); err != nil {
			return fmt.Errorf("persist tenant OAuth session for %d: %w", advertiserID, err)
		}
	}
	return nil
}

// explicitOAuthAccounts limits degraded discovery to an account explicitly
// selected by state, or the tenant's sole bound account. Ambiguous
// multi-account tenants still fail closed when discovery is unavailable.
func explicitOAuthAccounts(accounts []tenantstore.AccountBinding, accountID int64) []int64 {
	if accountID > 0 {
		for _, account := range accounts {
			if account.AdvertiserID == accountID {
				return []int64{accountID}
			}
		}
		return nil
	}
	if len(accounts) == 1 {
		return []int64{accounts[0].AdvertiserID}
	}
	return nil
}

// resolveOAuthAdvertisers maps the workspace, shop, or agency root account
// returned by OAuth to the advertiser accounts bound to this tenant.
func (f *TenantFactory) resolveOAuthAdvertisers(
	ctx context.Context,
	accessToken string,
	accounts []tenantstore.AccountBinding,
	accountID int64,
) ([]int64, error) {
	desired := make(map[int64]bool, len(accounts))
	for _, account := range accounts {
		if accountID > 0 && account.AdvertiserID != accountID {
			continue
		}
		desired[account.AdvertiserID] = true
	}
	if len(desired) == 0 {
		return nil, ErrTenantAccountMismatch
	}

	response, err := f.client.AdvertiserList(ctx, sdk.AdvertiserListReq{
		AccessToken: accessToken,
		AppId:       f.appID,
		Secret:      f.appSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("list OAuth root accounts: %w", err)
	}
	if response == nil || response.Code != 0 {
		if response != nil {
			return nil, fmt.Errorf("list OAuth root accounts[%d]: %s", response.Code, response.Message)
		}
		return nil, errors.New("list OAuth root accounts returned an empty response")
	}

	found := make(map[int64]bool, len(desired))
	for _, root := range response.Data.List {
		rootID := root.AdvertiserId
		if rootID <= 0 {
			continue
		}
		if desired[rootID] {
			found[rootID] = true
		}
		if len(found) == len(desired) {
			break
		}

		var children []int64
		switch root.AccountRole {
		case "CUSTOMER_ADMIN", "CUSTOMER_OPERATOR":
			children, err = f.customerCenterAdvertisers(ctx, accessToken, rootID)
		case "PLATFORM_ROLE_SHOP_ACCOUNT":
			children, err = f.shopAdvertisers(ctx, accessToken, rootID)
		case "PLATFORM_ROLE_QIANCHUAN_AGENT", "AGENT", "CHILD_AGENT", "AGENT_SYSTEM_ACCOUNT":
			children, err = f.agentAdvertisers(ctx, accessToken, rootID)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, childID := range children {
			if desired[childID] {
				found[childID] = true
			}
		}
		if len(found) == len(desired) {
			break
		}
	}

	matched := make([]int64, 0, len(found))
	for _, account := range accounts {
		if accountID > 0 && account.AdvertiserID != accountID {
			continue
		}
		if found[account.AdvertiserID] {
			matched = append(matched, account.AdvertiserID)
		}
	}
	return matched, nil
}

func (f *TenantFactory) customerCenterAdvertisers(
	ctx context.Context,
	accessToken string,
	rootID int64,
) ([]int64, error) {
	var result []int64
	for page := int64(1); page <= 100; page++ {
		response, err := f.client.CustomerCenterAdvertiserList(ctx, sdk.CustomerCenterAdvertiserListReq{
			AccessToken:    accessToken,
			CustomerCenter: rootID,
			Page:           page,
			PageSize:       100,
		})
		if err != nil {
			return nil, fmt.Errorf("list workspace advertiser accounts: %w", err)
		}
		if response == nil || response.Code != 0 {
			if response != nil {
				return nil, fmt.Errorf("list workspace advertiser accounts[%d]: %s", response.Code, response.Message)
			}
			return nil, errors.New("list workspace advertiser accounts returned an empty response")
		}
		for _, item := range response.Data.List {
			if item.AdvertiserId > 0 {
				result = append(result, item.AdvertiserId)
			}
		}
		if len(response.Data.List) < 100 || response.Data.PageInfo.TotalPage <= int(page) {
			break
		}
	}
	return result, nil
}

func (f *TenantFactory) shopAdvertisers(
	ctx context.Context,
	accessToken string,
	rootID int64,
) ([]int64, error) {
	var result []int64
	for page := int64(1); page <= 100; page++ {
		response, err := f.client.ShopAdvertiserList(ctx, sdk.ShopAdvertiserListReq{
			AccessToken: accessToken,
			ShopId:      rootID,
			Page:        page,
			PageSize:    100,
		})
		if err != nil {
			return nil, fmt.Errorf("list shop advertiser accounts: %w", err)
		}
		if response == nil || response.Code != 0 {
			if response != nil {
				return nil, fmt.Errorf("list shop advertiser accounts[%d]: %s", response.Code, response.Message)
			}
			return nil, errors.New("list shop advertiser accounts returned an empty response")
		}
		for _, item := range response.Data.List {
			if item.AdvertiserId > 0 {
				result = append(result, item.AdvertiserId)
			}
		}
		if len(response.Data.List) < 100 || response.Data.PageInfo.TotalPage <= int(page) {
			break
		}
	}
	return result, nil
}

func (f *TenantFactory) agentAdvertisers(
	ctx context.Context,
	accessToken string,
	rootID int64,
) ([]int64, error) {
	var result []int64
	for page := int64(1); page <= 100; page++ {
		response, err := f.client.AgentAdvertiserList(ctx, sdk.AgentAdvertiserListReq{
			AccessToken:  accessToken,
			AdvertiserId: rootID,
			Page:         page,
			PageSize:     100,
		})
		if err != nil {
			return nil, fmt.Errorf("list agent advertiser accounts: %w", err)
		}
		if response == nil || response.Code != 0 {
			if response != nil {
				return nil, fmt.Errorf("list agent advertiser accounts[%d]: %s", response.Code, response.Message)
			}
			return nil, errors.New("list agent advertiser accounts returned an empty response")
		}
		for _, item := range response.Data.List {
			if item.AdvertiserId > 0 {
				result = append(result, item.AdvertiserId)
			}
		}
		if len(response.Data.List) < 100 || response.Data.PageInfo.TotalPage <= int(page) {
			break
		}
	}
	return result, nil
}

func (f *TenantFactory) TokenStorePath(tenantID string) (string, error) {
	if err := validateTenantPath(tenantID); err != nil {
		return "", err
	}
	return filepath.Join(f.tokenRoot, tenantID, "tokens.json"), nil
}

func (f *TenantFactory) authorizationURL(selector string) (string, error) {
	var state string
	var err error
	tenantID, accountID := selector, int64(0)
	if separator := strings.LastIndex(selector, "|"); separator > 0 {
		tenantID = selector[:separator]
		accountID, err = strconv.ParseInt(selector[separator+1:], 10, 64)
		if err != nil || accountID <= 0 {
			return "", errors.New("invalid account selector")
		}
	}
	if err := validateTenantPath(tenantID); err != nil {
		return "", err
	}
	if accountID > 0 {
		if f.accountStates == nil {
			return "", errors.New("account-scoped OAuth state issuer is unavailable")
		}
		state, err = f.accountStates(tenantID, accountID)
	} else if f.states != nil {
		state, err = f.states(tenantID)
	} else {
		err = errors.New("OAuth state issuer is unavailable")
	}
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
		if sameAccounts(runtime.accounts, tenant.AccountList()) {
			return runtime, nil
		}
		delete(f.runtimes, tenantID)
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
	tools := NewTools(runtimeService, f.writeEnabled, "")
	tools.tenant = TenantIdentity{ID: tenantID}
	tools.accounts = tenant.AccountList()
	tools.authURLBuilder = f.authorizationURL
	tools.tokenStorePath = path
	runtime := &TenantRuntime{
		service:  runtimeService,
		tools:    tools,
		accounts: tenant.AccountList(),
	}
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
	return len(matchingAdvertisers(data, []tenantstore.AccountBinding{{AdvertiserID: expected}}, 0)) > 0
}

func matchingAdvertisers(
	data *sdk.OauthAccessTokenResData,
	accounts []tenantstore.AccountBinding,
	accountID int64,
) []int64 {
	if data == nil {
		return nil
	}
	returned := make(map[int64]bool)
	if data.AdvertiserId > 0 {
		returned[data.AdvertiserId] = true
	}
	for _, id := range data.AdvertiserIds {
		if id > 0 {
			returned[id] = true
		}
	}
	matched := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		if accountID > 0 && account.AdvertiserID != accountID {
			continue
		}
		if returned[account.AdvertiserID] {
			matched = append(matched, account.AdvertiserID)
		}
	}
	return matched
}

func sameAccounts(first, second []tenantstore.AccountBinding) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
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
