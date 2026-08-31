package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestPrettyJSON(t *testing.T) {
	tests := []struct {
		name     string
		response *http.Response
		wantBody string
	}{
		{
			name: "formats json response",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"name":"JoJo"}`)),
			},
			wantBody: "{\n  \"name\": \"JoJo\"\n}",
		},
		{
			name: "skips no content response",
			response: &http.Response{
				StatusCode: http.StatusNoContent,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader("")),
			},
			wantBody: "",
		},
		{
			name: "skips non json response",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("ok")),
			},
			wantBody: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			PrettyJSON(t, tt.response)

			body, err := io.ReadAll(tt.response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != tt.wantBody {
				t.Fatalf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestCaptureResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want struct {
			ID   int
			Name string
		}
	}{
		{
			name: "captures response and preserves body",
			body: `{"ID":1,"Name":"JoJo"}`,
			want: struct {
				ID   int
				Name string
			}{ID: 1, Name: "JoJo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &http.Response{Body: io.NopCloser(strings.NewReader(tt.body))}

			var got struct {
				ID   int
				Name string
			}
			CaptureResponse(&got)(t, res)
			if got != tt.want {
				t.Fatalf("captured response = %+v, want %+v", got, tt.want)
			}

			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != tt.body {
				t.Fatalf("body = %q, want %q", body, tt.body)
			}
		})
	}
}

func TestModifyCookies(t *testing.T) {
	tests := []struct {
		name      string
		cookies   []*http.Cookie
		overwrite map[string]string
		want      []string
	}{
		{
			name: "rewrites matching cookies",
			cookies: []*http.Cookie{
				{Name: "session", Value: "old", Path: "/", Expires: time.Unix(1677136520, 0)},
				{Name: "theme", Value: "dark", Path: "/"},
			},
			overwrite: map[string]string{"session": "stable"},
			want:      []string{"session=stable; Path=/", "theme=dark; Path=/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &http.Response{Header: http.Header{}}
			for _, cookie := range tt.cookies {
				res.Header.Add("Set-Cookie", cookie.String())
			}

			ModifyCookies(tt.overwrite)(t, res)

			got := res.Header.Values("Set-Cookie")
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Set-Cookie mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
