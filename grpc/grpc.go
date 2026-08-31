package grpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	defaultBufSize       = 1024 * 1024
	defaultUnaryTimeout  = 5 * time.Second
	grpcStatusDetailsBin = "grpc-status-details-bin"
)

// Runner runs gRPC end-to-end tests against an in-process server.
type Runner struct {
	server           *grpc.Server
	listener         *bufconn.Listener
	conn             *grpc.ClientConn
	protoJSONOptions protojson.MarshalOptions
	unaryTimeout     time.Duration
	goldenDir        string
	closeOnce        sync.Once
	closeErr         error
}

// RunnerOption configures a Runner.
type RunnerOption func(*Runner)

// UseJSONNames renders proto messages with JSON field names, including
// json_name. By default, proto field names are used.
func UseJSONNames() RunnerOption {
	return func(runner *Runner) {
		runner.protoJSONOptions.UseProtoNames = false
	}
}

// WithUnaryTimeout sets the timeout for RunUnary calls.
func WithUnaryTimeout(timeout time.Duration) RunnerOption {
	return func(runner *Runner) {
		runner.unaryTimeout = timeout
	}
}

// WithGoldenDir sets the directory for golden files.
func WithGoldenDir(dir string) RunnerOption {
	return func(runner *Runner) {
		if dir != "" {
			runner.goldenDir = dir
		}
	}
}

// NewRunner starts server on an in-process listener and returns a Runner. It
// registers a cleanup function with t to close the runner.
func NewRunner(t testing.TB, server *grpc.Server, options ...RunnerOption) *Runner {
	t.Helper()

	if server == nil {
		t.Fatal("gRPC server is nil")
		return nil
	}

	runner := &Runner{
		server:           server,
		protoJSONOptions: defaultProtoJSONOptions(),
		unaryTimeout:     defaultUnaryTimeout,
		goldenDir:        defaultGoldenDir,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		option(runner)
	}

	listener := bufconn.Listen(defaultBufSize)
	runner.listener = listener
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("serve gRPC test server: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough:///e2e-grpc",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		server.Stop()
		_ = listener.Close()
		t.Fatal(err)
		return nil
	}
	runner.conn = conn

	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Errorf("close gRPC runner: %v", err)
		}
	})

	return runner
}

// Conn returns the client connection to the runner's in-process server.
func (runner *Runner) Conn() *grpc.ClientConn {
	if runner == nil {
		return nil
	}
	return runner.conn
}

// Close releases the runner's client connection, server, and listener.
func (runner *Runner) Close() error {
	if runner == nil {
		return nil
	}

	runner.closeOnce.Do(func() {
		var closeErr error
		if runner.conn != nil {
			closeErr = runner.conn.Close()
		}
		if runner.server != nil {
			runner.server.Stop()
		}
		var listenerErr error
		if runner.listener != nil {
			listenerErr = runner.listener.Close()
		}
		runner.closeErr = errors.Join(closeErr, listenerErr)
	})
	return runner.closeErr
}

// ResponseFilter can normalize nondeterministic gRPC response fields before
// golden comparison.
type ResponseFilter interface {
	FilterResponse(t *testing.T, res proto.Message)
}

// ResponseFilterFunc adapts a typed response function to ResponseFilter.
type ResponseFilterFunc[Res proto.Message] func(t *testing.T, res Res)

// FilterResponse implements ResponseFilter.
func (f ResponseFilterFunc[Res]) FilterResponse(t *testing.T, res proto.Message) {
	t.Helper()

	typed, ok := res.(Res)
	if !ok {
		var zero Res
		t.Fatalf("gRPC response has type %T, want %T", res, zero)
	}
	f(t, typed)
}

// UnaryCall is a generated unary gRPC client method.
type UnaryCall[Req proto.Message, Res proto.Message] func(context.Context, Req, ...grpc.CallOption) (Res, error)

// RunUnary calls a unary gRPC method and compares its public response or status
// with the golden file.
func (runner *Runner) RunUnary[Req proto.Message, Res proto.Message](
	t *testing.T,
	method string,
	req Req,
	want codes.Code,
	call UnaryCall[Req, Res],
	filters ...ResponseFilter,
) {
	t.Helper()

	if runner == nil {
		t.Fatal("gRPC runner is not registered")
	}

	t.Logf(">>> %s\n", method)

	ctx, cancel := context.WithTimeout(context.Background(), runner.unaryTimeout)
	defer cancel()

	var headers, trailers metadata.MD
	res, err := call(ctx, req, grpc.Header(&headers), grpc.Trailer(&trailers))
	gotCode := status.Code(err)
	if gotCode != want {
		t.Errorf("gRPC code: %s, want: %s (err: %v)", gotCode, want, err)
	}

	if err == nil {
		for _, f := range filters {
			f.FilterResponse(t, res)
		}
	}

	dump := renderResult(t, runner.protoJSONOptions, method, gotCode, headers, res, trailers, err)
	if shouldDump() {
		t.Logf("Raw gRPC response:\n%s", dump)
	}

	updateOrCompareGolden(t, "gRPC response", dump, runner.goldenDir)

	t.Logf("<<< %s\n", goldenFileName(t, runner.goldenDir))
}

type grpcGolden struct {
	Method   string              `json:"method"`
	Status   grpcGoldenStatus    `json:"status"`
	Headers  map[string][]string `json:"headers"`
	Response json.RawMessage     `json:"response"`
	Trailers map[string][]string `json:"trailers"`
}

type grpcGoldenStatus struct {
	Code    string            `json:"code"`
	Number  codes.Code        `json:"number"`
	Message string            `json:"message"`
	Details []json.RawMessage `json:"details"`
}

func renderResult[Res proto.Message](
	t *testing.T,
	protoJSONOptions protojson.MarshalOptions,
	method string,
	code codes.Code,
	headers metadata.MD,
	res Res,
	trailers metadata.MD,
	err error,
) []byte {
	t.Helper()

	result := grpcGolden{
		Method:   method,
		Status:   grpcStatus(t, protoJSONOptions, code, err),
		Headers:  metadataMap(headers),
		Response: json.RawMessage("null"),
		Trailers: metadataMap(trailers),
	}
	if err != nil {
		return marshalJSON(t, result)
	}
	if !isNilProto(res) {
		result.Response = marshalProto(t, protoJSONOptions, res)
	}
	return marshalJSON(t, result)
}

func grpcStatus(t *testing.T, protoJSONOptions protojson.MarshalOptions, code codes.Code, err error) grpcGoldenStatus {
	t.Helper()

	result := grpcGoldenStatus{
		Code:    code.String(),
		Number:  code,
		Details: []json.RawMessage{},
	}
	if err == nil {
		return result
	}
	st := status.Convert(err)
	result.Message = st.Message()
	for _, detail := range st.Details() {
		msg, ok := detail.(proto.Message)
		if !ok {
			continue
		}
		result.Details = append(result.Details, marshalProto(t, protoJSONOptions, msg))
	}
	return result
}

func metadataMap(md metadata.MD) map[string][]string {
	result := make(map[string][]string, len(md))
	for key, values := range md {
		if key == grpcStatusDetailsBin {
			continue
		}
		if strings.HasSuffix(key, "-bin") {
			result[key] = encodedBinaryMetadata(values)
			continue
		}
		result[key] = append([]string(nil), values...)
	}
	return result
}

func encodedBinaryMetadata(values []string) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = base64.StdEncoding.EncodeToString([]byte(value))
	}
	return result
}

func marshalJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	return normalizeNewlines(data)
}

func marshalProto(t *testing.T, options protojson.MarshalOptions, msg proto.Message) []byte {
	t.Helper()

	data, err := options.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, data, "", "  "); err != nil {
		t.Fatal(err)
	}
	return formatted.Bytes()
}

func defaultProtoJSONOptions() protojson.MarshalOptions {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}
}

func isNilProto(msg proto.Message) bool {
	if msg == nil {
		return true
	}
	value := reflect.ValueOf(msg)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return true
		}
	}
	return !msg.ProtoReflect().IsValid()
}
