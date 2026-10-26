package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User is the authenticated users row (never carries password_hash).
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
}

// IsSuperAdmin reports whether the user may access the /users management API.
func (u User) IsSuperAdmin() bool { return u.Role == "SUPER_ADMIN" }

type contextKey int

const userKey contextKey = 0

// UserFrom returns the user attached by Middleware, if any.
func UserFrom(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userKey).(User)

	return user, ok
}

// Middleware authenticates the opslira_session cookie against Redis and the
// users table, enforces the XSRF header on state-changing methods, and
// rejects inactive accounts with 401 (mirroring EnsureAccountIsActive).
func Middleware(pool *pgxpool.Pool, store *Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookie)
			if err != nil || cookie.Value == "" {
				httpError(w, http.StatusUnauthorized, "Unauthenticated.")
				return
			}

			sess, err := store.Get(r.Context(), cookie.Value)
			if err != nil {
				clearSessionCookies(w)
				httpError(w, http.StatusUnauthorized, "Unauthenticated.")
				return
			}

			user, err := loadUser(r.Context(), pool, sess.UserID)
			if errors.Is(err, pgx.ErrNoRows) {
				// The account vanished: invalidate the session outright.
				_ = store.Delete(r.Context(), cookie.Value)
				clearSessionCookies(w)
				httpError(w, http.StatusUnauthorized, "Unauthenticated.")
				return
			}
			if err != nil {
				httpError(w, http.StatusUnauthorized, "Unauthenticated.")
				return
			}
			if !user.IsActive {
				// Inactive mid-session: drop the session like Laravel's
				// EnsureAccountIsActive middleware and answer 401.
				_ = store.Delete(r.Context(), cookie.Value)
				clearSessionCookies(w)
				httpError(w, http.StatusUnauthorized, "Unauthenticated.")
				return
			}

			if !safeMethod(r.Method) && !validXSRF(r, sess.XSRF) {
				// 419 is Laravel's "page expired" code for CSRF failures;
				// kept verbatim so SPA error handling stays identical.
				httpError(w, 419, "CSRF token mismatch.")
				return
			}

			ctx := context.WithValue(r.Context(), userKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireSuperAdmin answers 403 unless the request user is SUPER_ADMIN.
func RequireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFrom(r.Context())
		if !ok || !user.IsSuperAdmin() {
			httpError(w, http.StatusForbidden, "Forbidden.")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loadUser(ctx context.Context, pool *pgxpool.Pool, id string) (User, error) {
	var u User
	err := pool.QueryRow(ctx,
		`SELECT id, name, email, role, is_active FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.IsActive)

	return u, err
}

func safeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func validXSRF(r *http.Request, expected string) bool {
	got := r.Header.Get(XSRFHeader)
	if got == "" {
		// Some clients echo the cookie value directly instead of the header.
		if cookie, err := r.Cookie(XSRFCookie); err == nil {
			got = cookie.Value
		}
	}
	if got == "" || expected == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
