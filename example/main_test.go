package main

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/satorunooshie/e2e/v2"
)

const requestTimeout = 3 * time.Second

// APITestName returns golden file name.
// ex) v1_health_200_success.golden
func APITestName(endpoint string, code int, description ...string) string {
	return strings.Join(append([]string{strings.ReplaceAll(endpoint[1:], "/", "_"), strconv.Itoa(code)}, description...), "_")
}

// TestHealthEndpoint shows multiple endpoints example.
func TestHealthEndpoint(t *testing.T) {
	runner := e2e.NewRunner(t, newRouter(), e2e.WithClientTimeout(requestTimeout))
	testHealthEndpoint(t, runner, "/v1/health")
	testHealthEndpoint(t, runner, "/v2/health")
}

func testHealthEndpoint(t *testing.T, runner *e2e.Runner, endpoint string) {
	t.Helper()

	tests := []struct {
		want int
	}{
		{
			want: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(APITestName(endpoint, tt.want), func(t *testing.T) {
			r := e2e.NewRequest(http.MethodGet, endpoint, nil)
			runner.RunTest(t, r, tt.want, e2e.PrettyJSON)
		})
	}
}

func TestUserGetEndpoint(t *testing.T) {
	const endpoint = "/v1/user"
	runner := e2e.NewRunner(t, newRouter(), e2e.WithClientTimeout(requestTimeout))

	tests := []struct {
		description []string
		path        string
		opts        []e2e.RequestOption
		want        int
	}{
		{
			description: []string{"exception"},
			path:        "/1",
			opts:        []e2e.RequestOption{e2e.WithQuery("typ", "exception")},
			want:        http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(APITestName(endpoint, tt.want, tt.description...), func(t *testing.T) {
			endpoint := endpoint + tt.path
			r := e2e.NewRequest(http.MethodGet, endpoint, nil, tt.opts...)
			runner.RunTest(t, r, tt.want)
		})
	}
}

// TestUserPostEndpoint shows ModifyJSON example.
func TestUserPostEndpoint(t *testing.T) {
	const endpoint = "/v1/user"
	runner := e2e.NewRunner(t, newRouter(), e2e.WithClientTimeout(requestTimeout))

	tests := []struct {
		description []string
		body        map[string]any
		want        int
	}{
		{
			description: []string{"success"},
			body:        map[string]any{"name": "Jonathan Joestar"},
			want:        http.StatusCreated,
		},
	}
	for _, tt := range tests {
		t.Run(APITestName(endpoint, tt.want, tt.description...), func(t *testing.T) {
			r := e2e.NewRequest(http.MethodPost, endpoint, e2e.JSONBody(t, tt.body))
			runner.RunTest(t, r, tt.want, e2e.ModifyJSON(e2e.Fields{
				"created_time": e2e.Verify(UnixTime).ReplaceWith(1677136520),
				"upload_url":   URLValueModifier(e2e.Verify(URL)).MaskQueryExceptKeys("region"),
			}), e2e.PrettyJSON)
		})
	}
}

// TestUserPutEndpoint shows http.StatusNoContent example.
func TestUserPutEndpoint(t *testing.T) {
	const endpoint = "/v1/user"
	runner := e2e.NewRunner(t, newRouter(), e2e.WithClientTimeout(requestTimeout))

	tests := []struct {
		description []string
		path        string
		body        map[string]any
		want        int
	}{
		{
			description: []string{"success"},
			path:        "/1",
			body:        map[string]any{"name": "JoJo"},
			want:        http.StatusNoContent,
		},
	}
	for _, tt := range tests {
		t.Run(APITestName(endpoint, tt.want, tt.description...), func(t *testing.T) {
			endpoint := endpoint + tt.path
			r := e2e.NewRequest(http.MethodPut, endpoint, e2e.JSONBody(t, tt.body))
			runner.RunTest(t, r, tt.want)
		})
	}
}

// TestUserScenario shows a scenario testing example.
func TestUserScenario(t *testing.T) {
	runner := e2e.NewRunner(t, newRouter(), e2e.WithClientTimeout(requestTimeout))
	resp := struct{ ID int }{}
	// TestName: number methodName description
	t.Run("1 UserPost registration", func(t *testing.T) {
		const endpoint = "/v1/user"
		r := e2e.NewRequest(http.MethodPost, endpoint, e2e.JSONBody(t, map[string]any{"name": "JoJo"}))
		runner.RunTest(t, r, http.StatusCreated, e2e.CaptureResponse(&resp), e2e.ModifyJSON(e2e.Fields{
			"created_time": e2e.Verify(UnixTime).ReplaceWith(1677136520),
			"upload_url":   URLValueModifier(e2e.Verify(URL)).MaskQueryExceptKeys("region"),
		}), e2e.PrettyJSON)
	})
	t.Run("2 UserGet after registration", func(t *testing.T) {
		endpoint := "/v1/user/" + strconv.Itoa(resp.ID)
		r := e2e.NewRequest(http.MethodGet, endpoint, nil)
		runner.RunTest(t, r, http.StatusOK, e2e.PrettyJSON)
	})
	t.Run("3 UserPut update user name", func(t *testing.T) {
		endpoint := "/v1/user/" + strconv.Itoa(resp.ID)
		r := e2e.NewRequest(http.MethodPut, endpoint, e2e.JSONBody(t, map[string]any{"name": "Giorno Giovanna"}))
		runner.RunTest(t, r, http.StatusNoContent)
	})
	t.Run("4 UserGet after user name update", func(t *testing.T) {
		endpoint := "/v1/user/" + strconv.Itoa(resp.ID)
		r := e2e.NewRequest(http.MethodGet, endpoint, nil, e2e.WithQuery("typ", "new"))
		runner.RunTest(t, r, http.StatusOK, e2e.PrettyJSON)
	})
}
