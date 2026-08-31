package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewRoundTripClient(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "returns json response",
			statusCode: http.StatusOK,
			body:       `{"ok":true}`,
		},
		{
			name:       "returns error status json response",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"bad request"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewRoundTripClient(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/ping" {
					t.Fatalf("path = %q", r.URL.Path)
				}
				return NewJSONResponse(tt.statusCode, tt.body), nil
			})

			res, err := client.Get("https://example.com/ping")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := res.Body.Close(); err != nil {
					t.Errorf("close response body: %v", err)
				}
			}()

			if res.StatusCode != tt.statusCode {
				t.Fatalf("status code = %d, want %d", res.StatusCode, tt.statusCode)
			}
			if got := res.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q", got)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(body)) != tt.body {
				t.Fatalf("body = %q, want %q", body, tt.body)
			}
		})
	}
}
