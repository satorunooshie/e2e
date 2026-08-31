package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
)

type RequestOption func(*http.Request)

// WithQuery adds query parameters.
func WithQuery(key string, values ...string) RequestOption {
	return func(r *http.Request) {
		q := r.URL.Query()
		for _, value := range values {
			q.Add(key, value)
		}
		r.URL.RawQuery = q.Encode()
	}
}

// WithHeader sets an HTTP header.
func WithHeader(key, value string) RequestOption {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

// WithContentType sets the Content-Type header.
func WithContentType(contentType string) RequestOption {
	return func(r *http.Request) {
		r.Header.Set("Content-Type", contentType)
	}
}

// WithBearerToken sets the Authorization header with a Bearer token.
func WithBearerToken(token string) RequestOption {
	return func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+token)
	}
}

// WithCookies adds cookies to the request.
func WithCookies(cookies ...*http.Cookie) RequestOption {
	return func(r *http.Request) {
		for _, c := range cookies {
			r.AddCookie(c)
		}
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

// JSONBody encodes value and returns it as an io.Reader.
func JSONBody(t *testing.T, value any) io.Reader {
	t.Helper()

	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(value); err != nil {
		t.Fatal(err)
	}
	return body
}

type MultipartField struct {
	FieldName string

	TextValue string
	Content   []byte
	FileName  string
	MIMEType  string

	// AllowEmpty emits an intentionally empty file part.
	AllowEmpty bool
}

// MultipartBody creates a multipart form body with the specified fields.
func MultipartBody(t *testing.T, fields []MultipartField) (body io.Reader, contentType string) {
	t.Helper()

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	for _, field := range fields {
		if field.TextValue == "" && len(field.Content) == 0 && !field.AllowEmpty {
			t.Fatalf("field %q must have either TextValue or Content (set AllowEmpty for a zero-byte file part)", field.FieldName)
		}
		if field.TextValue != "" {
			if err := writer.WriteField(field.FieldName, field.TextValue); err != nil {
				t.Fatalf("failed to write text field %q: %v", field.FieldName, err)
			}
			continue
		}

		filename := field.FileName
		if filename == "" {
			filename = "filename"
		}
		mimeType := field.MIMEType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
			"name":     field.FieldName,
			"filename": filename,
		}))
		header.Set("Content-Type", mimeType)

		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("failed to create form part for field %q: %v", field.FieldName, err)
		}
		if _, err := part.Write(field.Content); err != nil {
			t.Fatalf("failed to write content for field %q: %v", field.FieldName, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	return buf, writer.FormDataContentType()
}
