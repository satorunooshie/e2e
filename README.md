# github.com/satorunooshie/e2e/v2

[![Go Reference](https://pkg.go.dev/badge/github.com/satorunooshie/e2e/v2.svg)](https://pkg.go.dev/github.com/satorunooshie/e2e/v2)

Library for golden-file e2e and scenario testing in Go.

HTTP tests run against `httptest.NewTestServer`; gRPC tests run against an
in-process `bufconn` server. No external ports are required.

## Install

```sh
go get github.com/satorunooshie/e2e/v2
```

For gRPC:

```sh
go get github.com/satorunooshie/e2e/grpc/v2
```

## Usage

After golden files are generated with `go test -e2e.golden`, e2e compares each
result against its golden file.

The minimum test case is two lines: one creates the request; the other executes the test.

`e2e` uses the same workflow for HTTP and gRPC:

| | HTTP | gRPC |
| --- | --- | --- |
| Module | `github.com/satorunooshie/e2e/v2` | `github.com/satorunooshie/e2e/grpc/v2` |
| Server | `httptest.NewTestServer` | `bufconn` |
| Call | `e2e.NewRequest` | generated gRPC client method |
| Assertion | `runner.RunTest` with an HTTP status | `runner.RunUnary` with a gRPC code |
| Golden output | HTTP response dump | method, status, headers, response, trailers |

## HTTP

After creating a runner, the minimum HTTP test case is two lines: one creates
the request; the other executes the test.

```go
func TestHealthEndpoint(t *testing.T) {
    runner := e2e.NewRunner(t, newRouter())

    r := e2e.NewRequest(http.MethodGet, "/v1/health", nil)
    runner.RunTest(t, r, http.StatusOK, e2e.PrettyJSON)
}
```

The golden file stores the HTTP status code, headers, and body dump.

## gRPC

gRPC support is published as a separate module so HTTP users do not take a
gRPC/protobuf dependency.

```go
import "github.com/satorunooshie/e2e/grpc/v2"

func TestGetProfile(t *testing.T) {
    runner := grpc.NewRunner(t, newServer(), grpc.UseJSONNames())
    client := pb.NewProfileServiceClient(runner.Conn())

    runner.RunUnary(t,
        pb.ProfileService_GetProfile_FullMethodName,
        &pb.GetProfileRequest{UserId: 1},
        codes.OK,
        client.GetProfile,
    )
}
```

The golden file stores the method, status, headers, response, and trailers.
Status details are decoded into JSON; `grpc-status-details-bin` is omitted from
the trailer dump.

## Golden Files

Generate or update golden files first:

```sh
go test -v ./... -e2e.golden
```

Then run tests without `-e2e.golden` to compare responses:

```sh
go test -v ./...
```

Use `-e2e.dump` to log the raw response while debugging.

## Filters

Filters run before golden comparison. Use `ModifyJSON` to replace
nondeterministic fields and `PrettyJSON` to make JSON golden files readable.

`Verify` and `Format` let tests validate or normalize real values before they
are written to the golden file:

```go
runner.RunTest(t, req, http.StatusCreated, e2e.ModifyJSON(e2e.Fields{
    "created_time": e2e.Verify(UnixTime).ReplaceWith(1677136520),
}))
```

Keep domain-specific rules in your test package. See the
[HTTP example](https://github.com/satorunooshie/e2e/blob/main/example/main_test.go)
and its
[custom JSON modifiers](https://github.com/satorunooshie/e2e/blob/main/example/e2e_helper_test.go).

For gRPC responses, use `ModifyResponse` with protobuf field names or JSON names:

```go
runner.RunUnary(t,
    pb.ProfileService_GetProfile_FullMethodName,
    &pb.GetProfileRequest{UserId: 1},
    codes.OK,
    client.GetProfile,
    grpc.ModifyResponse(grpc.Fields{
        "createdAt": grpc.Verify(UnixTime).ReplaceWith(int64(1677136520)),
    }),
)
```

See the [gRPC example](https://github.com/satorunooshie/e2e/blob/main/grpc/example/main_test.go)
for in-process server setup, metadata, trailers, status assertions, and response
normalization.

## Scenario Tests

Use `CaptureResponse` when a later request depends on an earlier response.

```go
func TestUserScenario(t *testing.T) {
    runner := e2e.NewRunner(t, newRouter())
    var created struct{ ID int }

    t.Run("1 create user", func(t *testing.T) {
        r := e2e.NewRequest(http.MethodPost, "/v1/users", e2e.JSONBody(t, map[string]any{
            "name": "Jo",
        }))

        runner.RunTest(t, r, http.StatusCreated,
            e2e.CaptureResponse(&created),
            e2e.ModifyJSON(e2e.Fields{
                "id":         e2e.ReplaceWith("user-id"),
                "created_at": e2e.ReplaceWith("2000-01-01T00:00:00Z"),
            }),
            e2e.PrettyJSON,
        )
    })

    t.Run("2 get user", func(t *testing.T) {
        r := e2e.NewRequest(http.MethodGet, "/v1/users/"+strconv.Itoa(created.ID), nil)
        runner.RunTest(t, r, http.StatusOK, e2e.PrettyJSON)
    })
}
```

## License

[MIT](LICENSE)
