package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	e2egrpc "github.com/satorunooshie/e2e/grpc/v2"
	"google.golang.org/grpc/codes"
)

var (
	runner *e2egrpc.Runner
	client ProfileServiceClient
)

func TestMain(m *testing.M) {
	var err error
	runner, err = e2egrpc.NewRunner(newServer(), e2egrpc.UseJSONNames())
	if err != nil {
		log.Printf("new gRPC runner: %v", err)
		os.Exit(1)
	}
	client = NewProfileServiceClient(runner.Conn())

	code := m.Run()
	if err := runner.Close(); err != nil {
		log.Printf("close gRPC runner: %v", err)
		code = 1
	}
	os.Exit(code)
}

func GRPCTestName(funcName string, code codes.Code, description ...string) string {
	elems := append([]string{"v1", code.String()}, description...)
	return filepath.Join(funcName, strings.Join(elems, "_"))
}

func TestProfileService(t *testing.T) {
	tests := []struct {
		description []string
		request     *GetProfileRequest
		want        codes.Code
		filters     []e2egrpc.ResponseFilter
	}{
		{
			description: []string{"dynamic_fields_masked"},
			request: &GetProfileRequest{
				UserId: 1,
			},
			want: codes.OK,
			filters: []e2egrpc.ResponseFilter{
				e2egrpc.ModifyResponse(e2egrpc.Fields{
					"id":        e2egrpc.VerifyFormat(VerifyProfileID),
					"createdAt": e2egrpc.VerifyFormat(VerifyUnixtime).ReplaceWith(int64(1677136520)),
					"uploadUrl": MaskURL(e2egrpc.VerifyFormat(FormatURL)).MaskQueryExceptKeys("region"),
				}),
			},
		},
		{
			description: []string{"status_details"},
			request:     &GetProfileRequest{},
			want:        codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(GRPCTestName("GetProfile", tt.want, tt.description...), func(t *testing.T) {
			runner.RunUnary(
				t,
				ProfileService_GetProfile_FullMethodName,
				tt.request,
				tt.want,
				client.GetProfile,
				tt.filters...,
			)
		})
	}
}
