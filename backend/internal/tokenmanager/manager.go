package tokenmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/pkg/session"
	"github.com/CriarBrand/qianchuan-backend/pkg/tokenstore"
	"golang.org/x/sync/singleflight"
)

// Manager provides encrypted OAuth persistence and refreshes expiring
// access tokens. It never exposes credentials in logs or API responses.
type Manager struct {
	client sdk.QianchuanClient
	store  *tokenstore.Store
	group  singleflight.Group
	now    func() time.Time
}

func New(client sdk.QianchuanClient, store *tokenstore.Store) (*Manager, error) {
	if client == nil {
		return nil, errors.New("qianchuan client is required")
	}
	if store == nil {
		return nil, errors.New("token store is required")
	}
	return &Manager{client: client, store: store, now: time.Now}, nil
}

// SaveToken persists a freshly exchanged OAuth session.
func (m *Manager) SaveToken(advertiserID int64, value *session.UserSession) error {
	if value == nil {
		return errors.New("session is required")
	}
	if value.AdvertiserID == 0 {
		value.AdvertiserID = advertiserID
	}
	return m.store.Save(advertiserID, value)
}

// Load returns the encrypted persisted token for an advertiser.
func (m *Manager) Load(_ context.Context, advertiserID int64) (*session.UserSession, error) {
	return m.store.Load(advertiserID)
}

// AccessToken returns a valid access token, refreshing it five minutes
// before expiry. Concurrent callers for the same account share one refresh.
func (m *Manager) AccessToken(ctx context.Context, advertiserID int64) (string, error) {
	value, err := m.store.Load(advertiserID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("该账户尚未完成授权")
		}
		return "", err
	}
	if !value.NeedsRefresh() && !value.IsExpired() {
		return value.AccessToken, nil
	}
	if value.IsRefreshExpired() {
		return "", errors.New("授权已过期，请重新登录")
	}

	key := fmt.Sprintf("token-refresh:%d", advertiserID)
	result, err, _ := m.group.Do(key, func() (interface{}, error) {
		current, loadErr := m.store.Load(advertiserID)
		if loadErr != nil {
			return "", loadErr
		}
		if !current.NeedsRefresh() && !current.IsExpired() {
			return current.AccessToken, nil
		}
		refreshed, refreshErr := m.Refresh(ctx, advertiserID)
		if refreshErr != nil {
			return "", refreshErr
		}
		return refreshed.AccessToken, nil
	})
	if err != nil {
		return "", err
	}
	token, ok := result.(string)
	if !ok || token == "" {
		return "", errors.New("刷新授权未返回 access_token")
	}
	return token, nil
}

// Refresh rotates the refresh token and immediately persists the result.
func (m *Manager) Refresh(ctx context.Context, advertiserID int64) (*session.UserSession, error) {
	current, err := m.store.Load(advertiserID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("该账户尚未完成授权")
		}
		return nil, err
	}
	if current.IsRefreshExpired() {
		return nil, errors.New("授权已过期，请重新登录")
	}
	response, err := m.client.OauthRefreshToken(ctx, sdk.OauthRefreshTokenReq{
		RefreshToken: current.RefreshToken,
	})
	if err != nil {
		return nil, err
	}
	if response == nil || response.Code != 0 || response.Data == nil {
		if response != nil && response.Message != "" {
			return nil, fmt.Errorf("刷新授权失败: %s", response.Message)
		}
		return nil, errors.New("刷新授权失败")
	}
	if response.Data.AccessToken == "" || response.Data.RefreshToken == "" {
		return nil, errors.New("刷新授权响应缺少必要凭证")
	}
	refreshed := session.NewSessionFromRefreshResponse(
		response.Data.AccessToken,
		response.Data.RefreshToken,
		response.Data.ExpiresIn,
		response.Data.RefreshTokenExpiresIn,
		advertiserID,
	)
	if err := m.store.Save(advertiserID, refreshed); err != nil {
		return nil, fmt.Errorf("持久化刷新后的授权凭证失败: %w", err)
	}
	return refreshed, nil
}
