package auth

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	srv := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: srv.Addr()})

	return NewStore(rdb, 24*time.Hour), srv
}

// TestSessionTokenShape pins the sess:<64 hex> opaque format from the brief.
func TestSessionTokenShape(t *testing.T) {
	store, _ := newTestStore(t)

	token, sess, err := store.Create(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(token) != len("sess:")+64 || token[:5] != "sess:" {
		t.Fatalf("token shape wrong: %q", token)
	}
	if sess.UserID != "user-1" || sess.XSRF == "" || len(sess.XSRF) != 64 {
		t.Fatalf("session payload wrong: %#v", sess)
	}

	got, err := store.Get(context.Background(), token)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != sess.UserID || got.XSRF != sess.XSRF {
		t.Errorf("Get = %#v, want %#v", got, sess)
	}
}

func TestSessionMissingAndDeleted(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if _, err := store.Get(ctx, "sess:deadbeef"); err != ErrNoSession {
		t.Errorf("missing token err = %v, want ErrNoSession", err)
	}
	if _, err := store.Get(ctx, ""); err != ErrNoSession {
		t.Errorf("empty token err = %v, want ErrNoSession", err)
	}

	token, _, err := store.Create(ctx, "user-2")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, token); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, token); err != ErrNoSession {
		t.Errorf("deleted token err = %v, want ErrNoSession", err)
	}
}

// TestSessionTTL proves the Redis key expires (24h default in production).
func TestSessionTTL(t *testing.T) {
	store, srv := newTestStore(t)
	ctx := context.Background()

	token, _, err := store.Create(ctx, "user-3")
	if err != nil {
		t.Fatal(err)
	}
	ttl := srv.TTL(token)
	if ttl != 24*time.Hour {
		t.Errorf("TTL = %s, want 24h", ttl)
	}
}
