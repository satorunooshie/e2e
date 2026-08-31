package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// JSONValueModifier validates or normalizes a JSON value before golden comparison.
type JSONValueModifier interface {
	ModifyJSONValue(t *testing.T, value any) any
}

// Fields maps JSON object field names to modifiers.
type Fields map[string]JSONValueModifier

// JSONValueModifierFunc adapts a function to JSONValueModifier.
type JSONValueModifierFunc func(t *testing.T, value any) any

// ModifyJSONValue implements JSONValueModifier.
func (f JSONValueModifierFunc) ModifyJSONValue(t *testing.T, value any) any {
	t.Helper()

	return f(t, value)
}

// Format adapts a typed validation or normalization function to JSONValueModifier.
func Format[T, R any](format func(t *testing.T, value T) R) JSONValueModifierFunc {
	return func(t *testing.T, value any) any {
		t.Helper()

		typed, ok := value.(T)
		if !ok {
			var zero T
			t.Fatalf("JSON value has type %T, want %T", value, zero)
		}
		return format(t, typed)
	}
}

// Verify adapts a typed validation function to JSONValueModifier.
func Verify[T any](verify func(t *testing.T, value T)) JSONValueModifierFunc {
	return Format(func(t *testing.T, value T) T {
		t.Helper()
		verify(t, value)
		return value
	})
}

// ModifyJSONValue implements JSONValueModifier.
func (fields Fields) ModifyJSONValue(t *testing.T, value any) any {
	t.Helper()

	obj, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("JSON value has type %T, want map[string]any", value)
	}
	rewriteMap(t, obj, fields)
	return obj
}

// ReplaceWith returns a modifier that replaces the actual value with replaceWith.
func ReplaceWith(replaceWith any) JSONValueModifierFunc {
	return func(t *testing.T, value any) any {
		t.Helper()

		return replaceWith
	}
}

// ReplaceWith returns a modifier that validates or normalizes the actual value,
// then replaces it with replaceWith for golden comparison.
func (m JSONValueModifierFunc) ReplaceWith(replaceWith any) JSONValueModifierFunc {
	return func(t *testing.T, value any) any {
		t.Helper()

		m.ModifyJSONValue(t, value)
		return replaceWith
	}
}

type eachModifier struct {
	elem JSONValueModifier
}

// ModifyJSONValue implements JSONValueModifier.
func (m eachModifier) ModifyJSONValue(t *testing.T, value any) any {
	t.Helper()

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("JSON value has type %T, want []any", value)
	}
	for i, item := range items {
		items[i] = m.elem.ModifyJSONValue(t, item)
	}
	return items
}

// Each applies elem to every JSON array element.
func Each(elem JSONValueModifier) JSONValueModifier {
	return eachModifier{elem: elem}
}

func rewriteMap(t *testing.T, base map[string]any, overwrite Fields) {
	t.Helper()

	for key, modifier := range overwrite {
		if old, ok := base[key]; ok {
			base[key] = modifier.ModifyJSONValue(t, old)
		}
	}
}

// ModifyJSON modifies the specified JSON fields in the response body.
func ModifyJSON(overwrite Fields) ResponseFilter {
	return func(t *testing.T, r *http.Response) {
		t.Helper()

		var tmp map[string]any
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&tmp); err != nil {
			t.Fatal(err)
		}

		rewriteMap(t, tmp, overwrite)

		r.Body = io.NopCloser(bytes.NewReader(marshalJSON(t, tmp, "")))
	}
}
