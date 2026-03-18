package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGenerateToken_Length(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("token length = %d, want 64 hex chars", len(token))
	}
}

func TestGenerateToken_Uniqueness(t *testing.T) {
	t1, _ := GenerateToken()
	t2, _ := GenerateToken()
	if t1 == t2 {
		t.Error("two generated tokens should not be equal")
	}
}

func TestHashToken_Verifiable(t *testing.T) {
	token, _ := GenerateToken()
	hash, err := HashToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(token)) != nil {
		t.Error("bcrypt should verify the hashed token")
	}
}

func TestTokenService_ValidateToken(t *testing.T) {
	db := testDB(t)
	pairing := NewPairingService(db)
	tokens := NewTokenService(db)

	code, _ := pairing.GenerateCode()
	deviceID, token, _ := pairing.RedeemCode(code, "Test Device")

	gotID, err := tokens.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if gotID != deviceID {
		t.Errorf("device ID = %q, want %q", gotID, deviceID)
	}
}

func TestTokenService_InvalidToken(t *testing.T) {
	db := testDB(t)
	tokens := NewTokenService(db)

	_, err := tokens.ValidateToken("definitely_not_a_valid_token")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}

func TestTokenService_CachePopulated(t *testing.T) {
	db := testDB(t)
	pairing := NewPairingService(db)
	tokens := NewTokenService(db)

	code, _ := pairing.GenerateCode()
	_, token, _ := pairing.RedeemCode(code, "Test Device")

	// First call populates cache.
	tokens.ValidateToken(token)

	// Verify cache has an entry.
	cacheKey := tokenCacheKey(token)
	tokens.mu.RLock()
	_, cached := tokens.cache[cacheKey]
	tokens.mu.RUnlock()

	if !cached {
		t.Error("expected token to be cached after first validation")
	}

	// Second call should hit cache (we can't easily verify this without timing,
	// but we verify it still returns the correct result).
	gotID, err := tokens.ValidateToken(token)
	if err != nil {
		t.Fatalf("second validate: %v", err)
	}
	if gotID == "" {
		t.Error("cached validation should return device ID")
	}
}

func TestTokenService_InvalidateCache(t *testing.T) {
	db := testDB(t)
	pairing := NewPairingService(db)
	tokens := NewTokenService(db)

	code, _ := pairing.GenerateCode()
	_, token, _ := pairing.RedeemCode(code, "Test Device")

	// Populate cache.
	tokens.ValidateToken(token)
	tokens.InvalidateCache()

	// Should still work (re-scans DB).
	_, err := tokens.ValidateToken(token)
	if err != nil {
		t.Fatalf("validation after cache invalidation should succeed: %v", err)
	}
}
