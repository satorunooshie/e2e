package main

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	e2egrpc "github.com/satorunooshie/e2e/grpc/v2"
)

func ProfileID(t *testing.T, id int64) {
	t.Helper()

	if id <= 0 {
		t.Fatalf("profile id must be positive: %d", id)
	}
}

func UnixTime(t *testing.T, seconds int64) {
	t.Helper()

	if seconds <= 0 {
		t.Fatalf("Unix timestamp must be positive: %d", seconds)
	}
	if got := time.Unix(seconds, 0).UTC().Unix(); got != seconds {
		t.Fatalf("Unix timestamp = %d, want %d", got, seconds)
	}
}

func URL(t *testing.T, value string) {
	t.Helper()

	if _, err := url.ParseRequestURI(value); err != nil {
		t.Fatalf("gRPC field value is not a valid URL: %q", value)
	}
}

// URLValueModifier starts a URL modifier chain from base validation or normalization.
func URLValueModifier(base e2egrpc.ProtoValueModifier) urlValueModifier {
	return urlValueModifier{base: base}
}

func TestURLValueModifier(t *testing.T) {
	tests := []struct {
		name     string
		modifier e2egrpc.ProtoValueModifier
		value    string
		want     string
	}{
		{
			name:     "masks query values except kept key",
			modifier: URLValueModifier(e2egrpc.Verify(URL)).MaskQueryExceptKeys("region"),
			value:    "https://cdn.example.com/users/1/avatar.png?expires=1677136520&region=ap-northeast-1&signature=sig-123",
			want:     "https://cdn.example.com/users/1/avatar.png?expires=int&region=ap-northeast-1&signature=string",
		},
		{
			name:     "replaces path and masks all query values",
			modifier: URLValueModifier(e2egrpc.Verify(URL)).ReplaceURLPathWith("/download").MaskQueryExceptKeys(),
			value:    "https://cdn.example.com/users/1/avatar.png?expires=1677136520&signature=sig-123",
			want:     "https://cdn.example.com/download?expires=int&signature=string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.modifier.ModifyProtoValue(t, tt.value)
			if got != tt.want {
				t.Fatalf("URL modifier = %q, want %q", got, tt.want)
			}
		})
	}
}

type urlValueModifier struct {
	base           e2egrpc.ProtoValueModifier
	replaceURLPath string
	maskQuery      bool
	keepQueryKeys  []string
}

func (m urlValueModifier) ReplaceURLPathWith(replaceWith string) urlValueModifier {
	if !strings.HasPrefix(replaceWith, "/") {
		replaceWith = "/" + replaceWith
	}
	m.replaceURLPath = replaceWith
	return m
}

func (m urlValueModifier) MaskQueryExceptKeys(keepKeys ...string) e2egrpc.ProtoValueModifier {
	m.maskQuery = true
	m.keepQueryKeys = keepKeys
	return m
}

func (m urlValueModifier) ModifyProtoValue(t *testing.T, value any) any {
	t.Helper()

	value = m.base.ModifyProtoValue(t, value)
	rawURL, ok := value.(string)
	if !ok {
		t.Fatalf("gRPC field value has type %T, want string", value)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("gRPC field value is not a valid URL: %q", rawURL)
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
