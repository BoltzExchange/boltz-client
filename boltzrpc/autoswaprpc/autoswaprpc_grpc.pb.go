// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v5.26.0
// source: autoswaprpc/autoswaprpc.proto

package autoswaprpc

import (
	context "context"
	empty "github.com/golang/protobuf/ptypes/empty"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	AutoSwap_GetRecommendations_FullMethodName    = "/autoswaprpc.AutoSwap/GetRecommendations"
	AutoSwap_GetStatus_FullMethodName             = "/autoswaprpc.AutoSwap/GetStatus"
	AutoSwap_UpdateLightningConfig_FullMethodName = "/autoswaprpc.AutoSwap/UpdateLightningConfig"
	AutoSwap_UpdateChainConfig_FullMethodName     = "/autoswaprpc.AutoSwap/UpdateChainConfig"
	AutoSwap_GetConfig_FullMethodName             = "/autoswaprpc.AutoSwap/GetConfig"
	AutoSwap_ReloadConfig_FullMethodName          = "/autoswaprpc.AutoSwap/ReloadConfig"
)

// AutoSwapClient is the client API for AutoSwap service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AutoSwapClient interface {
	//
	//Returns a list of swaps which are currently recommended by autoswap. Also works when autoswap is not running.
	GetRecommendations(ctx context.Context, in *GetRecommendationsRequest, opts ...grpc.CallOption) (*GetRecommendationsResponse, error)
	//
	//Returns the current budget of autoswap and some relevant stats.
	GetStatus(ctx context.Context, in *GetStatusRequest, opts ...grpc.CallOption) (*GetStatusResponse, error)
	//
	//Partially updates the onchain configuration. The autoswapper will reload the configuration after this call.
	UpdateLightningConfig(ctx context.Context, in *UpdateLightningConfigRequest, opts ...grpc.CallOption) (*Config, error)
	//
	//Updates the lightning configuration completely or partially. Autoswap will reload the configuration after this call.
	UpdateChainConfig(ctx context.Context, in *UpdateChainConfigRequest, opts ...grpc.CallOption) (*Config, error)
	//
	//Returns the currently used configurationencoded as json.
	//If a key is specfied, only the value of that key will be returned.
	GetConfig(ctx context.Context, in *GetConfigRequest, opts ...grpc.CallOption) (*Config, error)
	//
	//Reloads the configuration from disk.
	ReloadConfig(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*Config, error)
}

type autoSwapClient struct {
	cc grpc.ClientConnInterface
}

func NewAutoSwapClient(cc grpc.ClientConnInterface) AutoSwapClient {
	return &autoSwapClient{cc}
}

func (c *autoSwapClient) GetRecommendations(ctx context.Context, in *GetRecommendationsRequest, opts ...grpc.CallOption) (*GetRecommendationsResponse, error) {
	out := new(GetRecommendationsResponse)
	err := c.cc.Invoke(ctx, AutoSwap_GetRecommendations_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *autoSwapClient) GetStatus(ctx context.Context, in *GetStatusRequest, opts ...grpc.CallOption) (*GetStatusResponse, error) {
	out := new(GetStatusResponse)
	err := c.cc.Invoke(ctx, AutoSwap_GetStatus_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *autoSwapClient) UpdateLightningConfig(ctx context.Context, in *UpdateLightningConfigRequest, opts ...grpc.CallOption) (*Config, error) {
	out := new(Config)
	err := c.cc.Invoke(ctx, AutoSwap_UpdateLightningConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *autoSwapClient) UpdateChainConfig(ctx context.Context, in *UpdateChainConfigRequest, opts ...grpc.CallOption) (*Config, error) {
	out := new(Config)
	err := c.cc.Invoke(ctx, AutoSwap_UpdateChainConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *autoSwapClient) GetConfig(ctx context.Context, in *GetConfigRequest, opts ...grpc.CallOption) (*Config, error) {
	out := new(Config)
	err := c.cc.Invoke(ctx, AutoSwap_GetConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *autoSwapClient) ReloadConfig(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*Config, error) {
	out := new(Config)
	err := c.cc.Invoke(ctx, AutoSwap_ReloadConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AutoSwapServer is the server API for AutoSwap service.
// All implementations must embed UnimplementedAutoSwapServer
// for forward compatibility
type AutoSwapServer interface {
	//
	//Returns a list of swaps which are currently recommended by autoswap. Also works when autoswap is not running.
	GetRecommendations(context.Context, *GetRecommendationsRequest) (*GetRecommendationsResponse, error)
	//
	//Returns the current budget of autoswap and some relevant stats.
	GetStatus(context.Context, *GetStatusRequest) (*GetStatusResponse, error)
	//
	//Partially updates the onchain configuration. The autoswapper will reload the configuration after this call.
	UpdateLightningConfig(context.Context, *UpdateLightningConfigRequest) (*Config, error)
	//
	//Updates the lightning configuration completely or partially. Autoswap will reload the configuration after this call.
	UpdateChainConfig(context.Context, *UpdateChainConfigRequest) (*Config, error)
	//
	//Returns the currently used configurationencoded as json.
	//If a key is specfied, only the value of that key will be returned.
	GetConfig(context.Context, *GetConfigRequest) (*Config, error)
	//
	//Reloads the configuration from disk.
	ReloadConfig(context.Context, *empty.Empty) (*Config, error)
	mustEmbedUnimplementedAutoSwapServer()
}

// UnimplementedAutoSwapServer must be embedded to have forward compatible implementations.
type UnimplementedAutoSwapServer struct {
}

func (UnimplementedAutoSwapServer) GetRecommendations(context.Context, *GetRecommendationsRequest) (*GetRecommendationsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRecommendations not implemented")
}
func (UnimplementedAutoSwapServer) GetStatus(context.Context, *GetStatusRequest) (*GetStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetStatus not implemented")
}
func (UnimplementedAutoSwapServer) UpdateLightningConfig(context.Context, *UpdateLightningConfigRequest) (*Config, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateLightningConfig not implemented")
}
func (UnimplementedAutoSwapServer) UpdateChainConfig(context.Context, *UpdateChainConfigRequest) (*Config, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateChainConfig not implemented")
}
func (UnimplementedAutoSwapServer) GetConfig(context.Context, *GetConfigRequest) (*Config, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetConfig not implemented")
}
func (UnimplementedAutoSwapServer) ReloadConfig(context.Context, *empty.Empty) (*Config, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReloadConfig not implemented")
}
func (UnimplementedAutoSwapServer) mustEmbedUnimplementedAutoSwapServer() {}

// UnsafeAutoSwapServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AutoSwapServer will
// result in compilation errors.
type UnsafeAutoSwapServer interface {
	mustEmbedUnimplementedAutoSwapServer()
}

func RegisterAutoSwapServer(s grpc.ServiceRegistrar, srv AutoSwapServer) {
	s.RegisterService(&AutoSwap_ServiceDesc, srv)
}

func _AutoSwap_GetRecommendations_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRecommendationsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AutoSwapServer).GetRecommendations(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AutoSwap_GetRecommendations_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AutoSwapServer).GetRecommendations(ctx, req.(*GetRecommendationsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AutoSwap_GetStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AutoSwapServer).GetStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AutoSwap_GetStatus_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AutoSwapServer).GetStatus(ctx, req.(*GetStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AutoSwap_UpdateLightningConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateLightningConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AutoSwapServer).UpdateLightningConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AutoSwap_UpdateLightningConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AutoSwapServer).UpdateLightningConfig(ctx, req.(*UpdateLightningConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AutoSwap_UpdateChainConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateChainConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AutoSwapServer).UpdateChainConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AutoSwap_UpdateChainConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AutoSwapServer).UpdateChainConfig(ctx, req.(*UpdateChainConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AutoSwap_GetConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AutoSwapServer).GetConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AutoSwap_GetConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AutoSwapServer).GetConfig(ctx, req.(*GetConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AutoSwap_ReloadConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AutoSwapServer).ReloadConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AutoSwap_ReloadConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AutoSwapServer).ReloadConfig(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// AutoSwap_ServiceDesc is the grpc.ServiceDesc for AutoSwap service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var AutoSwap_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "autoswaprpc.AutoSwap",
	HandlerType: (*AutoSwapServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetRecommendations",
			Handler:    _AutoSwap_GetRecommendations_Handler,
		},
		{
			MethodName: "GetStatus",
			Handler:    _AutoSwap_GetStatus_Handler,
		},
		{
			MethodName: "UpdateLightningConfig",
			Handler:    _AutoSwap_UpdateLightningConfig_Handler,
		},
		{
			MethodName: "UpdateChainConfig",
			Handler:    _AutoSwap_UpdateChainConfig_Handler,
		},
		{
			MethodName: "GetConfig",
			Handler:    _AutoSwap_GetConfig_Handler,
		},
		{
			MethodName: "ReloadConfig",
			Handler:    _AutoSwap_ReloadConfig_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "autoswaprpc/autoswaprpc.proto",
}
