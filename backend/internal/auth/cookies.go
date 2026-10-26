package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// httpError writes the Laravel-compatible {"message": ...} body.
func httpError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

// setSessionCookies writes the HttpOnly session cookie and the readable
// XSRF cookie the SPA echoes back in X-XSRF-TOKEN.
func setSessionCookies(w http.ResponseWriter, token, xsrf string, ttl time.Duration, secure bool) {
	maxAge := int(ttl / time.Second)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     XSRFCookie,
		Value:    xsrf,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

// clearSessionCookies expires both cookies (logout / forced invalidation).
func clearSessionCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	http.SetCookie(w, &http.Cookie{Name: XSRFCookie, Value: "", Path: "/", MaxAge: -1})
}

// secureCookies reports whether cookies may carry the Secure flag, derived
// from the configured browser origin (https deployments only).
func secureCookies(frontendOrigin string) bool {
	return strings.HasPrefix(frontendOrigin, "https://")
}
