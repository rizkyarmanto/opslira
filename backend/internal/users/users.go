// Package users implements the SUPER_ADMIN-gated managed-user API.
package users

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/auth"
	"opslira/backend/internal/httpx"
	"opslira/backend/internal/uuid"
)

// Roles allowed by the users_role_check constraint.
var Roles = []string{"SUPER_ADMIN", "SYSADMIN", "DEVELOPER", "MANAGER"}

// Exact self-management rejection messages from UserController.
const (
	selfRoleMessage    = "You cannot change your own role or disable your own account."
	selfDisableMessage = "You cannot disable your own account."
)

// User is the ManagedUser DTO (snake_case JSON).
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// Handler serves the /users routes.
type Handler struct {
	pool *pgxpool.Pool
}

// NewHandler wires the user management endpoints.
func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

// Mount registers /users behind the session middleware and the
// SUPER_ADMIN gate.
func (h *Handler) Mount(r chi.Router, mw func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(mw)
		r.Use(auth.RequireSuperAdmin)
		r.Get("/users", h.index)
		r.Post("/users", h.store)
		r.Put("/users/{userID}", h.update)
		r.Delete("/users/{userID}", h.destroy)
	})
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name, email, role, is_active, created_at
		   FROM users ORDER BY name`)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load users.")
		return
	}
	defer rows.Close()

	list := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.IsActive, &u.CreatedAt); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to load users.")
			return
		}
		list = append(list, u)
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"users":          list,
		"canManageUsers": true,
	})
}

type writeRequest struct {
	Name                 string `json:"name"`
	Email                string `json:"email"`
	Role                 string `json:"role"`
	IsActive             *bool  `json:"is_active"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

func (h *Handler) store(w http.ResponseWriter, r *http.Request) {
	var req writeRequest
	if httpx.DecodeOr422(w, r, &req) {
		return
	}
	req.Email = strings.TrimSpace(req.Email)

	errs := h.validate(req, "", true)
	if len(errs) > 0 {
		httpx.Invalid(w, errs)
		return
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to create user.")
		return
	}

	id := uuid.New()
	var createdAt time.Time
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO users (id, name, email, password_hash, role, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		 RETURNING created_at`,
		id, req.Name, req.Email, hash, req.Role, active,
	).Scan(&createdAt)
	if isUniqueViolation(err) {
		httpx.Invalid(w, httpx.ValidationErrors{"email": {"The email has already been taken."}})
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to create user.")
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"user": User{ID: id, Name: req.Name, Email: req.Email, Role: req.Role, IsActive: active, CreatedAt: createdAt},
	})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if !uuid.Valid(userID) {
		httpx.Fail(w, http.StatusNotFound, "User not found.")
		return
	}
	actor, _ := auth.UserFrom(r.Context())

	var req writeRequest
	if httpx.DecodeOr422(w, r, &req) {
		return
	}
	req.Email = strings.TrimSpace(req.Email)

	existing, err := h.load(r.Context(), userID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Fail(w, http.StatusNotFound, "User not found.")
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load user.")
		return
	}

	errs := h.validate(req, userID, req.Password == "")
	if len(errs) > 0 {
		httpx.Invalid(w, errs)
		return
	}

	// Self-management guards with the exact Laravel messages.
	wantActive := existing.IsActive
	if req.IsActive != nil {
		wantActive = *req.IsActive
	}
	if actor.ID == existing.ID && (req.Role != existing.Role || !wantActive) {
		httpx.Invalid(w, httpx.ValidationErrors{"role": {selfRoleMessage}})
		return
	}

	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to update user.")
			return
		}
		err = h.apply(r.Context(), userID, req.Name, req.Email, req.Role, wantActive, &hash)
	} else {
		err = h.apply(r.Context(), userID, req.Name, req.Email, req.Role, wantActive, nil)
	}
	if isUniqueViolation(err) {
		httpx.Invalid(w, httpx.ValidationErrors{"email": {"The email has already been taken."}})
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to update user.")
		return
	}

	fresh, err := h.load(r.Context(), userID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to update user.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": fresh})
}

func (h *Handler) destroy(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if !uuid.Valid(userID) {
		httpx.Fail(w, http.StatusNotFound, "User not found.")
		return
	}
	actor, _ := auth.UserFrom(r.Context())

	if actor.ID == userID {
		httpx.Invalid(w, httpx.ValidationErrors{"user": {selfDisableMessage}})
		return
	}

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE users SET is_active = false, updated_at = now() WHERE id = $1`, userID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to disable user.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "User not found.")
		return
	}

	httpx.Ok(w)
}

func (h *Handler) load(ctx context.Context, id string) (User, error) {
	var u User
	err := h.pool.QueryRow(ctx,
		`SELECT id, name, email, role, is_active, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.IsActive, &u.CreatedAt)

	return u, err
}

func (h *Handler) apply(ctx context.Context, id, name, email, role string, active bool, hash *string) error {
	if hash != nil {
		_, err := h.pool.Exec(ctx,
			`UPDATE users SET name=$2, email=$3, role=$4, is_active=$5, password_hash=$6, updated_at=now()
			 WHERE id=$1`,
			id, name, email, role, active, *hash,
		)
		return err
	}
	_, err := h.pool.Exec(ctx,
		`UPDATE users SET name=$2, email=$3, role=$4, is_active=$5, updated_at=now()
		 WHERE id=$1`,
		id, name, email, role, active,
	)

	return err
}

// validate mirrors StoreUserRequest/UpdateUserRequest. passwordOptional is
// true on updates without a password change (the `nullable` rule).
func (h *Handler) validate(req writeRequest, ignoreID string, passwordOptional bool) httpx.ValidationErrors {
	errs := httpx.ValidationErrors{}

	if req.Name == "" {
		errs["name"] = append(errs["name"], "The name field is required.")
	} else if len(req.Name) > 120 {
		errs["name"] = append(errs["name"], "The name may not be greater than 120 characters.")
	}

	switch {
	case req.Email == "":
		errs["email"] = append(errs["email"], "The email field is required.")
	case !isEmail(req.Email):
		errs["email"] = append(errs["email"], "The email must be a valid email address.")
	case len(req.Email) > 150:
		errs["email"] = append(errs["email"], "The email may not be greater than 150 characters.")
	default:
		if taken, err := h.emailTaken(req.Email, ignoreID); err == nil && taken {
			errs["email"] = append(errs["email"], "The email has already been taken.")
		}
	}

	if req.Role == "" {
		errs["role"] = append(errs["role"], "The role field is required.")
	} else if !contains(Roles, req.Role) {
		errs["role"] = append(errs["role"], "The selected role is invalid.")
	}

	if req.Password == "" && !passwordOptional {
		errs["password"] = append(errs["password"], "The password field is required.")
	}
	if req.Password != "" {
		if len(req.Password) < 8 {
			errs["password"] = append(errs["password"], "The password must be at least 8 characters.")
		}
		if req.Password != req.PasswordConfirmation {
			errs["password"] = append(errs["password"], "The password confirmation does not match.")
		}
	}

	return errs
}

func (h *Handler) emailTaken(email, ignoreID string) (bool, error) {
	var exists bool
	var err error
	if ignoreID == "" {
		err = h.pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	} else {
		err = h.pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND id <> $2)`, email, ignoreID).Scan(&exists)
	}

	return exists, err
}

func isEmail(value string) bool {
	addr, err := mail.ParseAddress(value)

	return err == nil && addr.Address == value
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}

	return false
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
