package macaroons

import (
	"context"
	"errors"
	"google.golang.org/grpc"
)

func (service *Service) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if err := service.validateRequest(ctx, info.FullMethod); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

func (service *Service) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if err := service.validateRequest(ss.Context(), info.FullMethod); err != nil {
			return err
		}

		return handler(srv, ss)
	}
}

func (service *Service) validateRequest(ctx context.Context, fullMethod string) error {
	requiredPermissions, foundPermissions := RPCServerPermissions[fullMethod]

	if !foundPermissions {
		return errors.New("could not find permissions requires for method: " + fullMethod)
	}

	return service.ValidateMacaroon(ctx, requiredPermissions)
}
