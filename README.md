# github.com/satorunooshie/e2e
[![Go Reference](https://pkg.go.dev/badge/github.com/satorunooshie/e2e.svg)](https://pkg.go.dev/github.com/satorunooshie/e2e)

Library for e2e and scenario testing.

## Usage

Once a golden file generated by go test with golden flag, e2e compares HTTP status code and the response with the golden file.

Need at least only two lines, new request and run test, as below.

e2e testing only needs a minimum of two lines of code; one that creates an HTTP request and the other that executes the test.

```go
t.Run(APITestName, func(t *testing.T) {
    r := e2e.NewRequest(http.MethodGet, endpoint, nil)
    e2e.RunTest(t, r, http.StatusOK, e2e.PrettyJSON)
})
```

For more detail, see [examples](https://github.com/satorunooshie/e2e/blob/main/example/main_test.go).
