package rpcserver

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type PasswordAuth struct {
	password []byte
}

func NewPasswordAuth(password string) *PasswordAuth {
	return &PasswordAuth{[]byte(password)}
}

func (auth *PasswordAuth) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if err := auth.validateRequest(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func (auth *PasswordAuth) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if err := auth.validateRequest(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func (auth *PasswordAuth) validateRequest(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	authorization := md.Get("authorization")
	if len(authorization) != 1 {
		return status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if subtle.ConstantTimeCompare(auth.password, []byte(authorization[0])) == 0 {
		return status.Error(codes.Unauthenticated, "invalid authorization")
	}

	return nil
}
