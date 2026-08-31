package e2e

import (
	"net/http"
	"testing"
)

func TestRunnerRunTest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	tests := []struct {
		name string
		run  func(*testing.T, *http.Request)
	}{
		{
			name: "runs request against handler",
			run: func(t *testing.T, r *http.Request) {
				t.Helper()
				NewRunner(handler).RunTest(t, r, http.StatusOK, PrettyJSON)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t, NewRequest(http.MethodGet, "/health", nil))
		})
	}
}
