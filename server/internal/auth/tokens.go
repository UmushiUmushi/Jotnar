// Token creation and validation.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// tokenCacheEntry stores a validated token mapping with an expiry time.
type tokenCacheEntry struct {
	deviceID  string
	expiresAt time.Time
}

// tokenCacheTTL is how long a validated token stays cached before re-checking bcrypt.
const tokenCacheTTL = 5 * time.Minute

type TokenService struct {
	db    *sql.DB
	mu    sync.RWMutex
	cache map[string]tokenCacheEntry // keyed by SHA-256 of the raw token
}

func NewTokenService(db *sql.DB) *TokenService {
	return &TokenService{
		db:    db,
		cache: make(map[string]tokenCacheEntry),
	}
}

// GenerateToken creates a random 32-byte hex token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns a bcrypt hash of the token.
func HashToken(token string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// tokenCacheKey returns a fast, non-reversible key for the cache lookup.
func tokenCacheKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// InvalidateCache removes all cached entries. Call this when a device is revoked.
func (s *TokenService) InvalidateCache() {
	s.mu.Lock()
	s.cache = make(map[string]tokenCacheEntry)
	s.mu.Unlock()
}

// ValidateToken checks the token against all devices and returns the device ID if valid.
// Recently validated tokens are cached by SHA-256 hash to avoid repeated bcrypt comparisons.
func (s *TokenService) ValidateToken(token string) (string, error) {
	cacheKey := tokenCacheKey(token)

	// Check cache first.
	s.mu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.RUnlock()
		// Update last_seen asynchronously — best effort.
		go func() {
			_, _ = s.db.Exec("UPDATE devices SET last_seen = ? WHERE id = ?", time.Now().UTC(), entry.deviceID)
		}()
		return entry.deviceID, nil
	}
	s.mu.RUnlock()

	// Cache miss — fall back to bcrypt scan.
	rows, err := s.db.Query("SELECT id, token_hash FROM devices")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return "", err
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(token)) == nil {
			// Populate cache.
			s.mu.Lock()
			s.cache[cacheKey] = tokenCacheEntry{
				deviceID:  id,
				expiresAt: time.Now().Add(tokenCacheTTL),
			}
			s.mu.Unlock()

			_, _ = s.db.Exec("UPDATE devices SET last_seen = ? WHERE id = ?", time.Now().UTC(), id)
			return id, nil
		}
	}

	return "", fmt.Errorf("invalid token")
}
