package tokenstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CriarBrand/qianchuan-backend/pkg/session"
)

// Store persists one encrypted OAuth session per Qianchuan advertiser.
type Store struct {
	path string
	aead cipher.AEAD
	mu   sync.RWMutex
}

type fileData struct {
	Version int                        `json:"version"`
	Tokens  map[string]encryptedRecord `json:"tokens"`
}

type encryptedRecord struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// New creates an AES-256-GCM encrypted token store.
func New(path, encodedKey string) (*Store, error) {
	if path == "" {
		return nil, errors.New("TOKEN_STORE_PATH is required")
	}
	key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil {
		return nil, fmt.Errorf("TOKEN_ENCRYPTION_KEY must be base64 without padding: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("TOKEN_ENCRYPTION_KEY must decode to exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create token encryption cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create token encryption GCM: %w", err)
	}
	store := &Store{path: filepath.Clean(path), aead: aead}
	if err := store.ensureFile(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) ensureFile() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create token store directory: %w", err)
	}
	_, err := os.Stat(s.path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat token store: %w", err)
	}
	return s.write(fileData{Version: 1, Tokens: map[string]encryptedRecord{}})
}

// Save replaces any existing session for that advertiser.
func (s *Store) Save(advertiserID int64, value *session.UserSession) error {
	if advertiserID <= 0 || value == nil {
		return errors.New("advertiser_id and session are required")
	}
	plaintext, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate token nonce: %w", err)
	}
	sealed := s.aead.Seal(nil, nonce, plaintext, []byte(tokenAAD(advertiserID)))

	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	data.Tokens[tokenKey(advertiserID)] = encryptedRecord{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(sealed),
	}
	return s.write(data)
}

// Load returns a copy of the persisted session for an advertiser.
func (s *Store) Load(advertiserID int64) (*session.UserSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	record, ok := data.Tokens[tokenKey(advertiserID)]
	if !ok {
		return nil, os.ErrNotExist
	}
	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode token nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode token ciphertext: %w", err)
	}
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, []byte(tokenAAD(advertiserID)))
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}
	var result session.UserSession
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}
	return &result, nil
}

// Latest returns the most recently created valid session.
func (s *Store) Latest() (*session.UserSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	var latest *session.UserSession
	for key := range data.Tokens {
		advertiserID, parseErr := parseTokenKey(key)
		if parseErr != nil {
			continue
		}
		value, loadErr := s.loadUnlocked(advertiserID, data)
		if loadErr != nil {
			continue
		}
		if latest == nil || value.CreatedAt > latest.CreatedAt {
			latest = value
		}
	}
	if latest == nil {
		return nil, os.ErrNotExist
	}
	return latest, nil
}

// CleanupExpired removes records whose refresh token cannot be used anymore.
func (s *Store) CleanupExpired(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	changed := false
	for key := range data.Tokens {
		advertiserID, parseErr := parseTokenKey(key)
		if parseErr != nil {
			delete(data.Tokens, key)
			changed = true
			continue
		}
		value, loadErr := s.loadUnlocked(advertiserID, data)
		if loadErr != nil || now.Unix() >= value.RefreshExpires {
			delete(data.Tokens, key)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.write(data)
}

func (s *Store) loadUnlocked(advertiserID int64, data fileData) (*session.UserSession, error) {
	record, ok := data.Tokens[tokenKey(advertiserID)]
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
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, []byte(tokenAAD(advertiserID)))
	if err != nil {
		return nil, err
	}
	var result session.UserSession
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Store) read() (fileData, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return fileData{}, fmt.Errorf("read token store: %w", err)
	}
	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fileData{}, fmt.Errorf("parse token store: %w", err)
	}
	if data.Version != 1 || data.Tokens == nil {
		return fileData{}, errors.New("unsupported token store format")
	}
	return data, nil
}

func (s *Store) write(data fileData) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token store: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".qianchuan-token-*.tmp")
	if err != nil {
		return fmt.Errorf("create token store temp file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("secure token store temp file: %w", err)
	}
	if _, err := temp.Write(raw); err != nil {
		temp.Close()
		return fmt.Errorf("write token store: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync token store: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close token store: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		return fmt.Errorf("replace token store: %w", err)
	}
	return nil
}

func tokenKey(advertiserID int64) string {
	return fmt.Sprintf("%d", advertiserID)
}

func parseTokenKey(value string) (int64, error) {
	var advertiserID int64
	_, err := fmt.Sscanf(value, "%d", &advertiserID)
	return advertiserID, err
}

func tokenAAD(advertiserID int64) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("qianchuan-token:%d", advertiserID)))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
