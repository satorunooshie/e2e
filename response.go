package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"testing"
	"time"
)

// ModifyCookies overwrites Set-Cookie values by cookie name.
func ModifyCookies(overwrite map[string]string) ResponseFilter {
	return func(t *testing.T, r *http.Response) {
		t.Helper()

		cookies := r.Cookies()
		found := make(map[string]struct{}, len(overwrite))
		for _, cookie := range cookies {
			newValue, ok := overwrite[cookie.Name]
			if !ok {
				continue
			}
			cookie.Value = newValue
			cookie.Expires = time.Time{}
			found[cookie.Name] = struct{}{}
		}

		missing := make([]string, 0, len(overwrite))
		for name := range overwrite {
			if _, ok := found[name]; !ok {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("cookies not found in response: %v", missing)
		}

		r.Header.Del("Set-Cookie")
		for _, cookie := range cookies {
			r.Header.Add("Set-Cookie", cookie.String())
		}
	}
}

// PrettyJSON formats JSON responses. It skips empty responses and non-JSON
// responses.
func PrettyJSON(t *testing.T, r *http.Response) {
	t.Helper()

	switch r.StatusCode {
	case http.StatusNoContent, http.StatusAccepted:
		return
	}
	if !isJSONResponse(r) {
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	r.Body = io.NopCloser(bytes.NewReader(indentJSON(t, body)))
}

// CaptureResponse unmarshals a JSON response while preserving the response body
// for later filters and golden comparison.
func CaptureResponse[T any](ptr *T) ResponseFilter {
	return func(t *testing.T, r *http.Response) {
		t.Helper()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err := json.Unmarshal(body, ptr); err != nil {
			t.Fatal(err)
		}
	}
}
