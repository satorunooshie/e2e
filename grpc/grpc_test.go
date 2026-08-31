package grpc

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	testpb "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestRenderResult(t *testing.T) {
	statusWithDetails := status.New(codes.PermissionDenied, "missing role")
	statusWithDetails, err := statusWithDetails.WithDetails(wrapperspb.String("admin"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name             string
		protoJSONOptions protojson.MarshalOptions
		method           string
		code             codes.Code
		headers          metadata.MD
		response         proto.Message
		trailers         metadata.MD
		err              error
		want             string
	}{
		{
			name:             "success renders status metadata response and trailers",
			protoJSONOptions: defaultProtoJSONOptions(),
			method:           "/example.Echo/Say",
			code:             codes.OK,
			headers:          metadata.Pairs("x-request-id", "req-1"),
			response:         wrapperspb.String("hello"),
			trailers:         metadata.Pairs("x-trailer", "done"),
			want: `{
  "method": "/example.Echo/Say",
  "status": {
    "code": "OK",
    "number": 0,
    "message": "",
    "details": []
  },
  "headers": {
    "x-request-id": [
      "req-1"
    ]
  },
  "response": "hello",
  "trailers": {
    "x-trailer": [
      "done"
    ]
  }
}
`,
		},
		{
			name:             "error renders status and proto details without response",
			protoJSONOptions: defaultProtoJSONOptions(),
			method:           "/example.Auth/Get",
			code:             codes.PermissionDenied,
			response:         (*emptypb.Empty)(nil),
			err:              statusWithDetails.Err(),
			want: `{
  "method": "/example.Auth/Get",
  "status": {
    "code": "PermissionDenied",
    "number": 7,
    "message": "missing role",
    "details": [
      "admin"
    ]
  },
  "headers": {},
  "response": null,
  "trailers": {}
}
`,
		},
		{
			name:             "success keeps nil response as null",
			protoJSONOptions: defaultProtoJSONOptions(),
			method:           "/example.Empty/Get",
			code:             codes.OK,
			response:         (*emptypb.Empty)(nil),
			want: `{
  "method": "/example.Empty/Get",
  "status": {
    "code": "OK",
    "number": 0,
    "message": "",
    "details": []
  },
  "headers": {},
  "response": null,
  "trailers": {}
}
`,
		},
		{
			name: "success renders JSON names when configured",
			protoJSONOptions: protojson.MarshalOptions{
				UseProtoNames: false,
			},
			method: "/example.Descriptor/Get",
			code:   codes.OK,
			response: &descriptorpb.FileDescriptorProto{
				SourceCodeInfo: &descriptorpb.SourceCodeInfo{},
			},
			want: `{
  "method": "/example.Descriptor/Get",
  "status": {
    "code": "OK",
    "number": 0,
    "message": "",
    "details": []
  },
  "headers": {},
  "response": {
    "sourceCodeInfo": {}
  },
  "trailers": {}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(renderResult(
				t,
				tt.protoJSONOptions,
				tt.method,
				tt.code,
				tt.headers,
				tt.response,
				tt.trailers,
				tt.err,
			))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("renderResult() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewRunner(t *testing.T) {
	tests := []struct {
		name              string
		server            func() *grpc.Server
		options           []RunnerOption
		wantUseProtoNames bool
		wantUnaryTimeout  time.Duration
	}{
		{
			name: "uses proto names by default",
			server: func() *grpc.Server {
				return grpc.NewServer()
			},
			wantUseProtoNames: true,
			wantUnaryTimeout:  defaultUnaryTimeout,
		},
		{
			name: "ignores nil option",
			server: func() *grpc.Server {
				return grpc.NewServer()
			},
			options:           []RunnerOption{nil},
			wantUseProtoNames: true,
			wantUnaryTimeout:  defaultUnaryTimeout,
		},
		{
			name: "uses JSON names when configured",
			server: func() *grpc.Server {
				return grpc.NewServer()
			},
			options:           []RunnerOption{UseJSONNames()},
			wantUseProtoNames: false,
			wantUnaryTimeout:  defaultUnaryTimeout,
		},
		{
			name: "uses unary timeout when configured",
			server: func() *grpc.Server {
				return grpc.NewServer()
			},
			options:           []RunnerOption{WithUnaryTimeout(2 * time.Second)},
			wantUseProtoNames: true,
			wantUnaryTimeout:  2 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(t, tt.server(), tt.options...)

			if runner.Conn() == nil {
				t.Fatal("Conn() is nil")
			}
			if got := runner.protoJSONOptions.UseProtoNames; got != tt.wantUseProtoNames {
				t.Fatalf("UseProtoNames = %t, want %t", got, tt.wantUseProtoNames)
			}
			if got := runner.unaryTimeout; got != tt.wantUnaryTimeout {
				t.Fatalf("unaryTimeout = %s, want %s", got, tt.wantUnaryTimeout)
			}
		})
	}
}

func TestModifyResponse(t *testing.T) {
	tests := []struct {
		name   string
		res    *testpb.SimpleResponse
		fields Fields
		want   *testpb.SimpleResponse
	}{
		{
			name: "formats and replaces scalar fields by proto and JSON names",
			res: &testpb.SimpleResponse{
				Username:   "before-filter",
				OauthScope: "users.read",
				ServerId:   "server-1",
			},
			fields: Fields{
				"username":    VerifyFormat(requireString("before-filter")).ReplaceWith("after-filter"),
				"oauth_scope": ReplaceWith("users.write"),
				"serverId": Format(func(t *testing.T, value string) string {
					t.Helper()
					if value != "server-1" {
						t.Fatalf("serverId = %q, want server-1", value)
					}
					return "server"
				}),
			},
			want: &testpb.SimpleResponse{
				Username:   "after-filter",
				OauthScope: "users.write",
				ServerId:   "server",
			},
		},
		{
			name: "rewrites nested message fields",
			res: &testpb.SimpleResponse{
				Payload: &testpb.Payload{
					Body: []byte("hello"),
				},
			},
			fields: Fields{
				"payload": Fields{
					"body": ReplaceWith([]byte("masked")),
				},
			},
			want: &testpb.SimpleResponse{
				Payload: &testpb.Payload{
					Body: []byte("masked"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ModifyResponse(tt.fields).FilterResponse(t, tt.res)

			if diff := cmp.Diff(tt.want, tt.res, protocmp.Transform()); diff != "" {
				t.Fatalf("ModifyResponse() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRunnerRunUnary(t *testing.T) {
	server := grpc.NewServer()
	testpb.RegisterTestServiceServer(server, runnerUnaryService{})

	runner := NewRunner(t, server)

	client := testpb.NewTestServiceClient(runner.Conn())
	tests := []struct {
		description []string
		request     *testpb.SimpleRequest
		want        codes.Code
		filters     []ResponseFilter
	}{
		{
			description: []string{"filters_response"},
			request: &testpb.SimpleRequest{
				Payload: &testpb.Payload{
					Body: []byte("hello"),
				},
			},
			want: codes.OK,
			filters: []ResponseFilter{
				ModifyResponse(Fields{
					"username": VerifyFormat(requireString("before-filter")).ReplaceWith("after-filter"),
					"serverId": VerifyFormat(requireString("server-1")).ReplaceWith("server"),
				}),
			},
		},
		{
			description: []string{"status"},
			request: &testpb.SimpleRequest{
				ResponseStatus: &testpb.EchoStatus{
					Code:    int32(codes.InvalidArgument),
					Message: "name is required",
				},
			},
			want: codes.InvalidArgument,
			filters: []ResponseFilter{
				ResponseFilterFunc[*testpb.SimpleResponse](func(t *testing.T, _ *testpb.SimpleResponse) {
					t.Fatal("filter was called for error response")
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(grpcTestName("UnaryCall", tt.want, tt.description...), func(t *testing.T) {
			runner.RunUnary(
				t,
				testpb.TestService_UnaryCall_FullMethodName,
				tt.request,
				tt.want,
				client.UnaryCall,
				tt.filters...,
			)
		})
	}
}

func TestRunnerClose(t *testing.T) {
	t.Run("nil runner is no-op", func(t *testing.T) {
		var runner *Runner
		if runner.Conn() != nil {
			t.Fatal("Conn() is not nil")
		}
		if err := runner.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("closes client connection", func(t *testing.T) {
		runner := NewRunner(t, grpc.NewServer())
		conn := runner.Conn()
		if conn == nil {
			t.Fatal("Conn() is nil")
		}
		if err := runner.Close(); err != nil {
			t.Fatal(err)
		}

		err := conn.Invoke(context.Background(), "/missing.Service/Method", &emptypb.Empty{}, &emptypb.Empty{})
		if err == nil {
			t.Fatal("Invoke() error is nil")
		}
		if got := status.Code(err); got != codes.Canceled {
			t.Fatalf("Invoke() code = %s, want %s (err: %v)", got, codes.Canceled, err)
		}
	})
}

func grpcTestName(funcName string, code codes.Code, description ...string) string {
	elems := append([]string{"v1", code.String()}, description...)
	return filepath.Join(funcName, strings.Join(elems, "_"))
}

type runnerUnaryService struct {
	testpb.UnimplementedTestServiceServer
}

func (runnerUnaryService) UnaryCall(ctx context.Context, req *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
	if responseStatus := req.GetResponseStatus(); responseStatus != nil {
		return nil, status.Error(codes.Code(responseStatus.GetCode()), responseStatus.GetMessage())
	}
	if err := grpc.SendHeader(ctx, metadata.Pairs("x-request-id", "req-1")); err != nil {
		return nil, err
	}
	if err := grpc.SetTrailer(ctx, metadata.Pairs("x-result", "stored")); err != nil {
		return nil, err
	}

	return &testpb.SimpleResponse{
		Payload:    req.GetPayload(),
		Username:   "before-filter",
		OauthScope: "users.read",
		ServerId:   "server-1",
	}, nil
}

func requireString(want string) func(t *testing.T, value string) {
	return func(t *testing.T, value string) {
		t.Helper()

		if value != want {
			t.Fatalf("gRPC field value = %q, want %q", value, want)
		}
	}
}
