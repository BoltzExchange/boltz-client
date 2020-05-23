// Code generated by protoc-gen-go. DO NOT EDIT.
// source: rpc.proto

package boltzrpc

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type GetInfoRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetInfoRequest) Reset()         { *m = GetInfoRequest{} }
func (m *GetInfoRequest) String() string { return proto.CompactTextString(m) }
func (*GetInfoRequest) ProtoMessage()    {}
func (*GetInfoRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_77a6da22d6a3feb1, []int{0}
}

func (m *GetInfoRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetInfoRequest.Unmarshal(m, b)
}
func (m *GetInfoRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetInfoRequest.Marshal(b, m, deterministic)
}
func (m *GetInfoRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetInfoRequest.Merge(m, src)
}
func (m *GetInfoRequest) XXX_Size() int {
	return xxx_messageInfo_GetInfoRequest.Size(m)
}
func (m *GetInfoRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GetInfoRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GetInfoRequest proto.InternalMessageInfo

type GetInfoResponse struct {
	Symbol               string   `protobuf:"bytes,1,opt,name=symbol,proto3" json:"symbol,omitempty"`
	LndPubkey            string   `protobuf:"bytes,2,opt,name=lnd_pubkey,json=lndPubkey,proto3" json:"lnd_pubkey,omitempty"`
	PendingSwaps         []string `protobuf:"bytes,3,rep,name=pending_swaps,json=pendingSwaps,proto3" json:"pending_swaps,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetInfoResponse) Reset()         { *m = GetInfoResponse{} }
func (m *GetInfoResponse) String() string { return proto.CompactTextString(m) }
func (*GetInfoResponse) ProtoMessage()    {}
func (*GetInfoResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_77a6da22d6a3feb1, []int{1}
}

func (m *GetInfoResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetInfoResponse.Unmarshal(m, b)
}
func (m *GetInfoResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetInfoResponse.Marshal(b, m, deterministic)
}
func (m *GetInfoResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetInfoResponse.Merge(m, src)
}
func (m *GetInfoResponse) XXX_Size() int {
	return xxx_messageInfo_GetInfoResponse.Size(m)
}
func (m *GetInfoResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GetInfoResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GetInfoResponse proto.InternalMessageInfo

func (m *GetInfoResponse) GetSymbol() string {
	if m != nil {
		return m.Symbol
	}
	return ""
}

func (m *GetInfoResponse) GetLndPubkey() string {
	if m != nil {
		return m.LndPubkey
	}
	return ""
}

func (m *GetInfoResponse) GetPendingSwaps() []string {
	if m != nil {
		return m.PendingSwaps
	}
	return nil
}

type GetSwapInfoRequest struct {
	Id                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetSwapInfoRequest) Reset()         { *m = GetSwapInfoRequest{} }
func (m *GetSwapInfoRequest) String() string { return proto.CompactTextString(m) }
func (*GetSwapInfoRequest) ProtoMessage()    {}
func (*GetSwapInfoRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_77a6da22d6a3feb1, []int{2}
}

func (m *GetSwapInfoRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetSwapInfoRequest.Unmarshal(m, b)
}
func (m *GetSwapInfoRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetSwapInfoRequest.Marshal(b, m, deterministic)
}
func (m *GetSwapInfoRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetSwapInfoRequest.Merge(m, src)
}
func (m *GetSwapInfoRequest) XXX_Size() int {
	return xxx_messageInfo_GetSwapInfoRequest.Size(m)
}
func (m *GetSwapInfoRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GetSwapInfoRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GetSwapInfoRequest proto.InternalMessageInfo

func (m *GetSwapInfoRequest) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

type GetSwapInfoResponse struct {
	Id                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Status               string   `protobuf:"bytes,2,opt,name=status,proto3" json:"status,omitempty"`
	PrivateKey           string   `protobuf:"bytes,3,opt,name=privateKey,proto3" json:"privateKey,omitempty"`
	Preimage             string   `protobuf:"bytes,4,opt,name=preimage,proto3" json:"preimage,omitempty"`
	RedeemScript         string   `protobuf:"bytes,5,opt,name=redeem_script,json=redeemScript,proto3" json:"redeem_script,omitempty"`
	Invoice              string   `protobuf:"bytes,6,opt,name=invoice,proto3" json:"invoice,omitempty"`
	Address              string   `protobuf:"bytes,7,opt,name=address,proto3" json:"address,omitempty"`
	ExpectedAmount       int64    `protobuf:"varint,8,opt,name=expected_amount,json=expectedAmount,proto3" json:"expected_amount,omitempty"`
	TimeoutBlockHeight   uint32   `protobuf:"varint,9,opt,name=timeout_block_height,json=timeoutBlockHeight,proto3" json:"timeout_block_height,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetSwapInfoResponse) Reset()         { *m = GetSwapInfoResponse{} }
func (m *GetSwapInfoResponse) String() string { return proto.CompactTextString(m) }
func (*GetSwapInfoResponse) ProtoMessage()    {}
func (*GetSwapInfoResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_77a6da22d6a3feb1, []int{3}
}

func (m *GetSwapInfoResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetSwapInfoResponse.Unmarshal(m, b)
}
func (m *GetSwapInfoResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetSwapInfoResponse.Marshal(b, m, deterministic)
}
func (m *GetSwapInfoResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetSwapInfoResponse.Merge(m, src)
}
func (m *GetSwapInfoResponse) XXX_Size() int {
	return xxx_messageInfo_GetSwapInfoResponse.Size(m)
}
func (m *GetSwapInfoResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GetSwapInfoResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GetSwapInfoResponse proto.InternalMessageInfo

func (m *GetSwapInfoResponse) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *GetSwapInfoResponse) GetStatus() string {
	if m != nil {
		return m.Status
	}
	return ""
}

func (m *GetSwapInfoResponse) GetPrivateKey() string {
	if m != nil {
		return m.PrivateKey
	}
	return ""
}

func (m *GetSwapInfoResponse) GetPreimage() string {
	if m != nil {
		return m.Preimage
	}
	return ""
}

func (m *GetSwapInfoResponse) GetRedeemScript() string {
	if m != nil {
		return m.RedeemScript
	}
	return ""
}

func (m *GetSwapInfoResponse) GetInvoice() string {
	if m != nil {
		return m.Invoice
	}
	return ""
}

func (m *GetSwapInfoResponse) GetAddress() string {
	if m != nil {
		return m.Address
	}
	return ""
}

func (m *GetSwapInfoResponse) GetExpectedAmount() int64 {
	if m != nil {
		return m.ExpectedAmount
	}
	return 0
}

func (m *GetSwapInfoResponse) GetTimeoutBlockHeight() uint32 {
	if m != nil {
		return m.TimeoutBlockHeight
	}
	return 0
}

type CreateSwapRequest struct {
	Amount               int64    `protobuf:"varint,1,opt,name=amount,proto3" json:"amount,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CreateSwapRequest) Reset()         { *m = CreateSwapRequest{} }
func (m *CreateSwapRequest) String() string { return proto.CompactTextString(m) }
func (*CreateSwapRequest) ProtoMessage()    {}
func (*CreateSwapRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_77a6da22d6a3feb1, []int{4}
}

func (m *CreateSwapRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CreateSwapRequest.Unmarshal(m, b)
}
func (m *CreateSwapRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CreateSwapRequest.Marshal(b, m, deterministic)
}
func (m *CreateSwapRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CreateSwapRequest.Merge(m, src)
}
func (m *CreateSwapRequest) XXX_Size() int {
	return xxx_messageInfo_CreateSwapRequest.Size(m)
}
func (m *CreateSwapRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CreateSwapRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CreateSwapRequest proto.InternalMessageInfo

func (m *CreateSwapRequest) GetAmount() int64 {
	if m != nil {
		return m.Amount
	}
	return 0
}

type CreateSwapResponse struct {
	Id                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Address              string   `protobuf:"bytes,2,opt,name=address,proto3" json:"address,omitempty"`
	ExpectedAmount       int64    `protobuf:"varint,3,opt,name=expected_amount,json=expectedAmount,proto3" json:"expected_amount,omitempty"`
	Bip21                string   `protobuf:"bytes,4,opt,name=bip21,proto3" json:"bip21,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CreateSwapResponse) Reset()         { *m = CreateSwapResponse{} }
func (m *CreateSwapResponse) String() string { return proto.CompactTextString(m) }
func (*CreateSwapResponse) ProtoMessage()    {}
func (*CreateSwapResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_77a6da22d6a3feb1, []int{5}
}

func (m *CreateSwapResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CreateSwapResponse.Unmarshal(m, b)
}
func (m *CreateSwapResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CreateSwapResponse.Marshal(b, m, deterministic)
}
func (m *CreateSwapResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CreateSwapResponse.Merge(m, src)
}
func (m *CreateSwapResponse) XXX_Size() int {
	return xxx_messageInfo_CreateSwapResponse.Size(m)
}
func (m *CreateSwapResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_CreateSwapResponse.DiscardUnknown(m)
}

var xxx_messageInfo_CreateSwapResponse proto.InternalMessageInfo

func (m *CreateSwapResponse) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *CreateSwapResponse) GetAddress() string {
	if m != nil {
		return m.Address
	}
	return ""
}

func (m *CreateSwapResponse) GetExpectedAmount() int64 {
	if m != nil {
		return m.ExpectedAmount
	}
	return 0
}

func (m *CreateSwapResponse) GetBip21() string {
	if m != nil {
		return m.Bip21
	}
	return ""
}

func init() {
	proto.RegisterType((*GetInfoRequest)(nil), "boltzrpc.GetInfoRequest")
	proto.RegisterType((*GetInfoResponse)(nil), "boltzrpc.GetInfoResponse")
	proto.RegisterType((*GetSwapInfoRequest)(nil), "boltzrpc.GetSwapInfoRequest")
	proto.RegisterType((*GetSwapInfoResponse)(nil), "boltzrpc.GetSwapInfoResponse")
	proto.RegisterType((*CreateSwapRequest)(nil), "boltzrpc.CreateSwapRequest")
	proto.RegisterType((*CreateSwapResponse)(nil), "boltzrpc.CreateSwapResponse")
}

func init() { proto.RegisterFile("rpc.proto", fileDescriptor_77a6da22d6a3feb1) }

var fileDescriptor_77a6da22d6a3feb1 = []byte{
	// 446 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x93, 0xdb, 0x6e, 0xd3, 0x40,
	0x10, 0x86, 0xe5, 0x98, 0x1c, 0x3c, 0xb4, 0x29, 0x0c, 0x55, 0xb5, 0x84, 0x16, 0x45, 0x06, 0x89,
	0x48, 0x48, 0x11, 0x94, 0x7b, 0x24, 0xca, 0x45, 0x38, 0xdc, 0x20, 0xf7, 0x01, 0x2c, 0xdb, 0x3b,
	0xa4, 0xab, 0xda, 0xde, 0x65, 0x77, 0x5d, 0x08, 0x12, 0x8f, 0xc4, 0x1b, 0xf1, 0x30, 0xc8, 0x9b,
	0x4d, 0xeb, 0x94, 0x54, 0xdc, 0xf9, 0xff, 0xbf, 0xd1, 0xce, 0x41, 0xbf, 0x21, 0xd2, 0xaa, 0x98,
	0x2b, 0x2d, 0xad, 0xc4, 0x51, 0x2e, 0x4b, 0xfb, 0x53, 0xab, 0x22, 0x7e, 0x00, 0xe3, 0x05, 0xd9,
	0x8f, 0xf5, 0x57, 0x99, 0xd0, 0xb7, 0x86, 0x8c, 0x8d, 0x2b, 0x38, 0xb8, 0x76, 0x8c, 0x92, 0xb5,
	0x21, 0x3c, 0x82, 0x81, 0x59, 0x55, 0xb9, 0x2c, 0x59, 0x30, 0x0d, 0x66, 0x51, 0xe2, 0x15, 0x9e,
	0x00, 0x94, 0x35, 0x4f, 0x55, 0x93, 0x5f, 0xd2, 0x8a, 0xf5, 0x1c, 0x8b, 0xca, 0x9a, 0x7f, 0x71,
	0x06, 0x3e, 0x83, 0x7d, 0x45, 0x35, 0x17, 0xf5, 0x32, 0x35, 0xdf, 0x33, 0x65, 0x58, 0x38, 0x0d,
	0x67, 0x51, 0xb2, 0xe7, 0xcd, 0xf3, 0xd6, 0x8b, 0x9f, 0x03, 0x2e, 0xc8, 0xb6, 0xdf, 0x9d, 0x21,
	0x70, 0x0c, 0x3d, 0xc1, 0x7d, 0xb7, 0x9e, 0xe0, 0xf1, 0xef, 0x1e, 0x3c, 0xda, 0x2a, 0xf3, 0x93,
	0xdd, 0xaa, 0x73, 0x93, 0xda, 0xcc, 0x36, 0xc6, 0x4f, 0xe3, 0x15, 0x3e, 0x05, 0x50, 0x5a, 0x5c,
	0x65, 0x96, 0x3e, 0xd3, 0x8a, 0x85, 0x8e, 0x75, 0x1c, 0x9c, 0xc0, 0x48, 0x69, 0x12, 0x55, 0xb6,
	0x24, 0x76, 0xcf, 0xd1, 0x6b, 0xdd, 0xae, 0xa1, 0x89, 0x13, 0x55, 0xa9, 0x29, 0xb4, 0x50, 0x96,
	0xf5, 0x5d, 0xc1, 0xde, 0xda, 0x3c, 0x77, 0x1e, 0x32, 0x18, 0x8a, 0xfa, 0x4a, 0x8a, 0x82, 0xd8,
	0xc0, 0xe1, 0x8d, 0x6c, 0x49, 0xc6, 0xb9, 0x26, 0x63, 0xd8, 0x70, 0x4d, 0xbc, 0xc4, 0x17, 0x70,
	0x40, 0x3f, 0x14, 0x15, 0x96, 0x78, 0x9a, 0x55, 0xb2, 0xa9, 0x2d, 0x1b, 0x4d, 0x83, 0x59, 0x98,
	0x8c, 0x37, 0xf6, 0x3b, 0xe7, 0xe2, 0x2b, 0x38, 0xb4, 0xa2, 0x22, 0xd9, 0xd8, 0x34, 0x2f, 0x65,
	0x71, 0x99, 0x5e, 0x90, 0x58, 0x5e, 0x58, 0x16, 0x4d, 0x83, 0xd9, 0x7e, 0x82, 0x9e, 0x9d, 0xb5,
	0xe8, 0x83, 0x23, 0xf1, 0x4b, 0x78, 0xf8, 0x5e, 0x53, 0x66, 0xa9, 0xbd, 0xd8, 0xe6, 0xa8, 0x47,
	0x30, 0xf0, 0x6d, 0x02, 0xd7, 0xc6, 0xab, 0xf8, 0x17, 0x60, 0xb7, 0xf8, 0x8e, 0xd3, 0x76, 0xf6,
	0xe8, 0xfd, 0x77, 0x8f, 0x70, 0xe7, 0x1e, 0x87, 0xd0, 0xcf, 0x85, 0x3a, 0x7d, 0xed, 0x4f, 0xbc,
	0x16, 0xa7, 0x7f, 0x02, 0xe8, 0x9f, 0xb5, 0x79, 0xc4, 0xb7, 0x30, 0xf4, 0xd1, 0x43, 0x36, 0xdf,
	0x44, 0x74, 0xbe, 0x9d, 0xcf, 0xc9, 0xe3, 0x1d, 0xc4, 0x8f, 0xfc, 0x09, 0xee, 0x77, 0x42, 0x82,
	0xc7, 0x5b, 0x95, 0xb7, 0x22, 0x36, 0x39, 0xb9, 0x83, 0xfa, 0xb7, 0x16, 0x00, 0x37, 0x47, 0xc1,
	0x27, 0x37, 0xc5, 0xff, 0xdc, 0x75, 0x72, 0xbc, 0x1b, 0xae, 0x1f, 0xca, 0x07, 0xee, 0x97, 0x7b,
	0xf3, 0x37, 0x00, 0x00, 0xff, 0xff, 0x1e, 0xc1, 0xa6, 0x93, 0x7f, 0x03, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// BoltzClient is the client API for Boltz service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type BoltzClient interface {
	GetInfo(ctx context.Context, in *GetInfoRequest, opts ...grpc.CallOption) (*GetInfoResponse, error)
	GetSwapInfo(ctx context.Context, in *GetSwapInfoRequest, opts ...grpc.CallOption) (*GetSwapInfoResponse, error)
	CreateSwap(ctx context.Context, in *CreateSwapRequest, opts ...grpc.CallOption) (*CreateSwapResponse, error)
}

type boltzClient struct {
	cc *grpc.ClientConn
}

func NewBoltzClient(cc *grpc.ClientConn) BoltzClient {
	return &boltzClient{cc}
}

func (c *boltzClient) GetInfo(ctx context.Context, in *GetInfoRequest, opts ...grpc.CallOption) (*GetInfoResponse, error) {
	out := new(GetInfoResponse)
	err := c.cc.Invoke(ctx, "/boltzrpc.Boltz/GetInfo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *boltzClient) GetSwapInfo(ctx context.Context, in *GetSwapInfoRequest, opts ...grpc.CallOption) (*GetSwapInfoResponse, error) {
	out := new(GetSwapInfoResponse)
	err := c.cc.Invoke(ctx, "/boltzrpc.Boltz/GetSwapInfo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *boltzClient) CreateSwap(ctx context.Context, in *CreateSwapRequest, opts ...grpc.CallOption) (*CreateSwapResponse, error) {
	out := new(CreateSwapResponse)
	err := c.cc.Invoke(ctx, "/boltzrpc.Boltz/CreateSwap", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BoltzServer is the server API for Boltz service.
type BoltzServer interface {
	GetInfo(context.Context, *GetInfoRequest) (*GetInfoResponse, error)
	GetSwapInfo(context.Context, *GetSwapInfoRequest) (*GetSwapInfoResponse, error)
	CreateSwap(context.Context, *CreateSwapRequest) (*CreateSwapResponse, error)
}

func RegisterBoltzServer(s *grpc.Server, srv BoltzServer) {
	s.RegisterService(&_Boltz_serviceDesc, srv)
}

func _Boltz_GetInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BoltzServer).GetInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/boltzrpc.Boltz/GetInfo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BoltzServer).GetInfo(ctx, req.(*GetInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Boltz_GetSwapInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSwapInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BoltzServer).GetSwapInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/boltzrpc.Boltz/GetSwapInfo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BoltzServer).GetSwapInfo(ctx, req.(*GetSwapInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Boltz_CreateSwap_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSwapRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BoltzServer).CreateSwap(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/boltzrpc.Boltz/CreateSwap",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BoltzServer).CreateSwap(ctx, req.(*CreateSwapRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Boltz_serviceDesc = grpc.ServiceDesc{
	ServiceName: "boltzrpc.Boltz",
	HandlerType: (*BoltzServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetInfo",
			Handler:    _Boltz_GetInfo_Handler,
		},
		{
			MethodName: "GetSwapInfo",
			Handler:    _Boltz_GetSwapInfo_Handler,
		},
		{
			MethodName: "CreateSwap",
			Handler:    _Boltz_CreateSwap_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "rpc.proto",
}
