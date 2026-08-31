package main

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/satorunooshie/e2e/v2"
)

// UnixTime validates that value is a JSON Unix timestamp.
func UnixTime(t *testing.T, value json.Number) {
	t.Helper()

	seconds, err := value.Int64()
	if err != nil {
		t.Fatalf("JSON value is not a valid Unix timestamp: %v", value)
	}
	if got := time.Unix(seconds, 0).UTC().Unix(); got != seconds {
		t.Fatalf("JSON value is not a valid Unix timestamp: %v", value)
	}
}

// URL validates that value is a URL string.
func URL(t *testing.T, value string) {
	t.Helper()

	if _, err := url.ParseRequestURI(value); err != nil {
		t.Fatalf("JSON value is not a valid URL: %q", value)
	}
}

// URLValueModifier starts a URL modifier chain from base validation or normalization.
func URLValueModifier(base e2e.JSONValueModifier) urlValueModifier {
	return urlValueModifier{base: base}
}

func TestURLValueModifier(t *testing.T) {
	tests := []struct {
		name     string
		modifier e2e.JSONValueModifier
		value    string
		want     string
	}{
		{
			name:     "masks query values except kept key",
			modifier: URLValueModifier(e2e.Verify(URL)).MaskQueryExceptKeys("region"),
			value:    "https://cdn.example.com/users/1/avatar.png?expires=1677136520&region=ap-northeast-1&signature=sig-123",
			want:     "https://cdn.example.com/users/1/avatar.png?expires=int&region=ap-northeast-1&signature=string",
		},
		{
			name:     "replaces path and masks all query values",
			modifier: URLValueModifier(e2e.Verify(URL)).ReplaceURLPathWith("/download").MaskQueryExceptKeys(),
			value:    "https://cdn.example.com/users/1/avatar.png?expires=1677136520&signature=sig-123",
			want:     "https://cdn.example.com/download?expires=int&signature=string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.modifier.ModifyJSONValue(t, tt.value)
			if got != tt.want {
				t.Fatalf("URL modifier = %q, want %q", got, tt.want)
			}
		})
	}
}

type urlValueModifier struct {
	base           e2e.JSONValueModifier
	replaceURLPath string
	maskQuery      bool
	keepQueryKeys  []string
}

// ReplaceURLPathWith replaces the URL path while preserving the origin and
// query string.
func (m urlValueModifier) ReplaceURLPathWith(replaceWith string) urlValueModifier {
	if !strings.HasPrefix(replaceWith, "/") {
		replaceWith = "/" + replaceWith
	}
	m.replaceURLPath = replaceWith
	return m
}

// MaskQueryExceptKeys replaces query parameter values with stable type
// placeholders. Keys passed as keepKeys keep their actual values. When keepKeys
// is empty, all query parameter values are masked.
func (m urlValueModifier) MaskQueryExceptKeys(keepKeys ...string) e2e.JSONValueModifier {
	m.maskQuery = true
	m.keepQueryKeys = keepKeys
	return m
}

func (m urlValueModifier) ModifyJSONValue(t *testing.T, value any) any {
	t.Helper()

	value = m.base.ModifyJSONValue(t, value)
	rawURL, ok := value.(string)
	if !ok {
		t.Fatalf("JSON value has type %T, want string", value)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("JSON value is not a valid URL: %q", rawURL)
	}

	if m.replaceURLPath != "" {
		u.Path = m.replaceURLPath
		u.RawPath = ""
	}
	if m.maskQuery {
		maskURLQueryExceptKeys(t, u, rawURL, m.keepQueryKeys)
	}
	return u.String()
}

func maskURLQueryExceptKeys(t *testing.T, u *url.URL, original string, keepKeys []string) {
	t.Helper()

	query := u.Query()
	keep := make(map[string]struct{}, len(keepKeys))
	for _, keepKey := range keepKeys {
		actualKey := findURLQueryKey(t, query, keepKey, original)
		keep[actualKey] = struct{}{}
	}
	for key, values := range query {
		if _, ok := keep[key]; ok {
			continue
		}
		for i, value := range values {
			values[i] = urlQueryValueType(value)
		}
		query[key] = values
	}
	u.RawQuery = query.Encode()
	u.ForceQuery = false
}

func findURLQueryKey(t *testing.T, query url.Values, keepKey, original string) string {
	t.Helper()

	if _, ok := query[keepKey]; ok {
		return keepKey
	}

	var matched string
	for key := range query {
		if !strings.EqualFold(key, keepKey) {
			continue
		}
		if matched != "" {
			t.Fatalf("URL query parameter %q is ambiguous in %q", keepKey, original)
		}
		matched = key
	}
	if matched == "" {
		t.Fatalf("URL query parameter %q not found in %q", keepKey, original)
	}
	return matched
}

func urlQueryValueType(value string) string {
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return "int"
	}
	return "string"
}
