package macaroons

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
		return nil, err
	}

	for _, caveat := range info.Conditions() {
		cond, arg, err := checkers.ParseCaveat(caveat)
		if err != nil {
			return nil, err
		}
		if cond == checkers.CondDeclared {
			split := strings.Split(arg, " ")
			if split[0] == string(entityContextKey) {
				if split[1] != "" {
					entity, err := strconv.ParseInt(split[1], 10, 64)
					if err != nil {
						return nil, err
					}
					ctx = addEntityToContext(ctx, entity)
				}
			}
		}
	}

	if entity := md.Get("entity"); len(entity) > 0 {
		if len(entity) != 1 {
			return nil, fmt.Errorf("expected 1 entity, got %s", entity)
		}
		if ctx.Value(entityContextKey) != nil {
			return nil, errors.New("entity restriction already set by macaroon")
		}
		entity, err := strconv.ParseInt(entity[0], 10, 64)
		if err != nil {
			return nil, err
		}
		ctx = addEntityToContext(ctx, entity)
	}

	if entityId := EntityFromContext(ctx); entityId != nil {
		if _, err := service.Database.GetEntity(*entityId); err != nil {
			return nil, fmt.Errorf("invalid entity %d: %w", *entityId, err)
		}
	}

	return ctx, err
}
