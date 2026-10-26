package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/httpx"
)

// Handler serves POST /auth/login, POST /auth/logout and GET /auth/me.
type Handler struct {
	pool           *pgxpool.Pool
	store          *Store
	ttl            time.Duration
	frontendOrigin string
}

// NewHandler wires the auth endpoints.
func NewHandler(pool *pgxpool.Pool, store *Store, ttl time.Duration, frontendOrigin string) *Handler {
	return &Handler{pool: pool, store: store, ttl: ttl, frontendOrigin: frontendOrigin}
}

// Mount registers the auth routes: login is public, logout/me sit behind the
// session middleware.
func (h *Handler) Mount(r chi.Router, mw func(http.Handler) http.Handler) {
	r.Post("/auth/login", h.login)
	r.Group(func(r chi.Router) {
		r.Use(mw)
		r.Post("/auth/logout", h.logout)
		r.Get("/auth/me", h.me)
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Invalid(w, httpx.ValidationErrors{"email": {"The email field is required."}})
		return
	}
	req.Email = strings.TrimSpace(req.Email)

	if req.Email == "" || req.Password == "" {
		errs := httpx.ValidationErrors{}
		if req.Email == "" {
			errs["email"] = []string{"The email field is required."}
		}
		if req.Password == "" {
			errs["password"] = []string{"The password field is required."}
		}
		httpx.Invalid(w, errs)
		return
	}

	user, hash, active, err := h.lookup(r.Context(), req.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		// Laravel answers a non-existent email with the same 422 as a wrong
		// password: the endpoint never discloses which addresses exist.
		httpx.Invalid(w, httpx.ValidationErrors{"email": {"These credentials do not match our records."}})
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Login failed.")
		return
	}

	if !VerifyPassword(hash, req.Password) {
		httpx.Invalid(w, httpx.ValidationErrors{"email": {"These credentials do not match our records."}})
		return
	}
	if !active {
		// Inactive accounts fail Auth::attempt (is_active=true filter), but
		// the API contract wants 401 here.
		httpError(w, http.StatusUnauthorized, "Your account is disabled.")
		return
	}

	token, sess, err := h.store.Create(r.Context(), user.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "Login failed.")
		return
	}
	setSessionCookies(w, token, sess.XSRF, h.ttl, secureCookies(h.frontendOrigin))

	httpx.JSON(w, http.StatusOK, map[string]any{"user": publicUser(user)})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookie); err == nil && cookie.Value != "" {
		_ = h.store.Delete(r.Context(), cookie.Value)
	}
	clearSessionCookies(w)

	httpx.Ok(w)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())
	httpx.JSON(w, http.StatusOK, map[string]any{
		"user": publicUser(user),
		// Inertia shared props: the SPA gates the Users nav item on
		// can.manageUsers (HandleInertiaRequests::share).
		"can": map[string]bool{"manageUsers": user.IsSuperAdmin()},
	})
}

// Me is the exported alias so main can mount GET /me beside /auth/me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) { h.me(w, r) }

func (h *Handler) lookup(ctx context.Context, email string) (User, string, bool, error) {
	var (
		u      User
		hash   string
		active bool
	)
	err := h.pool.QueryRow(ctx,
		`SELECT id, name, email, role, is_active, password_hash FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &active, &hash)
	u.IsActive = active

	return u, hash, active, err
}

// publicUser strips is_active: the Inertia session shared shape is exactly
// id/name/email/role.
func publicUser(u User) map[string]any {
	return map[string]any{
		"id":    u.ID,
		"name":  u.Name,
		"email": u.Email,
		"role":  u.Role,
	}
}
