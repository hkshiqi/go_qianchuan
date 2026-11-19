package authstate

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

var (
	ErrInvalidState = errors.New("OAuth state is invalid or expired")
)

type entry struct {
	tenantID  string
	accountID int64
	expiresAt time.Time
}

type Store struct {
	mu      sync.Mutex
	entries map[string]entry
	ttl     time.Duration
	now     func() time.Time
}

func New(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return newStoreWithClock(ttl, time.Now)
}

func newStoreWithClock(ttl time.Duration, now func() time.Time) *Store {
	return &Store{
		entries: make(map[string]entry),
		ttl:     ttl,
		now:     now,
	}
}

func (s *Store) Issue(tenantID string) (string, error) {
	return s.IssueForAccount(tenantID, 0)
}

func (s *Store) IssueForAccount(tenantID string, accountID int64) (string, error) {
	if err := (tenantstore.Tenant{ID: tenantID, Name: tenantID, AdvertiserID: 1}).Validate(); err != nil {
		return "", err
	}
	if accountID < 0 {
		return "", errors.New("account id must be positive")
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(nonce)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, value := range s.entries {
		if !now.Before(value.expiresAt) {
			delete(s.entries, key)
		}
	}
	s.entries[state] = entry{tenantID: tenantID, accountID: accountID, expiresAt: now.Add(s.ttl)}
	return state, nil
}

func (s *Store) Consume(state string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, exists := s.entries[state]
	if !exists {
		return "", ErrInvalidState
	}
	delete(s.entries, state)
	if !s.now().Before(value.expiresAt) {
		return "", ErrInvalidState
	}
	return value.tenantID, nil
}

func (s *Store) Peek(state string) (string, error) {
	tenantID, _, err := s.PeekAccount(state)
	return tenantID, err
}

func (s *Store) PeekAccount(state string) (string, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, exists := s.entries[state]
	if !exists || !s.now().Before(value.expiresAt) {
		if exists {
			delete(s.entries, state)
		}
		return "", 0, ErrInvalidState
	}
	return value.tenantID, value.accountID, nil
}

// Discard invalidates a state even when the OAuth callback failed.
func (s *Store) Discard(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, state)
}
