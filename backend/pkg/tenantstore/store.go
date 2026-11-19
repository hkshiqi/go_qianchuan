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
	ErrBindingInUse = errors.New("advertiser is already bound to another tenant")
	ErrTenantExists = errors.New("tenant already exists")
)

type Tenant struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	KeyHash      string    `json:"key_hash"`
	AdvertiserID int64     `json:"advertiser_id"`
	AwemeID      int64     `json:"aweme_id"`
	LiveRoomID   int64     `json:"live_room_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (t Tenant) Validate() error {
	if !validTenantID(t.ID) {
		return errors.New("tenant id must contain only letters, digits, underscore or hyphen")
	}
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("tenant name is required")
	}
	if t.AdvertiserID <= 0 {
		return errors.New("advertiser id is required")
	}
	return nil
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
		if current.AdvertiserID == tenant.AdvertiserID {
			return ErrBindingInUse
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
	data.Bindings[bindingKey(tenant.AdvertiserID)] = tenant.ID
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
	if data.Bindings[bindingKey(tenant.AdvertiserID)] == tenantID {
		delete(data.Bindings, bindingKey(tenant.AdvertiserID))
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
	previousAdvertiserID := tenant.AdvertiserID
	if advertiserID <= 0 {
		return errors.New("advertiser id is required")
	}
	if owner, exists := data.Bindings[bindingKey(advertiserID)]; exists && owner != tenantID {
		return ErrBindingInUse
	}
	tenant.AdvertiserID = advertiserID
	tenant.AwemeID = awemeID
	tenant.LiveRoomID = liveRoomID
	tenant.UpdatedAt = time.Now().UTC()
	delete(data.Bindings, bindingKey(previousAdvertiserID))
	encoded, err := s.sealTenant(*tenant)
	if err != nil {
		return err
	}
	data.Tenants[tenantID] = encoded
	data.Bindings[bindingKey(advertiserID)] = tenantID
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
		Version:  2,
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
		data.Version = 2
		data.Bindings = map[string]string{}
		for id := range data.Tenants {
			tenant, openErr := s.openTenant(id, data)
			if openErr != nil {
				return fileData{}, openErr
			}
			data.Bindings[bindingKey(tenant.AdvertiserID)] = id
		}
	} else if data.Version != 2 || data.Tenants == nil || data.Index == nil || data.Bindings == nil {
		return fileData{}, errors.New("unsupported tenant store format")
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
