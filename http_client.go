package e2e

import (
	"io"
	"net/http"
	"strconv"
	"strings"
)

// RoundTripFunc adapts a function to http.RoundTripper.
type RoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper.
func (f RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// NewRoundTripClient returns an HTTP client that uses fn as its transport.
func NewRoundTripClient(fn RoundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

// NewJSONResponse returns a JSON HTTP response for transport fakes.
func NewJSONResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode:    statusCode,
		Status:        strconv.Itoa(statusCode) + " " + http.StatusText(statusCode),
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}
