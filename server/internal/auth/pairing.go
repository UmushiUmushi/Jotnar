// Device pairing and one-time code generation.
package auth

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
)

type PairingService struct {
	db *sql.DB
}

func NewPairingService(db *sql.DB) *PairingService {
	return &PairingService{db: db}
}

// GenerateCode creates a 6-character alphanumeric one-time pairing code.
// The code is stored in the database so it can be redeemed by the running server
// even when generated from a separate process (e.g. docker exec).
func (s *PairingService) GenerateCode() (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no O/0/I/1 for readability
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code[i] = charset[n.Int64()]
	}

	codeStr := string(code)
	expiresAt := time.Now().UTC().Add(10 * time.Minute)

	_, err := s.db.Exec(
		"INSERT INTO pairing_codes (code, expires_at) VALUES (?, ?)",
		codeStr, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("store pairing code: %w", err)
	}

	return codeStr, nil
}

// RedeemCode validates and consumes a pairing code, creating a new device.
func (s *PairingService) RedeemCode(code, deviceName string) (deviceID, token string, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var expiresAt time.Time
	err = tx.QueryRow("SELECT expires_at FROM pairing_codes WHERE code = ?", code).Scan(&expiresAt)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("invalid or expired pairing code")
	}
	if err != nil {
		return "", "", fmt.Errorf("query pairing code: %w", err)
	}

	if time.Now().UTC().After(expiresAt) {
		tx.Exec("DELETE FROM pairing_codes WHERE code = ?", code)
		tx.Commit()
		return "", "", fmt.Errorf("invalid or expired pairing code")
	}

	// Consume the code.
	if _, err := tx.Exec("DELETE FROM pairing_codes WHERE code = ?", code); err != nil {
		return "", "", fmt.Errorf("delete pairing code: %w", err)
	}

	token, err = GenerateToken()
	if err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	hash, err := HashToken(token)
	if err != nil {
		return "", "", fmt.Errorf("hash token: %w", err)
	}

	deviceID = uuid.New().String()
	now := time.Now().UTC()

	_, err = tx.Exec(
		"INSERT INTO devices (id, name, paired_at, token_hash, last_seen) VALUES (?, ?, ?, ?, ?)",
		deviceID, deviceName, now, hash, now,
	)
	if err != nil {
		return "", "", fmt.Errorf("insert device: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("commit: %w", err)
	}

	return deviceID, token, nil
}

// HasPairedDevices returns true if at least one device is paired.
func (s *PairingService) HasPairedDevices() (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
