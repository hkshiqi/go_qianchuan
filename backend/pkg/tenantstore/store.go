package tenantstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrBindingInUse    = errors.New("advertiser is already bound to another tenant")
	ErrTenantExists    = errors.New("tenant already exists")
	ErrAccountRequired = errors.New("account_id is required when the tenant has multiple accounts")
	ErrAccountNotFound = errors.New("account_id is not bound to this tenant")
	ErrLastAccount     = errors.New("tenant must keep at least one account")
)

type AccountBinding struct {
	AdvertiserID int64 `json:"advertiser_id"`
	AwemeID      int64 `json:"aweme_id"`
	LiveRoomID   int64 `json:"live_room_id"`
}

type Tenant struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	KeyHash      string           `json:"key_hash"`
	AdvertiserID int64            `json:"advertiser_id"`
	AwemeID      int64            `json:"aweme_id"`
	LiveRoomID   int64            `json:"live_room_id"`
	Accounts     []AccountBinding `json:"accounts,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

func (t Tenant) Validate() error {
	if !validTenantID(t.ID) {
		return errors.New("tenant id must contain only letters, digits, underscore or hyphen")
	}
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("tenant name is required")
	}
	if len(t.AccountList()) == 0 {
		return errors.New("at least one advertiser account is required")
	}
	seen := make(map[int64]bool)
	for _, account := range t.Accounts {
		if account.AdvertiserID <= 0 {
			return errors.New("advertiser id must be positive")
		}
		if seen[account.AdvertiserID] {
			return fmt.Errorf("duplicate advertiser id %d", account.AdvertiserID)
		}
		seen[account.AdvertiserID] = true
	}
	return nil
}

// AccountList returns the normalized account bindings while accepting the
// legacy single-account fields stored by version 2 tenants.
func (t Tenant) AccountList() []AccountBinding {
	result := make([]AccountBinding, 0, len(t.Accounts)+1)
	seen := make(map[int64]bool)
	appendAccount := func(account AccountBinding) {
		if account.AdvertiserID <= 0 || seen[account.AdvertiserID] {
			return
		}
		seen[account.AdvertiserID] = true
		result = append(result, account)
	}
	for _, account := range t.Accounts {
		appendAccount(account)
	}
	appendAccount(AccountBinding{
		AdvertiserID: t.AdvertiserID,
		AwemeID:      t.AwemeID,
		LiveRoomID:   t.LiveRoomID,
	})
	return result
}

func (t Tenant) ResolveAccount(accountID string) (AccountBinding, error) {
	accounts := t.AccountList()
	if strings.TrimSpace(accountID) == "" {
		if len(accounts) == 1 {
			return accounts[0], nil
		}
		if len(accounts) == 0 {
			return AccountBinding{}, ErrAccountNotFound
		}
		return AccountBinding{}, ErrAccountRequired
	}
	for _, account := range accounts {
		if fmt.Sprintf("%d", account.AdvertiserID) == strings.TrimSpace(accountID) {
			return account, nil
		}
	}
	return AccountBinding{}, ErrAccountNotFound
}

type fileData struct {
	Version  int                        `json:"version"`
	Tenants  map[string]encryptedTenant `json:"tenants"`
	Index    map[string]string          `json:"index"`
	Bindings map[string]string         `json:"bindings"`
}

type encryptedTenant struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type Store struct {
	path    string
	aead    cipher.AEAD
	keyHash []byte
	mu      sync.RWMutex
}

func New(path, encryptionKey string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("TENANT_STORE_PATH is required")
	}
	key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(encryptionKey))
	if err != nil || len(key) != 32 {
		return nil, errors.New("TENANT_ENCRYPTION_KEY must be 32 bytes encoded as base64 without padding")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create tenant cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create tenant GCM: %w", err)
	}
	store := &Store{
		path:    filepath.Clean(path),
		aead:    aead,
		keyHash: append([]byte{}, key...),
	}
	if err := store.ensureFile(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Register(tenant Tenant, rawKey string) error {
	if err := tenant.Validate(); err != nil {
		return err
	}
	if len(strings.TrimSpace(rawKey)) < 32 {
		return errors.New("an API key of at least 32 characters is required")
	}
	tenant.KeyHash = s.hashKey(rawKey)
	now := time.Now().UTC()
	if tenant.CreatedAt.IsZero() {
		tenant.CreatedAt = now
	}
	tenant.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	if _, exists := data.Tenants[tenant.ID]; exists {
		return ErrTenantExists
	}
	for existingID := range data.Tenants {
		current, openErr := s.openTenant(existingID, data)
		if openErr != nil {
			return openErr
		}
		for _, account := range tenant.AccountList() {
			for _, existing := range current.AccountList() {
				if current.ID != tenant.ID && existing.AdvertiserID == account.AdvertiserID {
					return ErrBindingInUse
				}
			}
		}
	}
	encoded, err := s.sealTenant(tenant)
	if err != nil {
		return err
	}
	if owner, exists := data.Index[tenant.KeyHash]; exists && owner != tenant.ID {
		return ErrTenantExists
	}
	data.Tenants[tenant.ID] = encoded
	data.Index[tenant.KeyHash] = tenant.ID
	for _, account := range tenant.AccountList() {
		data.Bindings[bindingKey(account.AdvertiserID)] = tenant.ID
	}
	return s.write(data)
}

func (s *Store) Authenticate(rawKey string) (*Tenant, error) {
	if strings.TrimSpace(rawKey) == "" {
		return nil, os.ErrPermission
	}
	hash := s.hashKey(rawKey)
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	tenantID, ok := data.Index[hash]
	if !ok {
		return nil, os.ErrPermission
	}
	storedHash := ""
	if encoded, exists := data.Tenants[tenantID]; exists {
		if tenant, openErr := s.openTenant(tenantID, data); openErr == nil {
			storedHash = tenant.KeyHash
		}
		_ = encoded
	}
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(hash)) != 1 {
		return nil, os.ErrPermission
	}
	return s.openTenant(tenantID, data)
}

func (s *Store) Get(tenantID string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	return s.openTenant(tenantID, data)
}

func (s *Store) Delete(tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	tenant, err := s.openTenant(tenantID, data)
	if err != nil {
		return err
	}
	delete(data.Tenants, tenantID)
	delete(data.Index, tenant.KeyHash)
	for _, account := range tenant.AccountList() {
		if data.Bindings[bindingKey(account.AdvertiserID)] == tenantID {
			delete(data.Bindings, bindingKey(account.AdvertiserID))
		}
	}
	return s.write(data)
}

func (s *Store) UpdateBinding(tenantID string, advertiserID, awemeID, liveRoomID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	tenant, err := s.openTenant(tenantID, data)
	if err != nil {
		return err
	}
	if advertiserID <= 0 {
		return errors.New("advertiser id is required")
	}
	if owner, exists := data.Bindings[bindingKey(advertiserID)]; exists && owner != tenantID {
		return ErrBindingInUse
	}
	tenant.Accounts = tenant.AccountList()
	found := false
	for index := range tenant.Accounts {
		if tenant.Accounts[index].AdvertiserID == advertiserID {
			tenant.Accounts[index].AwemeID = awemeID
			tenant.Accounts[index].LiveRoomID = liveRoomID
			found = true
			break
		}
	}
	if !found {
		tenant.Accounts = append(tenant.Accounts, AccountBinding{
			AdvertiserID: advertiserID,
			AwemeID:      awemeID,
			LiveRoomID:   liveRoomID,
		})
	}
	tenant.AdvertiserID = tenant.Accounts[0].AdvertiserID
	tenant.AwemeID = tenant.Accounts[0].AwemeID
	tenant.LiveRoomID = tenant.Accounts[0].LiveRoomID
	tenant.UpdatedAt = time.Now().UTC()
	for _, account := range tenant.Accounts {
		data.Bindings[bindingKey(account.AdvertiserID)] = tenantID
	}
	encoded, err := s.sealTenant(*tenant)
	if err != nil {
		return err
	}
	data.Tenants[tenantID] = encoded
	return s.write(data)
}

func (s *Store) RemoveAccount(tenantID string, advertiserID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	tenant, err := s.openTenant(tenantID, data)
	if err != nil {
		return err
	}
	accounts := tenant.AccountList()
	if len(accounts) <= 1 {
		return ErrLastAccount
	}
	filtered := make([]AccountBinding, 0, len(accounts)-1)
	found := false
	for _, account := range accounts {
		if account.AdvertiserID == advertiserID {
			found = true
			continue
		}
		filtered = append(filtered, account)
	}
	if !found {
		return ErrAccountNotFound
	}
	tenant.Accounts = filtered
	tenant.AdvertiserID = filtered[0].AdvertiserID
	tenant.AwemeID = filtered[0].AwemeID
	tenant.LiveRoomID = filtered[0].LiveRoomID
	tenant.UpdatedAt = time.Now().UTC()
	delete(data.Bindings, bindingKey(advertiserID))
	encoded, err := s.sealTenant(*tenant)
	if err != nil {
		return err
	}
	data.Tenants[tenantID] = encoded
	return s.write(data)
}

func (s *Store) List() ([]Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	result := make([]Tenant, 0, len(data.Tenants))
	for id := range data.Tenants {
		tenant, openErr := s.openTenant(id, data)
		if openErr != nil {
			return nil, openErr
		}
		tenant.KeyHash = ""
		result = append(result, *tenant)
	}
	sortTenants(result)
	return result, nil
}

func (s *Store) ensureFile() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create tenant directory: %w", err)
	}
	if _, err := os.Stat(s.path); err == nil {
		return nil
	}
	return s.write(fileData{
		Version:  3,
		Tenants:  map[string]encryptedTenant{},
		Index:    map[string]string{},
		Bindings: map[string]string{},
	})
}

func (s *Store) read() (fileData, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return fileData{}, err
	}
	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fileData{}, err
	}
	if data.Version == 1 {
		if data.Tenants == nil || data.Index == nil {
			return fileData{}, errors.New("unsupported tenant store format")
		}
		data.Version = 3
		data.Bindings = map[string]string{}
		for id := range data.Tenants {
			tenant, openErr := s.openTenant(id, data)
			if openErr != nil {
				return fileData{}, openErr
			}
			data.Bindings[bindingKey(tenant.AdvertiserID)] = id
		}
	} else if (data.Version != 2 && data.Version != 3) || data.Tenants == nil || data.Index == nil || data.Bindings == nil {
		return fileData{}, errors.New("unsupported tenant store format")
	}
	data.Version = 3
	data.Bindings = map[string]string{}
	for id := range data.Tenants {
		tenant, openErr := s.openTenant(id, data)
		if openErr != nil {
			return fileData{}, openErr
		}
		for _, account := range tenant.AccountList() {
			key := bindingKey(account.AdvertiserID)
			if owner, exists := data.Bindings[key]; exists && owner != id {
				return fileData{}, ErrBindingInUse
			}
			data.Bindings[key] = id
		}
	}
	return data, nil
}

func (s *Store) write(data fileData) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".tenant-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(raw); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path)
}

func (s *Store) sealTenant(value Tenant) (encryptedTenant, error) {
	plaintext, err := json.Marshal(value)
	if err != nil {
		return encryptedTenant{}, err
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return encryptedTenant{}, err
	}
	sealed := s.aead.Seal(nil, nonce, plaintext, []byte("tenant:"+value.ID))
	return encryptedTenant{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(sealed),
	}, nil
}

func (s *Store) openTenant(id string, data fileData) (*Tenant, error) {
	record, ok := data.Tenants[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return nil, err
	}
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, []byte("tenant:"+id))
	if err != nil {
		return nil, err
	}
	var tenant Tenant
	if err := json.Unmarshal(plaintext, &tenant); err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (s *Store) hashKey(raw string) string {
	mac := hmac.New(sha256.New, s.keyHash)
	_, _ = mac.Write([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(mac.Sum(nil))
}

func bindingKey(advertiserID int64) string {
	return fmt.Sprintf("%d", advertiserID)
}

func validTenantID(value string) bool {
	if value == "" || value == "." || value == ".." || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func sortTenants(tenants []Tenant) {
	if len(tenants) < 2 {
		return
	}
	for i := 1; i < len(tenants); i++ {
		for j := i; j > 0 && tenants[j].ID < tenants[j-1].ID; j-- {
			tenants[j], tenants[j-1] = tenants[j-1], tenants[j]
		}
	}
}
