package e2e

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name    string
		options []RequestOption
		assert  func(*testing.T, *http.Request)
	}{
		{
			name:    "adds query values",
			options: []RequestOption{WithQuery("id", "1", "2")},
			assert: func(t *testing.T, r *http.Request) {
				t.Helper()
				got := r.URL.Query()["id"]
				if len(got) != 2 || got[0] != "1" || got[1] != "2" {
					t.Fatalf("query id = %v", got)
				}
			},
		},
		{
			name:    "sets arbitrary header",
			options: []RequestOption{WithHeader("X-Test", "ok")},
			assert: func(t *testing.T, r *http.Request) {
				t.Helper()
				if got := r.Header.Get("X-Test"); got != "ok" {
					t.Fatalf("X-Test = %q", got)
				}
			},
		},
		{
			name:    "sets content type",
			options: []RequestOption{WithContentType("application/json")},
			assert: func(t *testing.T, r *http.Request) {
				t.Helper()
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Fatalf("Content-Type = %q", got)
				}
			},
		},
		{
			name:    "sets bearer token",
			options: []RequestOption{WithBearerToken("token")},
			assert: func(t *testing.T, r *http.Request) {
				t.Helper()
				if got := r.Header.Get("Authorization"); got != "Bearer token" {
					t.Fatalf("Authorization = %q", got)
				}
			},
		},
		{
			name:    "adds cookies",
			options: []RequestOption{WithCookies(&http.Cookie{Name: "sid", Value: "abc"})},
			assert: func(t *testing.T, r *http.Request) {
				t.Helper()
				if got := r.Header.Get("Cookie"); got != "sid=abc" {
					t.Fatalf("Cookie = %q", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(http.MethodPost, "/users", strings.NewReader("body"), tt.options...)
			if req.Method != http.MethodPost {
				t.Fatalf("method = %q", req.Method)
			}
			if req.URL.Path != "/users" {
				t.Fatalf("path = %q", req.URL.Path)
			}
			if req.URL.Scheme != "http" {
				t.Fatalf("scheme = %q", req.URL.Scheme)
			}
			if req.URL.Host != "example.com" {
				t.Fatalf("host = %q", req.URL.Host)
			}
			tt.assert(t, req)
		})
	}
}

func TestJSONBody(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  map[string]any
	}{
		{
			name:  "encodes map",
			value: map[string]any{"name": "Jonathan"},
			want:  map[string]any{"name": "Jonathan"},
		},
		{
			name:  "encodes struct",
			value: struct{ Name string }{Name: "JoJo"},
			want:  map[string]any{"Name": "JoJo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := JSONBody(t, tt.value)

			var got map[string]any
			if err := json.NewDecoder(body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("JSON body mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMultipartBody(t *testing.T) {
	tests := []struct {
		name       string
		fields     []MultipartField
		assertForm func(*testing.T, *multipart.Form)
	}{
		{
			name: "writes text field",
			fields: []MultipartField{
				{FieldName: "name", TextValue: "Jonathan"},
			},
			assertForm: func(t *testing.T, form *multipart.Form) {
				t.Helper()
				if got := form.Value["name"]; len(got) != 1 || got[0] != "Jonathan" {
					t.Fatalf("form name = %v", got)
				}
			},
		},
		{
			name: "writes file field",
			fields: []MultipartField{
				{
					FieldName: "avatar",
					Content:   []byte("image-bytes"),
					FileName:  "avatar.png",
					MIMEType:  "image/png",
				},
			},
			assertForm: func(t *testing.T, form *multipart.Form) {
				t.Helper()
				files := form.File["avatar"]
				if len(files) != 1 {
					t.Fatalf("avatar file count = %d", len(files))
				}
				if files[0].Filename != "avatar.png" {
					t.Fatalf("avatar filename = %q", files[0].Filename)
				}
				if got := files[0].Header.Get("Content-Type"); got != "image/png" {
					t.Fatalf("avatar content type = %q", got)
				}
				file, err := files[0].Open()
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := file.Close(); err != nil {
						t.Errorf("close uploaded file: %v", err)
					}
				}()
				data, err := io.ReadAll(file)
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != "image-bytes" {
					t.Fatalf("avatar content = %q", data)
				}
			},
		},
		{
			name: "writes allowed empty file field",
			fields: []MultipartField{
				{FieldName: "empty", FileName: "empty.txt", AllowEmpty: true},
			},
			assertForm: func(t *testing.T, form *multipart.Form) {
				t.Helper()
				files := form.File["empty"]
				if len(files) != 1 {
					t.Fatalf("empty file count = %d", len(files))
				}
				if files[0].Size != 0 {
					t.Fatalf("empty file size = %d", files[0].Size)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, contentType := MultipartBody(t, tt.fields)
			mediaType, params, err := mime.ParseMediaType(contentType)
			if err != nil {
				t.Fatal(err)
			}
			if mediaType != "multipart/form-data" {
				t.Fatalf("media type = %q", mediaType)
			}

			reader := multipart.NewReader(body, params["boundary"])
			form, err := reader.ReadForm(1024)
			if err != nil {
				t.Fatal(err)
			}
			tt.assertForm(t, form)
		})
	}
}
