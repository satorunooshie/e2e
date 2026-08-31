package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const grpcAddress = ":50051"

func main() {
	server := newServer()
	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		log.Fatalf("listen gRPC server: %v", err)
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("serve gRPC server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit

	server.GracefulStop()
}

func newServer() *grpc.Server {
	server := grpc.NewServer()
	RegisterProfileServiceServer(server, profileService{})
	return server
}

type profileService struct {
	UnimplementedProfileServiceServer
}

func (profileService) GetProfile(ctx context.Context, req *GetProfileRequest) (*GetProfileResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, invalidProfileRequestError("user_id", "must be positive")
	}
	if err := grpc.SendHeader(ctx, metadata.Pairs("x-request-id", "req-1")); err != nil {
		return nil, err
	}
	if err := grpc.SetTrailer(ctx, metadata.Pairs("x-response-source", "profile")); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &GetProfileResponse{
		Id:              req.GetUserId(),
		Name:            "Jonathan Joestar",
		CreatedUnixTime: now.Unix(),
		UploadUrl: fmt.Sprintf(
			"https://cdn.example.com/users/%d/avatar.png?expires=%d&region=ap-northeast-1&signature=sig-%d",
			req.GetUserId(),
			now.Add(15*time.Minute).Unix(),
			now.UnixNano(),
		),
	}, nil
}

func invalidProfileRequestError(field, description string) error {
	st := status.New(codes.InvalidArgument, "invalid profile request")
	st, err := st.WithDetails(&errdetails.BadRequest{
		FieldViolations: []*errdetails.BadRequest_FieldViolation{
			{
				Field:       field,
				Description: description,
			},
		},
	})
	if err != nil {
		return err
	}
	return st.Err()
}
