package middleware

import (
	"fmt"
	"os"
	"strings"
)

// knownJWTSecrets are example and default texts that have stood in the repository's compose
// files and docs: they are public, so a token signed with one can be made by anyone (S36)
var knownJWTSecrets = []string{
	"change-me-in-production",
	"replace-this-with-a-long-random-text",
	"changeme",
	"secret",
}

// minJWTSecret: shorter secrets refuse the start (an HMAC key that can be guessed offline from
// one token); minGoodJWTSecret: shorter ones only warn
const (
	minJWTSecret     = 16
	minGoodJWTSecret = 32
)

// CheckJWTSecret refuses a JWT_SECRET anyone could know or guess: a known example text, a short
// one, or — on an installation with a database server (DB_HOST, production and every Docker
// installation) — none at all (a random key would log everybody out at each start and leave
// the stored secrets unencrypted). It returns a warning for a secret that is allowed but short.
func CheckJWTSecret() (warning string, err error) {
	return checkJWTSecret(os.Getenv("JWT_SECRET"), os.Getenv("DB_HOST") != "")
}

func checkJWTSecret(secret string, server bool) (string, error) {
	const how = "set JWT_SECRET to a long random text, e.g. the output of: openssl rand -hex 32 (changing it logs everyone out; stored secrets such as the SMTP password must be entered again unless OPENERP_ENCRYPTION_KEY is set)"
	if secret == "" {
		if server {
			return "", fmt.Errorf("JWT_SECRET is not set: %s", how)
		}
		return "", nil // development: a random key (DefaultJWTConfig warns)
	}
	for _, known := range knownJWTSecrets {
		if strings.EqualFold(strings.TrimSpace(secret), known) {
			return "", fmt.Errorf("JWT_SECRET is an example text everyone can read in the repository: %s", how)
		}
	}
	if len(secret) < minJWTSecret {
		return "", fmt.Errorf("JWT_SECRET is shorter than %d characters: %s", minJWTSecret, how)
	}
	if len(secret) < minGoodJWTSecret {
		return fmt.Sprintf("WARNING: JWT_SECRET is shorter than %d characters — %s", minGoodJWTSecret, how), nil
	}
	return "", nil
}
