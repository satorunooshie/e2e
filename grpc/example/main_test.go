package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	e2egrpc "github.com/satorunooshie/e2e/grpc/v2"
	"google.golang.org/grpc/codes"
)

const unaryTimeout = 3 * time.Second

func GRPCTestName(funcName string, code codes.Code, description ...string) string {
	elems := append([]string{"v1", code.String()}, description...)
	return filepath.Join(funcName, strings.Join(elems, "_"))
}

func TestProfileService(t *testing.T) {
	runner := e2egrpc.NewRunner(t, newServer(), e2egrpc.UseJSONNames(), e2egrpc.WithUnaryTimeout(unaryTimeout))
	client := NewProfileServiceClient(runner.Conn())

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
					"id":        e2egrpc.Verify(ProfileID),
					"createdAt": e2egrpc.Verify(UnixTime).ReplaceWith(int64(1677136520)),
					"uploadUrl": URLValueModifier(e2egrpc.Verify(URL)).MaskQueryExceptKeys("region"),
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
