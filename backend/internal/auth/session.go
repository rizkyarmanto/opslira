package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Cookie and header names used by the session/CSRF pair. The XSRF cookie is
// deliberately readable by JavaScript: the SPA echoes it back in the
// X-XSRF-TOKEN header on every state-changing request.
const (
	SessionCookie = "opslira_session"
	XSRFCookie    = "XSRF-TOKEN"
	XSRFHeader    = "X-XSRF-TOKEN"
)

// ErrNoSession is returned when the token is missing or expired.
var ErrNoSession = errors.New("no session")

// Session is the Redis-stored payload behind an opaque token.
type Session struct {
	UserID string `json:"user_id"`
	XSRF   string `json:"xsrf"`
}

// Store creates and resolves opaque sessions in Redis DB0.
type Store struct {
	rdb *goredis.Client
	ttl time.Duration
}

// NewStore builds a session store; ttl is the absolute session lifetime
// (24h per spec, overridable via SESSION_TTL_SECONDS).
func NewStore(rdb *goredis.Client, ttl time.Duration) *Store {
	return &Store{rdb: rdb, ttl: ttl}
}

// Create mints a fresh sess:<32-byte hex> token plus its XSRF value.
func (s *Store) Create(ctx context.Context, userID string) (token string, sess Session, err error) {
	tokenBytes, err := randomHex(32)
	if err != nil {
		return "", Session{}, err
	}
	xsrf, err := randomHex(32)
	if err != nil {
		return "", Session{}, err
	}

	token = "sess:" + tokenBytes
	sess = Session{UserID: userID, XSRF: xsrf}

	payload, err := json.Marshal(sess)
	if err != nil {
		return "", Session{}, err
	}
	if err := s.rdb.Set(ctx, token, payload, s.ttl).Err(); err != nil {
		return "", Session{}, fmt.Errorf("session save: %w", err)
	}

	return token, sess, nil
}

// Get resolves a token to its session; expired/unknown tokens yield
// ErrNoSession.
func (s *Store) Get(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrNoSession
	}
	raw, err := s.rdb.Get(ctx, token).Bytes()
	if errors.Is(err, goredis.Nil) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("session read: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return Session{}, ErrNoSession
	}

	return sess, nil
}

// Delete removes a session (logout and inactive-account invalidation).
func (s *Store) Delete(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, token).Err()
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}
