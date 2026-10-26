package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummarizeMirrorsLaravel(t *testing.T) {
	cases := []struct {
		name string
		errs ValidationErrors
		want string
	}{
		{"empty bag", ValidationErrors{}, ErrMessage},
		{"single field", ValidationErrors{"email": {"These credentials do not match our records."}},
			"These credentials do not match our records."},
		{"self disable role", ValidationErrors{"role": {"You cannot change your own role or disable your own account."}},
			"You cannot change your own role or disable your own account."},
		{"two messages", ValidationErrors{"a": {"first"}, "b": {"second"}}, "first (and 1 more error)"},
		{"three messages", ValidationErrors{"a": {"first"}, "b": {"second", "third"}}, "first (and 2 more errors)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := summarize(tc.errs); got != tc.want {
				t.Errorf("summarize() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSummarizeDeterministic: Go map iteration order must not leak into the
// message; sorted keys keep identical requests byte-identical.
func TestSummarizeDeterministic(t *testing.T) {
	errs := ValidationErrors{"zeta": {"z"}, "alpha": {"a"}, "mid": {"m"}}
	first := summarize(errs)
	for range 50 {
		if got := summarize(errs); got != first {
			t.Fatalf("summarize drifted: %q vs %q", got, first)
		}
	}
	if first != "a (and 2 more errors)" {
		t.Errorf("summarize = %q, want %q", first, "a (and 2 more errors)")
	}
}

func TestInvalidWritesLaravelShape(t *testing.T) {
	recorder := httptest.NewRecorder()
	Invalid(recorder, ValidationErrors{"password": {"The password field is required."}})

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", recorder.Code)
	}
	if ct := recorder.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content type = %q", ct)
	}

	var body struct {
		Message string              `json:"message"`
		Errors  map[string][]string `json:"errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body.Message != "The password field is required." {
		t.Errorf("message = %q", body.Message)
	}
	if len(body.Errors["password"]) != 1 {
		t.Errorf("errors = %#v", body.Errors)
	}
}

func TestDecodeOr422FieldTypes(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"port":"eighty"}`))

	var payload struct {
		Port *int `json:"port"`
	}
	handled := DecodeOr422(recorder, request, &payload)
	if !handled {
		t.Fatal("expected 422 for wrong-typed field")
	}
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "The port field is invalid.") {
		t.Errorf("body = %s", recorder.Body.String())
	}
}

func TestCORSAllowsConfiguredOriginOnly(t *testing.T) {
	handler := CORS("http://localhost")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	allowed := httptest.NewRecorder()
	handler.ServeHTTP(allowed, httptest.NewRequest(http.MethodGet, "/", nil))
	if allowed.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("missing Origin header must not get CORS headers")
	}

	allowed = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost")
	handler.ServeHTTP(allowed, req)
	if allowed.Header().Get("Access-Control-Allow-Origin") != "http://localhost" {
		t.Errorf("allow-origin = %q", allowed.Header().Get("Access-Control-Allow-Origin"))
	}
	if allowed.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("credentials header missing")
	}

	blocked := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.example")
	handler.ServeHTTP(blocked, req)
	if blocked.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("foreign origin was allowed")
	}

	preflight := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost")
	handler.ServeHTTP(preflight, req)
	if preflight.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", preflight.Code)
	}
	if !strings.Contains(preflight.Header().Get("Access-Control-Allow-Headers"), "X-XSRF-TOKEN") {
		t.Errorf("allow-headers = %q", preflight.Header().Get("Access-Control-Allow-Headers"))
	}
}
