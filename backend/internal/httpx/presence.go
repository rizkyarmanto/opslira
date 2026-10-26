package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// DecodeWithPresence decodes the JSON body twice: into dst and into a raw
// map that tells absent keys from explicit nulls, mirroring Laravel's
// validated()-only field writes on partial updates. It reports whether the
// request was handled (422 already written on failure).
func DecodeWithPresence(w http.ResponseWriter, r *http.Request, dst any) (map[string]json.RawMessage, bool) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		Invalid(w, ValidationErrors{})
		return nil, false
	}

	var presence map[string]json.RawMessage
	if err := json.Unmarshal(raw, &presence); err != nil {
		Invalid(w, ValidationErrors{})
		return nil, false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			Invalid(w, ValidationErrors{typeErr.Field: {"The " + typeErr.Field + " field is invalid."}})
		} else {
			Invalid(w, ValidationErrors{})
		}
		return nil, false
	}

	return presence, true
}
