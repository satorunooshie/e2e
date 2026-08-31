package e2e

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestRunnerRunTest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/health", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	tests := []struct {
		name string
		path string
		run  func(*testing.T, *http.Request)
	}{
		{
			name: "runs request against handler",
			path: "/health",
			run: func(t *testing.T, r *http.Request) {
				t.Helper()
				NewRunner(t, handler).RunTest(t, r, http.StatusOK, PrettyJSON)
			},
		},
		{
			name: "does not follow redirects",
			path: "/redirect",
			run: func(t *testing.T, r *http.Request) {
				t.Helper()
				NewRunner(t, handler).RunTest(t, r, http.StatusFound)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t, NewRequest(http.MethodGet, tt.path, nil))
		})
	}
}

func TestGoldenFileName(t *testing.T) {
	t.Run("uses default directory", func(t *testing.T) {
		got := goldenFileName(t, "")
		want := filepath.Join(defaultGoldenDir, t.Name()+".golden")

		if got != want {
			t.Errorf("goldenFileName() = %q, want %q", got, want)
		}
	})

	t.Run("uses configured directory", func(t *testing.T) {
		got := goldenFileName(t, "testdata/http")
		want := filepath.Join("testdata/http", t.Name()+".golden")

		if got != want {
			t.Errorf("goldenFileName() = %q, want %q", got, want)
		}
	})
}

func TestNewRunnerOptions(t *testing.T) {
	t.Run("ignores nil option", func(t *testing.T) {
		runner := NewRunner(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil)

		if runner == nil {
			t.Fatal("runner is nil")
		}
	})

	t.Run("sets client timeout", func(t *testing.T) {
		runner := NewRunner(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), WithClientTimeout(2*time.Second))

		if got := runner.client.Timeout; got != 2*time.Second {
			t.Errorf("client timeout = %s, want %s", got, 2*time.Second)
		}
	})

	t.Run("sets golden directory", func(t *testing.T) {
		runner := NewRunner(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), WithGoldenDir("testdata/http"))

		if got := runner.goldenDir; got != "testdata/http" {
			t.Errorf("goldenDir = %q, want %q", got, "testdata/http")
		}
	})

	t.Run("follows redirects when configured", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/redirect" {
				http.Redirect(w, r, "/done", http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		runner := NewRunner(t, handler, FollowRedirects())

		res, err := runner.client.Get("http://example.com/redirect")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := res.Body.Close(); err != nil {
				t.Errorf("close response body: %v", err)
			}
		})
		if got := res.StatusCode; got != http.StatusNoContent {
			t.Errorf("status code = %d, want %d", got, http.StatusNoContent)
		}
	})

	t.Run("configures server before client use", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server, ok := r.Context().Value(http.ServerContextKey).(*http.Server)
			if !ok {
				http.Error(w, "request context does not contain server", http.StatusInternalServerError)
				return
			}
			if got := server.ReadHeaderTimeout; got != time.Second {
				http.Error(w, "server ReadHeaderTimeout is not configured", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		runner := NewRunner(t, handler, WithServer(func(server *http.Server) {
			server.ReadHeaderTimeout = time.Second
		}))

		res, err := runner.client.Get("http://example.com/")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := res.Body.Close(); err != nil {
				t.Errorf("close response body: %v", err)
			}
		})
		if got := res.StatusCode; got != http.StatusNoContent {
			t.Errorf("status code = %d, want %d", got, http.StatusNoContent)
		}
	})
}
