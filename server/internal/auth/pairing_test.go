package auth

import (
	"testing"
	"time"
)

func TestPairingService_GenerateAndRedeem(t *testing.T) {
	db := testDB(t)
	svc := NewPairingService(db)

	code, err := svc.GenerateCode()
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("code length = %d, want 6", len(code))
	}

	deviceID, token, err := svc.RedeemCode(code, "Test Device")
	if err != nil {
		t.Fatalf("redeem code: %v", err)
	}
	if deviceID == "" {
		t.Error("device ID should not be empty")
	}
	if token == "" {
		t.Error("token should not be empty")
	}
}

func TestPairingService_RedeemInvalidCode(t *testing.T) {
	db := testDB(t)
	svc := NewPairingService(db)

	_, _, err := svc.RedeemCode("BADCOD", "Test Device")
	if err == nil {
		t.Fatal("expected error for invalid code, got nil")
	}
}

func TestPairingService_RedeemExpiredCode(t *testing.T) {
	db := testDB(t)
	svc := NewPairingService(db)

	// Insert an already-expired code directly.
	expired := time.Now().UTC().Add(-1 * time.Minute)
	db.Exec("INSERT INTO pairing_codes (code, expires_at) VALUES (?, ?)", "EXPIRD", expired)

	_, _, err := svc.RedeemCode("EXPIRD", "Test Device")
	if err == nil {
		t.Fatal("expected error for expired code, got nil")
	}
}

func TestPairingService_CodeConsumedAfterRedeem(t *testing.T) {
	db := testDB(t)
	svc := NewPairingService(db)

	code, _ := svc.GenerateCode()
	svc.RedeemCode(code, "Device 1")

	_, _, err := svc.RedeemCode(code, "Device 2")
	if err == nil {
		t.Fatal("expected error when reusing consumed code, got nil")
	}
}

func TestPairingService_HasPairedDevices_None(t *testing.T) {
	db := testDB(t)
	svc := NewPairingService(db)

	has, err := svc.HasPairedDevices()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has {
		t.Error("expected no paired devices")
	}
}

func TestPairingService_HasPairedDevices_One(t *testing.T) {
	db := testDB(t)
	svc := NewPairingService(db)

	code, _ := svc.GenerateCode()
	svc.RedeemCode(code, "Test Device")

	has, err := svc.HasPairedDevices()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has {
		t.Error("expected paired devices after pairing")
	}
}
