package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
)

// ErrMessage is the Laravel 422 message field.
const ErrMessage = "The given data was invalid."

// ValidationErrors maps a field name to its human-readable messages.
type ValidationErrors map[string][]string

// FieldError marks a JSON body whose value had the wrong type for the named
// field, so Decode failures surface as a normal 422 field error.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Message }

// JSON writes v as JSON with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Ok writes {"ok": true} with 200.
func Ok(w http.ResponseWriter) {
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Fail writes {"message": ...} with the given status.
func Fail(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"message": message})
}

// Invalid writes the Laravel 422 validation shape. The message field follows
// ValidationException::summarize: the first bag message, plus " (and N more
// errors)" when others follow; an empty bag falls back to ErrMessage. Map
// keys are sorted before flattening so identical requests answer identically.
func Invalid(w http.ResponseWriter, errs ValidationErrors) {
	if errs == nil {
		errs = ValidationErrors{}
	}
	JSON(w, http.StatusUnprocessableEntity, map[string]any{
		"message": summarize(errs),
		"errors":  errs,
	})
}

// summarize mirrors ValidationException::summarize for one error bag.
func summarize(errs ValidationErrors) string {
	keys := make([]string, 0, len(errs))
	for key := range errs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	flattened := []string{}
	for _, key := range keys {
		flattened = append(flattened, errs[key]...)
	}
	if len(flattened) == 0 {
		return ErrMessage
	}
	if len(flattened) == 1 {
		return flattened[0]
	}
	count := len(flattened) - 1
	plural := "errors"
	if count == 1 {
		plural = "error"
	}

	return flattened[0] + fmt.Sprintf(" (and %d more %s)", count, plural)
}

// Unauthorized writes the Laravel unauthenticated shape.
func Unauthorized(w http.ResponseWriter) {
	Fail(w, http.StatusUnauthorized, "Unauthenticated.")
}

// Decode strictly decodes a JSON request body into dst. A wrong-typed value
// is returned as *FieldError for conversion into a 422 field message.
func Decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return &FieldError{
				Field:   typeErr.Field,
				Message: fmt.Sprintf("The %s field is invalid.", typeErr.Field),
			}
		}
		return err
	}

	return nil
}

// DecodeOr422 decodes the body, answering 422 on failure. It reports whether
// the request was handled.
func DecodeOr422(w http.ResponseWriter, r *http.Request, dst any) bool {
	err := Decode(r, dst)
	if err == nil {
		return false
	}

	var fieldErr *FieldError
	if errors.As(err, &fieldErr) {
		Invalid(w, ValidationErrors{fieldErr.Field: {fieldErr.Message}})
	} else {
		Invalid(w, ValidationErrors{})
	}

	return true
}
