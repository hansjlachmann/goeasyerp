package secrets

import (
	"errors"
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	defer SetKeyForTest("test key")()

	stored, err := Encrypt("abcd efgh ijkl mnop")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(stored) || strings.Contains(stored, "abcd") {
		t.Fatalf("stored = %q, want an encrypted value", stored)
	}
	if len(stored) != EncryptedLength(len("abcd efgh ijkl mnop")) {
		t.Errorf("length %d, EncryptedLength says %d", len(stored), EncryptedLength(19))
	}
	again, _ := Encrypt("abcd efgh ijkl mnop")
	if again == stored {
		t.Error("two encryptions of the same value are equal (nonce reused)")
	}
	if plain, err := Decrypt(stored); err != nil || plain != "abcd efgh ijkl mnop" {
		t.Errorf("Decrypt = %q, %v", plain, err)
	}
	if same, _ := Encrypt(stored); same != stored {
		t.Error("an encrypted value was encrypted again")
	}
	if empty, err := Encrypt(""); err != nil || empty != "" {
		t.Errorf("Encrypt(\"\") = %q, %v", empty, err)
	}
}

func TestDecryptPlainAndWrongKey(t *testing.T) {
	restore := SetKeyForTest("first key")
	stored, err := Encrypt("secret")
	restore()
	if err != nil {
		t.Fatal(err)
	}

	defer SetKeyForTest("second key")()
	if _, err := Decrypt(stored); !errors.Is(err, ErrDecrypt) {
		t.Errorf("Decrypt with another key: %v, want ErrDecrypt", err)
	}
	// A value stored before encryption is read as it is
	if plain, err := Decrypt("old plain password"); err != nil || plain != "old plain password" {
		t.Errorf("Decrypt(plain) = %q, %v", plain, err)
	}
}

func TestNoKey(t *testing.T) {
	defer SetKeyForTest("")()
	if _, err := Encrypt("secret"); !errors.Is(err, ErrNoKey) {
		t.Errorf("Encrypt without key: %v, want ErrNoKey", err)
	}
	if _, err := Decrypt(prefix + "AAAA"); !errors.Is(err, ErrNoKey) {
		t.Errorf("Decrypt without key: %v, want ErrNoKey", err)
	}
}

// tools/tablegen computes the same length to size encrypted columns (maxEncryptedPlain)
func TestEncryptedLengthMatchesTablegen(t *testing.T) {
	if got := EncryptedLength(154); got != 250 {
		t.Errorf("EncryptedLength(154) = %d, want 250 (as tablegen's encryptedLength)", got)
	}
}

// An installation that adds OPENERP_ENCRYPTION_KEY keeps its secrets: the key derived from
// JWT_SECRET still decrypts them, and Rewrap encrypts them with the new key — after which
// JWT_SECRET can change. OPENERP_ENCRYPTION_KEY_OLD does the same when the key is replaced.
func TestOldKeysAndRewrap(t *testing.T) {
	restore := SetKeysForTest("", "", "the jwt secret")
	fromJWT, err := Encrypt("smtp password")
	restore()
	if err != nil {
		t.Fatal(err)
	}

	restore = SetKeysForTest("new key", "", "the jwt secret")
	if plain, err := Decrypt(fromJWT); err != nil || plain != "smtp password" {
		t.Fatalf("Decrypt with the JWT_SECRET key as old key = %q, %v", plain, err)
	}
	wrapped, changed, err := Rewrap(fromJWT)
	if err != nil || !changed || wrapped == fromJWT {
		t.Fatalf("Rewrap = %q, %v, %v", wrapped, changed, err)
	}
	if same, changed, err := Rewrap(wrapped); err != nil || changed || same != wrapped {
		t.Errorf("Rewrap of a value with the current key = %v, %v", changed, err)
	}
	if typed, changed, err := Rewrap("typed"); err != nil || !changed || !IsEncrypted(typed) {
		t.Errorf("Rewrap of a typed value = %q, %v, %v", typed, changed, err)
	}
	restore()

	// JWT_SECRET replaced: the rewrapped value still decrypts, the old one does not
	restore = SetKeysForTest("new key", "", "another jwt secret")
	if plain, err := Decrypt(wrapped); err != nil || plain != "smtp password" {
		t.Errorf("after the JWT_SECRET change: %q, %v", plain, err)
	}
	if _, _, err := Rewrap(fromJWT); !errors.Is(err, ErrDecrypt) {
		t.Errorf("a value of no known key: %v, want ErrDecrypt", err)
	}
	restore()

	// the key replaced, the former one as OPENERP_ENCRYPTION_KEY_OLD
	defer SetKeysForTest("third key", "new key", "")()
	if plain, err := Decrypt(wrapped); err != nil || plain != "smtp password" {
		t.Errorf("with the old key in OPENERP_ENCRYPTION_KEY_OLD: %q, %v", plain, err)
	}
	if _, changed, err := Rewrap(wrapped); err != nil || !changed {
		t.Errorf("Rewrap with the old key: %v, %v", changed, err)
	}
}
