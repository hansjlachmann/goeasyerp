// Package secrets encrypts secrets the application stores in the database (e.g. the SMTP
// password), so a database dump or backup does not reveal them.
//
// Values are encrypted with AES-256-GCM and stored as "enc:v1:<base64(nonce|ciphertext)>".
// The key comes from OPENERP_ENCRYPTION_KEY, else it is derived from JWT_SECRET (so an
// installation with a JWT_SECRET needs no new setting). Without either, values are stored
// as they are and a warning is logged at startup.
//
// Old keys still decrypt: with OPENERP_ENCRYPTION_KEY set, the key derived from JWT_SECRET and
// the key in OPENERP_ENCRYPTION_KEY_OLD are tried after it, and the server re-encrypts what it
// finds with them at startup (Rewrap) — so an installation can add OPENERP_ENCRYPTION_KEY, or
// replace it, without entering its secrets again. A value no key decrypts has to be entered
// again.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"sync"
)

// prefix marks an encrypted value (v1: AES-256-GCM, 12-byte nonce)
const prefix = "enc:v1:"

var (
	// ErrNoKey: neither OPENERP_ENCRYPTION_KEY nor JWT_SECRET is set
	ErrNoKey = errors.New("secrets: no encryption key (set OPENERP_ENCRYPTION_KEY or JWT_SECRET)")
	// ErrDecrypt: the value was encrypted with another key, or is damaged
	ErrDecrypt = errors.New("secrets: value cannot be decrypted (was the encryption key changed?)")
)

var (
	keyOnce   sync.Once
	keyBytes  []byte
	keySource string
	// oldKeys decrypt values encrypted before the current key (never used to encrypt)
	oldKeys [][]byte
)

// loadKey reads the key once: OPENERP_ENCRYPTION_KEY, else derived from JWT_SECRET.
// Any string is accepted; the AES key is its SHA-256 (with a label for JWT_SECRET, so
// the encryption key differs from the session signing key). With OPENERP_ENCRYPTION_KEY set,
// the key derived from JWT_SECRET and OPENERP_ENCRYPTION_KEY_OLD are old keys.
func loadKey() {
	keyOnce.Do(func() {
		keyBytes, keySource, oldKeys = keysFrom(os.Getenv("OPENERP_ENCRYPTION_KEY"),
			os.Getenv("OPENERP_ENCRYPTION_KEY_OLD"), os.Getenv("JWT_SECRET"))
	})
}

// keysFrom: the current key, its source and the old keys from the three settings
func keysFrom(key, oldKey, jwtSecret string) ([]byte, string, [][]byte) {
	derived := func(j string) []byte {
		sum := sha256.Sum256([]byte("openerp secrets v1\x00" + j))
		return sum[:]
	}
	plainKey := func(k string) []byte {
		sum := sha256.Sum256([]byte(k))
		return sum[:]
	}
	if key == "" {
		if jwtSecret == "" {
			return nil, "", nil
		}
		return derived(jwtSecret), "JWT_SECRET", nil
	}
	var old [][]byte
	if oldKey != "" && oldKey != key {
		old = append(old, plainKey(oldKey))
	}
	if jwtSecret != "" {
		old = append(old, derived(jwtSecret))
	}
	return plainKey(key), "OPENERP_ENCRYPTION_KEY", old
}

// SetKeysForTest sets the keys as the environment would give them and returns a function
// restoring the ones before.
func SetKeysForTest(key, oldKey, jwtSecret string) func() {
	loadKey()
	k, src, old := keyBytes, keySource, oldKeys
	keyBytes, keySource, oldKeys = keysFrom(key, oldKey, jwtSecret)
	return func() { keyBytes, keySource, oldKeys = k, src, old }
}

// Source names where the key comes from ("OPENERP_ENCRYPTION_KEY", "JWT_SECRET"), or ""
// when there is none.
func Source() string {
	loadKey()
	return keySource
}

// SetKeyForTest replaces the key (nil: no key) and returns a function restoring it.
func SetKeyForTest(key string) func() {
	loadKey()
	oldKey, oldSource, oldOld := keyBytes, keySource, oldKeys
	keyBytes, keySource, oldKeys = nil, "", nil
	if key != "" {
		sum := sha256.Sum256([]byte(key))
		keyBytes, keySource = sum[:], "test"
	}
	return func() { keyBytes, keySource, oldKeys = oldKey, oldSource, oldOld }
}

// IsEncrypted reports whether a stored value is encrypted
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, prefix)
}

// Encrypt returns the stored form of a secret. "" and already encrypted values are
// returned as they are; without a key it fails with ErrNoKey.
func Encrypt(plain string) (string, error) {
	if plain == "" || IsEncrypted(plain) {
		return plain, nil
	}
	gcm, err := newGCM()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

// Decrypt returns the secret of a stored value. A value that is not encrypted (stored
// before encryption, or without a key) is returned as it is. A value encrypted with an old
// key (see the package comment) is decrypted with it.
func Decrypt(stored string) (string, error) {
	plain, _, err := decrypt(stored)
	return plain, err
}

// decrypt: the secret, and whether an old key decrypted it
func decrypt(stored string) (string, bool, error) {
	if !IsEncrypted(stored) {
		return stored, false, nil
	}
	loadKey()
	if keyBytes == nil {
		return "", false, ErrNoKey
	}
	sealed, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", false, ErrDecrypt
	}
	for i, key := range append([][]byte{keyBytes}, oldKeys...) {
		gcm, err := gcmOf(key)
		if err != nil {
			return "", false, err
		}
		if len(sealed) < gcm.NonceSize() {
			return "", false, ErrDecrypt
		}
		plain, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
		if err == nil {
			return string(plain), i > 0, nil
		}
	}
	return "", false, ErrDecrypt
}

// Rewrap returns the stored form a value should have with the current key: a value stored as
// typed encrypted, one encrypted with an old key encrypted again with the current one.
// changed is false when the value is already right ("" too). A value no key decrypts is
// returned with ErrDecrypt.
func Rewrap(stored string) (string, bool, error) {
	if stored == "" {
		return stored, false, nil
	}
	if !IsEncrypted(stored) {
		enc, err := Encrypt(stored)
		return enc, err == nil, err
	}
	plain, old, err := decrypt(stored)
	if err != nil || !old {
		return stored, false, err
	}
	gcm, err := newGCM()
	if err != nil {
		return stored, false, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return stored, false, err
	}
	return prefix + base64.RawStdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), nil)), true, nil
}

// EncryptedLength is the stored length of a secret of n bytes
func EncryptedLength(n int) int {
	return len(prefix) + base64.RawStdEncoding.EncodedLen(12+n+16)
}

func newGCM() (cipher.AEAD, error) {
	loadKey()
	if keyBytes == nil {
		return nil, ErrNoKey
	}
	return gcmOf(keyBytes)
}

func gcmOf(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
