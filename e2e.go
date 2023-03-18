package e2e

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	router          http.Handler
	dumpRawResponse = flag.Bool("dump", false, "dump raw response")
	updateGolden    = flag.Bool("golden", false, "update golden files")
)

// RegisterRouter registers router for RunTest.
func RegisterRouter(rt http.Handler) {
	router = rt
}

// ResponseFilter is a function to modify HTTP response.
type ResponseFilter func(t *testing.T, r *http.Response)

// RunTest sends an HTTP request to router, then checks the status code and
// compare the response with the golden file. When `updateGolden` is true,
// update the golden file instead of comparison.
func RunTest(t *testing.T, r *http.Request, want int, filters ...ResponseFilter) {
	t.Helper()

	t.Logf(">>> %s %s\n", r.Method, r.URL)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	got := w.Result()
	if got.StatusCode != want {
		t.Errorf("HTTP StatusCode: %d, want: %d\n", got.StatusCode, want)
	}

	if *dumpRawResponse {
		var rc io.ReadCloser
		rc, got.Body = drainBody(t, got.Body)

		body, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(got.Header.Get("Content-Type"), "application/json") {
			switch got.StatusCode {
			case http.StatusOK, http.StatusCreated:
				body = indentJSON(t, body)
			}
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

	if *updateGolden {
		writeGolden(t, dump)
	} else {
		golden := readGolden(t)
		if diff := cmp.Diff(golden, dump); diff != "" {
			t.Errorf("HTTP Response mismatch (-want +got):\n%s", diff)
		}
	}

	t.Logf("<<< %s\n", goldenFileName(t.Name()))
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

	var tmp map[string]any
	if err := json.Unmarshal(body, &tmp); err != nil {
		t.Fatal(err)
	}
	body, err := json.MarshalIndent(&tmp, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func goldenFileName(name string) string {
	return filepath.Join("testdata", name+".golden")
}

func writeGolden(t *testing.T, data []byte) {
	t.Helper()

	filename := goldenFileName(t.Name())
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readGolden(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile(goldenFileName(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func rewriteMap(t *testing.T, base, overwrite map[string]any, parents ...string) {
	t.Helper()

	for k, v := range overwrite {
		if old, ok := base[k]; ok {
			switch v := v.(type) {
			case map[string]any:
				sub, ok := old.(map[string]any)
				if !ok {
					t.Fatalf("could not rewrite map: key = %q", strings.Join(append(parents, k), "."))
				}
				rewriteMap(t, sub, v, append(parents, k)...)
			case []map[string]any:
				sub, ok := old.([]any) // body is []any.
				if !ok {
					t.Fatalf("could not rewrite array map: key = %q", strings.Join(append(parents, k), "."))
				}
				if len(sub) != len(v) {
					t.Fatalf("could not rewrite array map: len(sub)=%d != len(v)=%d: key = %q",
						len(sub), len(v), strings.Join(append(parents, k), "."))
				}
				for i, vv := range v {
					kk := k + "#" + strconv.Itoa(i)
					sub2, ok := sub[i].(map[string]any)
					if !ok {
						t.Fatalf("could not rewrite array map: key = %q", strings.Join(append(parents, kk), "."))
					}
					rewriteMap(t, sub2, vv, append(parents, kk)...)
				}
			default:
				base[k] = v
			}
		}
	}
}

// ModifyJSON overwrites the specified key in the JSON field of the response
// body if it exists. When the map value of overwrite is map[string]any,
// change only the specified fields.
func ModifyJSON(overwrite map[string]any) ResponseFilter {
	return func(t *testing.T, r *http.Response) {
		t.Helper()

		var tmp map[string]any
		if err := json.NewDecoder(r.Body).Decode(&tmp); err != nil {
			t.Fatal(err)
		}

		rewriteMap(t, tmp, overwrite)

		body := new(bytes.Buffer)
		if err := json.NewEncoder(body).Encode(&tmp); err != nil {
			t.Fatal(err)
		}
		r.Body = io.NopCloser(body)
	}
}

// PrettyJSON is a ResponseFilter for formatting JSON responses. It adds
// indentation if the status code is not 204.
func PrettyJSON(t *testing.T, r *http.Response) {
	t.Helper()

	if r.StatusCode == http.StatusNoContent {
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		t.Fatal("Response is not JSON")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	r.Body = io.NopCloser(bytes.NewReader(indentJSON(t, body)))
}

// CaptureResponse unmarshals JSON response.
func CaptureResponse[T any](ptr *T) ResponseFilter {
	return func(t *testing.T, r *http.Response) {
		t.Helper()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err := json.Unmarshal(body, &ptr); err != nil {
			t.Fatal(err)
		}
	}
}

type RequestOption func(*http.Request)

// WithQuery sets query parameter.
func WithQuery(key string, values ...string) RequestOption {
	return func(r *http.Request) {
		q := r.URL.Query()
		for _, value := range values {
			q.Add(key, value)
		}
		r.URL.RawQuery = q.Encode()
	}
}

// WithHeader sets HTTP header.
func WithHeader(key, value string) RequestOption {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

// NewRequest creates a new HTTP request and applies options.
func NewRequest(method, endpoint string, body io.Reader, options ...RequestOption) *http.Request {
	r := httptest.NewRequest(method, endpoint, body)
	for _, opt := range options {
		opt(r)
	}
	return r
}

// JSONBody encodes m and returns it as an io.Reader.
func JSONBody(t *testing.T, m map[string]any) io.Reader {
	t.Helper()

	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(&m); err != nil {
		t.Fatal(err)
	}
	return body
}
