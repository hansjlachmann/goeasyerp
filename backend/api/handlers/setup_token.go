package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"log"
	"os"
	"strings"
	"sync"
)

// The setup token (S37): the first user — SUPER in every company — is made only with it, so a
// new installation reachable from the network does not belong to whoever calls /api/auth/init
// first. It is SETUP_TOKEN from the environment (the trial install scripts put one into .env and
// print it), else a random one this server writes to its log at start while no user exists.
var (
	setupTokenMu sync.RWMutex
	setupToken   string
)

// InitSetupToken sets the setup token at start (SETUP_TOKEN, else a random one) and logs it
// while the installation has no user yet
func InitSetupToken(db *sql.DB) {
	token := strings.TrimSpace(os.Getenv("SETUP_TOKEN"))
	fromEnv := token != ""
	if !fromEnv {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			log.Printf("setup token: %v (the first user cannot be made)", err)
			return
		}
		token = hex.EncodeToString(b)
	}
	SetSetupToken(token)

	var users int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "User"`).Scan(&users); err != nil || users > 0 {
		return
	}
	if fromEnv {
		log.Println("No user yet: create the first user on the login page with the setup token from SETUP_TOKEN")
		return
	}
	log.Printf("No user yet: create the first user on the login page with the setup token %s", token)
}

// SetSetupToken sets the token the first user needs ("" none: no first user can be made)
func SetSetupToken(token string) {
	setupTokenMu.Lock()
	defer setupTokenMu.Unlock()
	setupToken = token
}

// setupTokenValid reports whether given is the setup token (compared in constant time)
func setupTokenValid(given string) bool {
	setupTokenMu.RLock()
	defer setupTokenMu.RUnlock()
	given = strings.TrimSpace(given)
	if setupToken == "" || given == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(given), []byte(setupToken)) == 1
}
