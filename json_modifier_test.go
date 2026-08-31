package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestModifyJSON(t *testing.T) {
	requireString := Format(func(t *testing.T, value string) string {
		t.Helper()
		return value
	})

	requireInt := Verify(func(t *testing.T, value json.Number) {
		t.Helper()
		if _, err := value.Int64(); err != nil {
			t.Fatalf("JSON value is not an integer: %v", err)
		}
	})

	tests := []struct {
		name   string
		body   string
		fields Fields
		want   map[string]any
	}{
		{
			name: "formats and replaces scalar fields",
			body: `{
				"id": "0198f004-0000-7000-8000-000000000201",
				"created_at": 1677136520
			}`,
			fields: Fields{
				"id":         requireString.ReplaceWith("id"),
				"created_at": requireInt.ReplaceWith(json.Number("1677136520")),
			},
			want: map[string]any{
				"id":         "id",
				"created_at": json.Number("1677136520"),
			},
		},
		{
			name: "rewrites nested fields",
			body: `{
				"profile": {
					"display_name": "Jonathan",
					"rank": "hamon"
				}
			}`,
			fields: Fields{
				"profile": Fields{
					"display_name": ReplaceWith("name"),
				},
			},
			want: map[string]any{
				"profile": map[string]any{
					"display_name": "name",
					"rank":         "hamon",
				},
			},
		},
		{
			name: "rewrites array elements",
			body: `{
				"items": [
					{"id": "a", "name": "first"},
					{"id": "b", "name": "second"}
				]
			}`,
			fields: Fields{
				"items": Each(Fields{
					"id": ReplaceWith("item-id"),
				}),
			},
			want: map[string]any{
				"items": []any{
					map[string]any{"id": "item-id", "name": "first"},
					map[string]any{"id": "item-id", "name": "second"},
				},
			},
		},
		{
			name: "ignores missing fields",
			body: `{"id": "stable"}`,
			fields: Fields{
				"created_at": ReplaceWith(json.Number("1677136520")),
			},
			want: map[string]any{
				"id": "stable",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}

			ModifyJSON(tt.fields)(t, res)

			var got map[string]any
			dec := json.NewDecoder(res.Body)
			dec.UseNumber()
			if err := dec.Decode(&got); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("modified JSON mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
