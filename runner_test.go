package e2e

import (
	"net/http"
	"testing"
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
