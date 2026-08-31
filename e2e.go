package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"testing"
)

// Runner runs HTTP end-to-end tests against a handler.
type Runner struct {
	handler http.Handler
}

// NewRunner returns a Runner that sends requests to handler.
func NewRunner(handler http.Handler) *Runner {
	return &Runner{handler: handler}
}

// ResponseFilter is a function to modify HTTP response.
type ResponseFilter func(t *testing.T, r *http.Response)

// RunTest sends an HTTP request to the runner's handler, then checks the status
// code and compares the response with the golden file. When -e2e.golden is
// set, RunTest updates the golden file instead of comparing it.
func (runner *Runner) RunTest(t *testing.T, r *http.Request, want int, filters ...ResponseFilter) {
	t.Helper()

	if runner == nil || runner.handler == nil {
		t.Fatal("runner handler is not registered")
	}

	t.Logf(">>> %s %s\n", r.Method, r.URL)

	w := httptest.NewRecorder()
	runner.handler.ServeHTTP(w, r)

	got := w.Result()
	if got.StatusCode != want {
		t.Errorf("HTTP status code: %d, want: %d\n", got.StatusCode, want)
	}

	if shouldDump() {
		var rc io.ReadCloser
		rc, got.Body = drainBody(t, got.Body)

		body, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		if isJSONResponse(got) && got.StatusCode != http.StatusNoContent {
			body = indentJSON(t, body)
		}

		dump, err := httputil.DumpResponse(got, false)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Raw response:\n%s%s\n", dump, body)
	}

	for _, f := range filters {
		f(t, got)
	}

	dump, err := httputil.DumpResponse(got, true)
	if err != nil {
		t.Fatal(err)
	}

	updateOrCompareGolden(t, "HTTP response", dump)

	t.Logf("<<< %s\n", goldenFileName(t))
}

// This is a modified version of httputil.drainBody for this test.
func drainBody(t *testing.T, b io.ReadCloser) (dump, orig io.ReadCloser) {
	t.Helper()

	if b == nil || b == http.NoBody {
		return http.NoBody, http.NoBody
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(b); err != nil {
		t.Fatal(err)
	}
	_ = b.Close()
	return io.NopCloser(&buf), io.NopCloser(bytes.NewReader(buf.Bytes()))
}

func indentJSON(t *testing.T, body []byte) []byte {
	t.Helper()

	var tmp any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&tmp); err != nil {
		t.Fatal(err)
	}
	return marshalJSON(t, tmp, "  ")
}

func isJSONResponse(r *http.Response) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "application/json")
}

func marshalJSON(t *testing.T, value any, indent string) []byte {
	t.Helper()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(value); err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}
