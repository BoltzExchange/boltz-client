package macaroons

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gopkg.in/macaroon-bakery.v2/bakery/checkers"
	"strconv"
	"strings"
)

func (service *Service) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		var err error
		if ctx, err = service.validateRequest(ctx, info.FullMethod); err != nil {
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
		if _, err := service.validateRequest(ss.Context(), info.FullMethod); err != nil {
			return err
		}

		return handler(srv, ss)
	}
}

func (service *Service) validateRequest(ctx context.Context, fullMethod string) (context.Context, error) {
	requiredPermissions, foundPermissions := RPCServerPermissions[fullMethod]

	if !foundPermissions {
		return nil, errors.New("could not find permissions requires for method: " + fullMethod)
	}

	md, foundMetadata := metadata.FromIncomingContext(ctx)

	if !foundMetadata {
		return nil, errors.New("could not get metadata from context")
	}

	if len(md["macaroon"]) != 1 {
		return nil, errors.New("expected 1 macaroon, got " + strconv.Itoa(len(md["macaroon"])))
	}

	macBytes, err := hex.DecodeString(md["macaroon"][0])

	if err != nil {
		return nil, err
	}

	info, err := service.ValidateMacaroon(macBytes, requiredPermissions)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	var tenant string
	for _, caveat := range info.Conditions() {
		cond, arg, err := checkers.ParseCaveat(caveat)
		if err != nil {
			return nil, err
		}
		if cond == checkers.CondDeclared {
			split := strings.Split(arg, " ")
			if split[0] == string(tenantContextKey) {
				if split[1] != "" {
					tenant = split[1]
					break
				}
			}
		}
	}

	if param := md.Get(string(tenantContextKey)); len(param) > 0 {
		if len(param) != 1 {
			return nil, fmt.Errorf("expected 1 tenant, got %s", param)
		}
		if tenant != "" {
			return nil, status.Error(codes.InvalidArgument, "tenant restriction already set by macaroon")
		}
		tenant = param[0]
	}

	return service.validateTenant(ctx, tenant)
}
