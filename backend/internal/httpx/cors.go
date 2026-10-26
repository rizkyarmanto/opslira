package httpx

import "net/http"

// CORS allows exactly one browser origin (FRONTEND_ORIGIN) with credentials,
// and answers preflight. Requests without an Origin header (curl, probes,
// the scheduler calling itself) pass through untouched.
func CORS(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqOrigin := r.Header.Get("Origin")
			if reqOrigin != "" && reqOrigin == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Add("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, "+XSRFAcceptedHeader())
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// XSRFAcceptedHeader is the header the SPA echoes the XSRF cookie back in;
// spelled here so the CORS allow-list cannot drift from the middleware.
func XSRFAcceptedHeader() string { return "X-XSRF-TOKEN" }
