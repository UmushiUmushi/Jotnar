// Recovery key generation and verification.
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type RecoveryService struct {
	db *sql.DB
}

func NewRecoveryService(db *sql.DB) *RecoveryService {
	return &RecoveryService{db: db}
}

// GenerateRecoveryKey creates and stores a recovery key. Returns the plaintext key.
func (s *RecoveryService) GenerateRecoveryKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := hex.EncodeToString(b)

	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO recovery (id, key_hash) VALUES (1, ?)",
		string(hash),
	)
	if err != nil {
		return "", fmt.Errorf("store recovery key: %w", err)
	}

	return key, nil
}

// ValidateRecoveryKey checks if the provided key matches the stored hash.
func (s *RecoveryService) ValidateRecoveryKey(key string) error {
	var hash string
	err := s.db.QueryRow("SELECT key_hash FROM recovery WHERE id = 1").Scan(&hash)
	if err == sql.ErrNoRows {
		return fmt.Errorf("no recovery key configured")
	}
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
}
