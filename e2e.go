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
	"time"
)

// Runner runs HTTP end-to-end tests against a handler.
type Runner struct {
	client    *http.Client
	goldenDir string
}

type runnerConfig struct {
	configureServer []func(*http.Server)
	configureClient []func(*http.Client)
	goldenDir       string
}

// RunnerOption configures a Runner.
type RunnerOption func(*runnerConfig)

// WithServer applies configure to the runner's HTTP server before it is first
// used.
func WithServer(configure func(*http.Server)) RunnerOption {
	return func(config *runnerConfig) {
		if configure != nil {
			config.configureServer = append(config.configureServer, configure)
		}
	}
}

// WithClient applies configure to the runner's HTTP client.
func WithClient(configure func(*http.Client)) RunnerOption {
	return func(config *runnerConfig) {
		if configure != nil {
			config.configureClient = append(config.configureClient, configure)
		}
	}
}

// WithClientTimeout sets the runner's HTTP client timeout.
func WithClientTimeout(timeout time.Duration) RunnerOption {
	return WithClient(func(client *http.Client) {
		client.Timeout = timeout
	})
}

// FollowRedirects lets the runner's HTTP client follow redirects.
func FollowRedirects() RunnerOption {
	return WithClient(func(client *http.Client) {
		client.CheckRedirect = nil
	})
}

// WithGoldenDir sets the directory for golden files.
func WithGoldenDir(dir string) RunnerOption {
	return func(config *runnerConfig) {
		if dir != "" {
			config.goldenDir = dir
		}
	}
}

// NewRunner returns a Runner backed by httptest.NewTestServer.
func NewRunner(t testing.TB, handler http.Handler, options ...RunnerOption) *Runner {
	t.Helper()

	config := defaultRunnerConfig()
	for _, option := range options {
		if option == nil {
			continue
		}
		option(&config)
	}

	server := httptest.NewTestServer(t, handler)
	for _, configure := range config.configureServer {
		configure(server.Config)
	}

	client := server.Client()
	for _, configure := range config.configureClient {
		configure(client)
	}
	return &Runner{
		client:    client,
		goldenDir: config.goldenDir,
	}
}

func defaultRunnerConfig() runnerConfig {
	return runnerConfig{
		goldenDir: defaultGoldenDir,
		configureClient: []func(*http.Client){
			func(client *http.Client) {
				client.CheckRedirect = func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				}
			},
		},
	}
}

// ResponseFilter is a function to modify HTTP response.
type ResponseFilter func(t *testing.T, r *http.Response)

// RunTest sends an HTTP request through the runner's test server, then checks
// the status code and compares the response with the golden file. When
// -e2e.golden is set, RunTest updates the golden file instead of comparing it.
func (runner *Runner) RunTest(t *testing.T, r *http.Request, want int, filters ...ResponseFilter) {
	t.Helper()

	if runner == nil || runner.client == nil {
		t.Fatal("runner client is not registered")
	}

	t.Logf(">>> %s %s\n", r.Method, r.URL)

	got, err := runner.client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := got.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()
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

		dump, err := httputil.DumpResponse(got, false)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Raw response:\n%s%s\n", dump, body)
	}

	for _, f := range filters {
		f(t, got)
	}
	got.Header.Del("Date")
	resetResponseBody(t, got)

	dump, err := httputil.DumpResponse(got, true)
	if err != nil {
		t.Fatal(err)
	}

	updateOrCompareGolden(t, "HTTP response", dump, runner.goldenDir)

	t.Logf("<<< %s\n", goldenFileName(t, runner.goldenDir))
}

func resetResponseBody(t *testing.T, r *http.Response) {
	t.Helper()

	if r.Body == nil || r.Body == http.NoBody {
		r.Body = http.NoBody
		r.ContentLength = 0
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
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
