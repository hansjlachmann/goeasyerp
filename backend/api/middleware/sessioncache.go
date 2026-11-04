package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// CachedSession wraps a session with expiry info
type CachedSession struct {
	Session   *session.Session
	ExpiresAt time.Time
}

// SessionCache is a thread-safe in-memory cache for sessions
type SessionCache struct {
	mu      sync.RWMutex
	entries map[string]*CachedSession
	stop    chan struct{}
}

// NewSessionCache creates a new session cache and starts a background cleanup goroutine
func NewSessionCache() *SessionCache {
	sc := &SessionCache{
		entries: make(map[string]*CachedSession),
		stop:    make(chan struct{}),
	}
	go sc.cleanup()
	return sc
}

// Get returns a cached session, or nil if not found/expired
func (sc *SessionCache) Get(key string) *session.Session {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	entry, ok := sc.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil
	}
	return entry.Session
}

// Set stores a session in the cache with a TTL
func (sc *SessionCache) Set(key string, sess *session.Session, ttl time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.entries[key] = &CachedSession{
		Session:   sess,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Remove removes a specific entry from the cache
func (sc *SessionCache) Remove(key string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	delete(sc.entries, key)
}

// RemoveByUser removes all entries for a given user ID
func (sc *SessionCache) RemoveByUser(userID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	for key := range sc.entries {
		// Keys are "userID:company"
		if strings.HasPrefix(key, userID+":") {
			delete(sc.entries, key)
		}
	}
}

// RemoveByCompany removes all entries for a given company (renamed or deleted)
func (sc *SessionCache) RemoveByCompany(company string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	for key := range sc.entries {
		// Keys are "userID:company"; user IDs contain no ':'
		if _, c, ok := strings.Cut(key, ":"); ok && c == company {
			delete(sc.entries, key)
		}
	}
}

// Stop stops the background cleanup goroutine
func (sc *SessionCache) Stop() {
	close(sc.stop)
}

// cleanup periodically removes expired entries
func (sc *SessionCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sc.mu.Lock()
			now := time.Now()
			for key, entry := range sc.entries {
				if now.After(entry.ExpiresAt) {
					delete(sc.entries, key)
				}
			}
			sc.mu.Unlock()
		case <-sc.stop:
			return
		}
	}
}
