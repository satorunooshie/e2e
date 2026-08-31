package grpc

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ProtoValueModifier validates or normalizes a protobuf field value before
// golden comparison.
type ProtoValueModifier interface {
	ModifyProtoValue(t *testing.T, value any) any
}

// Fields maps protobuf field names or JSON names to modifiers.
type Fields map[string]ProtoValueModifier

// ProtoValueModifierFunc adapts a function to ProtoValueModifier.
type ProtoValueModifierFunc func(t *testing.T, value any) any

// ModifyProtoValue implements ProtoValueModifier.
func (f ProtoValueModifierFunc) ModifyProtoValue(t *testing.T, value any) any {
	t.Helper()

	return f(t, value)
}

// Format adapts a typed validation or normalization function to
// ProtoValueModifier.
func Format[T, R any](format func(t *testing.T, value T) R) ProtoValueModifierFunc {
	return func(t *testing.T, value any) any {
		t.Helper()

		typed, ok := value.(T)
		if !ok {
			var zero T
			t.Fatalf("gRPC field value has type %T, want %T", value, zero)
		}
		return format(t, typed)
	}
}

// Verify adapts a typed validation function to ProtoValueModifier.
func Verify[T any](verify func(t *testing.T, value T)) ProtoValueModifierFunc {
	return Format(func(t *testing.T, value T) T {
		t.Helper()
		verify(t, value)
		return value
	})
}

// ModifyProtoValue implements ProtoValueModifier.
func (fields Fields) ModifyProtoValue(t *testing.T, value any) any {
	t.Helper()

	msg, ok := value.(proto.Message)
	if !ok {
		t.Fatalf("gRPC field value has type %T, want proto.Message", value)
	}
	modifyProtoFields(t, msg, fields)
	return msg
}

// ReplaceWith returns a modifier that replaces the actual value with
// replaceWith.
func ReplaceWith(replaceWith any) ProtoValueModifierFunc {
	return func(t *testing.T, value any) any {
		t.Helper()

		return replaceWith
	}
}

// ReplaceWith returns a modifier that validates or normalizes the actual value,
// then replaces it with replaceWith for golden comparison.
func (m ProtoValueModifierFunc) ReplaceWith(replaceWith any) ProtoValueModifierFunc {
	return func(t *testing.T, value any) any {
		t.Helper()

		m.ModifyProtoValue(t, value)
		return replaceWith
	}
}

type responseFieldsFilter struct {
	fields Fields
}

// FilterResponse implements ResponseFilter.
func (f responseFieldsFilter) FilterResponse(t *testing.T, res proto.Message) {
	t.Helper()

	f.fields.ModifyProtoValue(t, res)
}

// ModifyResponse modifies the specified protobuf response fields. Field names
// may be protobuf field names or JSON names. Fields with presence that are not
// set are ignored.
func ModifyResponse(fields Fields) ResponseFilter {
	return responseFieldsFilter{fields: fields}
}

func modifyProtoFields(t *testing.T, msg proto.Message, fields Fields) {
	t.Helper()

	if isNilProto(msg) {
		t.Fatal("gRPC response is nil")
	}

	protoMsg := msg.ProtoReflect()
	for name, modifier := range fields {
		field := findProtoField(t, protoMsg.Descriptor(), name)
		if field.HasPresence() && !protoMsg.Has(field) {
			continue
		}

		value := protoFieldValue(t, field, protoMsg.Get(field))
		modified := modifier.ModifyProtoValue(t, value)
		setProtoField(t, protoMsg, field, modified)
	}
}

func findProtoField(t *testing.T, desc protoreflect.MessageDescriptor, name string) protoreflect.FieldDescriptor {
	t.Helper()

	fields := desc.Fields()
	if field := fields.ByName(protoreflect.Name(name)); field != nil {
		return field
	}
	if field := fields.ByJSONName(name); field != nil {
		return field
	}
	t.Fatalf("gRPC field %q not found in %s", name, desc.FullName())
	return nil
}

func protoFieldValue(t *testing.T, field protoreflect.FieldDescriptor, value protoreflect.Value) any {
	t.Helper()

	if field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind {
		msg := value.Message()
		if !msg.IsValid() {
			return nil
		}
		return msg.Interface()
	}
	return value.Interface()
}

func setProtoField(t *testing.T, msg protoreflect.Message, field protoreflect.FieldDescriptor, value any) {
	t.Helper()

	if value == nil {
		msg.Clear(field)
		return
	}

	switch v := value.(type) {
	case proto.Message:
		if isNilProto(v) {
			msg.Clear(field)
			return
		}
		value = v.ProtoReflect()
	case protoreflect.Message:
		if !v.IsValid() {
			msg.Clear(field)
			return
		}
	case protoreflect.Value:
		setProtoReflectValue(t, msg, field, v, value)
		return
	}

	setProtoValue(t, msg, field, value)
}

func setProtoValue(t *testing.T, msg protoreflect.Message, field protoreflect.FieldDescriptor, value any) {
	t.Helper()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("gRPC field %q cannot be set from %T: %v", field.FullName(), value, recovered)
		}
	}()
	msg.Set(field, protoreflect.ValueOf(value))
}

func setProtoReflectValue(
	t *testing.T,
	msg protoreflect.Message,
	field protoreflect.FieldDescriptor,
	value protoreflect.Value,
	source any,
) {
	t.Helper()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("gRPC field %q cannot be set from %T: %v", field.FullName(), source, recovered)
		}
	}()
	msg.Set(field, value)
}
