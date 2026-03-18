package auth

import (
	"testing"
)

func TestRecoveryService_GenerateAndValidate(t *testing.T) {
	db := testDB(t)
	svc := NewRecoveryService(db)

	key, err := svc.GenerateRecoveryKey()
	if err != nil {
		t.Fatalf("generate recovery key: %v", err)
	}
	if len(key) != 64 {
		t.Errorf("key length = %d, want 64 hex chars", len(key))
	}

	if err := svc.ValidateRecoveryKey(key); err != nil {
		t.Fatalf("validate recovery key: %v", err)
	}
}

func TestRecoveryService_InvalidKey(t *testing.T) {
	db := testDB(t)
	svc := NewRecoveryService(db)

	svc.GenerateRecoveryKey()

	if err := svc.ValidateRecoveryKey("wrong_key"); err == nil {
		t.Fatal("expected error for invalid recovery key, got nil")
	}
}

func TestRecoveryService_NoKeyConfigured(t *testing.T) {
	db := testDB(t)
	svc := NewRecoveryService(db)

	err := svc.ValidateRecoveryKey("any_key")
	if err == nil {
		t.Fatal("expected error when no recovery key configured, got nil")
	}
}
