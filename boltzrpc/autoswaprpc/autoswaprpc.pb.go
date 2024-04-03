// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v5.26.0
// source: autoswaprpc/autoswaprpc.proto

package autoswaprpc

import (
	boltzrpc "github.com/BoltzExchange/boltz-client/boltzrpc"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GetSwapRecommendationsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Do not return any dismissed recommendations
	NoDismissed *bool `protobuf:"varint,1,opt,name=no_dismissed,json=noDismissed,proto3,oneof" json:"no_dismissed,omitempty"`
}

func (x *GetSwapRecommendationsRequest) Reset() {
	*x = GetSwapRecommendationsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetSwapRecommendationsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetSwapRecommendationsRequest) ProtoMessage() {}

func (x *GetSwapRecommendationsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetSwapRecommendationsRequest.ProtoReflect.Descriptor instead.
func (*GetSwapRecommendationsRequest) Descriptor() ([]byte, []int) {
	return file_autoswaprpc_autoswaprpc_proto_rawDescGZIP(), []int{0}
}

func (x *GetSwapRecommendationsRequest) GetNoDismissed() bool {
	if x != nil && x.NoDismissed != nil {
		return *x.NoDismissed
	}
	return false
}

type SwapRecommendation struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type             string                     `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty"`
	Amount           uint64                     `protobuf:"varint,2,opt,name=amount,proto3" json:"amount,omitempty"`
	Channel          *boltzrpc.LightningChannel `protobuf:"bytes,3,opt,name=channel,proto3" json:"channel,omitempty"`
	FeeEstimate      uint64                     `protobuf:"varint,4,opt,name=fee_estimate,json=feeEstimate,proto3" json:"fee_estimate,omitempty"`
	DismissedReasons []string                   `protobuf:"bytes,5,rep,name=dismissed_reasons,json=dismissedReasons,proto3" json:"dismissed_reasons,omitempty"`
}

func (x *SwapRecommendation) Reset() {
	*x = SwapRecommendation{}
	if protoimpl.UnsafeEnabled {
		mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SwapRecommendation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SwapRecommendation) ProtoMessage() {}

func (x *SwapRecommendation) ProtoReflect() protoreflect.Message {
	mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SwapRecommendation.ProtoReflect.Descriptor instead.
func (*SwapRecommendation) Descriptor() ([]byte, []int) {
	return file_autoswaprpc_autoswaprpc_proto_rawDescGZIP(), []int{1}
}

func (x *SwapRecommendation) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *SwapRecommendation) GetAmount() uint64 {
	if x != nil {
		return x.Amount
	}
	return 0
}

func (x *SwapRecommendation) GetChannel() *boltzrpc.LightningChannel {
	if x != nil {
		return x.Channel
	}
	return nil
}

func (x *SwapRecommendation) GetFeeEstimate() uint64 {
	if x != nil {
		return x.FeeEstimate
	}
	return 0
}

func (x *SwapRecommendation) GetDismissedReasons() []string {
	if x != nil {
		return x.DismissedReasons
	}
	return nil
}

type Budget struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Total     uint64 `protobuf:"varint,1,opt,name=total,proto3" json:"total,omitempty"`
	Remaining int64  `protobuf:"varint,2,opt,name=remaining,proto3" json:"remaining,omitempty"`
	StartDate int64  `protobuf:"varint,3,opt,name=start_date,json=startDate,proto3" json:"start_date,omitempty"`
	EndDate   int64  `protobuf:"varint,4,opt,name=end_date,json=endDate,proto3" json:"end_date,omitempty"`
}

func (x *Budget) Reset() {
	*x = Budget{}
	if protoimpl.UnsafeEnabled {
		mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Budget) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Budget) ProtoMessage() {}

func (x *Budget) ProtoReflect() protoreflect.Message {
	mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Budget.ProtoReflect.Descriptor instead.
func (*Budget) Descriptor() ([]byte, []int) {
	return file_autoswaprpc_autoswaprpc_proto_rawDescGZIP(), []int{2}
}

func (x *Budget) GetTotal() uint64 {
	if x != nil {
		return x.Total
	}
	return 0
}

func (x *Budget) GetRemaining() int64 {
	if x != nil {
		return x.Remaining
	}
	return 0
}

func (x *Budget) GetStartDate() int64 {
	if x != nil {
		return x.StartDate
	}
	return 0
}

func (x *Budget) GetEndDate() int64 {
	if x != nil {
		return x.EndDate
	}
	return 0
}

type GetSwapRecommendationsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Swaps []*SwapRecommendation `protobuf:"bytes,1,rep,name=swaps,proto3" json:"swaps,omitempty"`
}

func (x *GetSwapRecommendationsResponse) Reset() {
	*x = GetSwapRecommendationsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetSwapRecommendationsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetSwapRecommendationsResponse) ProtoMessage() {}

func (x *GetSwapRecommendationsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetSwapRecommendationsResponse.ProtoReflect.Descriptor instead.
func (*GetSwapRecommendationsResponse) Descriptor() ([]byte, []int) {
	return file_autoswaprpc_autoswaprpc_proto_rawDescGZIP(), []int{3}
}

func (x *GetSwapRecommendationsResponse) GetSwaps() []*SwapRecommendation {
	if x != nil {
		return x.Swaps
	}
	return nil
}

type GetStatusRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetStatusRequest) Reset() {
	*x = GetStatusRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetStatusRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetStatusRequest) ProtoMessage() {}

func (x *GetStatusRequest) ProtoReflect() protoreflect.Message {
	mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetStatusRequest.ProtoReflect.Descriptor instead.
func (*GetStatusRequest) Descriptor() ([]byte, []int) {
	return file_autoswaprpc_autoswaprpc_proto_rawDescGZIP(), []int{4}
}

type GetStatusResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Running  bool                `protobuf:"varint,1,opt,name=running,proto3" json:"running,omitempty"`
	Strategy string              `protobuf:"bytes,2,opt,name=strategy,proto3" json:"strategy,omitempty"`
	Error    string              `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
	Stats    *boltzrpc.SwapStats `protobuf:"bytes,4,opt,name=stats,proto3,oneof" json:"stats,omitempty"`
	Budget   *Budget             `protobuf:"bytes,5,opt,name=budget,proto3,oneof" json:"budget,omitempty"`
}

func (x *GetStatusResponse) Reset() {
	*x = GetStatusResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetStatusResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetStatusResponse) ProtoMessage() {}

func (x *GetStatusResponse) ProtoReflect() protoreflect.Message {
	mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetStatusResponse.ProtoReflect.Descriptor instead.
func (*GetStatusResponse) Descriptor() ([]byte, []int) {
	return file_autoswaprpc_autoswaprpc_proto_rawDescGZIP(), []int{5}
}

func (x *GetStatusResponse) GetRunning() bool {
	if x != nil {
		return x.Running
	}
	return false
}

func (x *GetStatusResponse) GetStrategy() string {
	if x != nil {
		return x.Strategy
	}
	return ""
}

func (x *GetStatusResponse) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

func (x *GetStatusResponse) GetStats() *boltzrpc.SwapStats {
	if x != nil {
		return x.Stats
	}
	return nil
}

func (x *GetStatusResponse) GetBudget() *Budget {
	if x != nil {
		return x.Budget
	}
	return nil
}

type SetConfigValueRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key   string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *SetConfigValueRequest) Reset() {
	*x = SetConfigValueRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetConfigValueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetConfigValueRequest) ProtoMessage() {}

func (x *SetConfigValueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetConfigValueRequest.ProtoReflect.Descriptor instead.
func (*SetConfigValueRequest) Descriptor() ([]byte, []int) {
	return file_autoswaprpc_autoswaprpc_proto_rawDescGZIP(), []int{6}
}

func (x *SetConfigValueRequest) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *SetConfigValueRequest) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

type GetConfigRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key *string `protobuf:"bytes,1,opt,name=key,proto3,oneof" json:"key,omitempty"`
}

func (x *GetConfigRequest) Reset() {
	*x = GetConfigRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetConfigRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetConfigRequest) ProtoMessage() {}

func (x *GetConfigRequest) ProtoReflect() protoreflect.Message {
	mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetConfigRequest.ProtoReflect.Descriptor instead.
func (*GetConfigRequest) Descriptor() ([]byte, []int) {
	return file_autoswaprpc_autoswaprpc_proto_rawDescGZIP(), []int{7}
}

func (x *GetConfigRequest) GetKey() string {
	if x != nil && x.Key != nil {
		return *x.Key
	}
	return ""
}

type Config struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Enabled             bool              `protobuf:"varint,1,opt,name=enabled,proto3" json:"enabled,omitempty"`
	ChannelPollInterval uint64            `protobuf:"varint,2,opt,name=channel_poll_interval,json=channelPollInterval,proto3" json:"channel_poll_interval,omitempty"`
	StaticAddress       string            `protobuf:"bytes,3,opt,name=static_address,json=staticAddress,proto3" json:"static_address,omitempty"`
	MaxBalance          uint64            `protobuf:"varint,4,opt,name=max_balance,json=maxBalance,proto3" json:"max_balance,omitempty"`
	MinBalance          uint64            `protobuf:"varint,5,opt,name=min_balance,json=minBalance,proto3" json:"min_balance,omitempty"`
	MaxBalancePercent   float32           `protobuf:"fixed32,6,opt,name=max_balance_percent,json=maxBalancePercent,proto3" json:"max_balance_percent,omitempty"`
	MinBalancePercent   float32           `protobuf:"fixed32,7,opt,name=min_balance_percent,json=minBalancePercent,proto3" json:"min_balance_percent,omitempty"`
	MaxFeePercent       float32           `protobuf:"fixed32,8,opt,name=max_fee_percent,json=maxFeePercent,proto3" json:"max_fee_percent,omitempty"`
	AcceptZeroConf      bool              `protobuf:"varint,9,opt,name=accept_zero_conf,json=acceptZeroConf,proto3" json:"accept_zero_conf,omitempty"`
	FailureBackoff      uint64            `protobuf:"varint,10,opt,name=failure_backoff,json=failureBackoff,proto3" json:"failure_backoff,omitempty"`
	Budget              uint64            `protobuf:"varint,11,opt,name=budget,proto3" json:"budget,omitempty"`
	BudgetInterval      uint64            `protobuf:"varint,12,opt,name=budget_interval,json=budgetInterval,proto3" json:"budget_interval,omitempty"`
	Currency            boltzrpc.Currency `protobuf:"varint,13,opt,name=currency,proto3,enum=boltzrpc.Currency" json:"currency,omitempty"`
	SwapType            string            `protobuf:"bytes,14,opt,name=swap_type,json=swapType,proto3" json:"swap_type,omitempty"`
	PerChannel          bool              `protobuf:"varint,15,opt,name=per_channel,json=perChannel,proto3" json:"per_channel,omitempty"`
	Wallet              string            `protobuf:"bytes,16,opt,name=wallet,proto3" json:"wallet,omitempty"`
	MaxSwapAmount       uint64            `protobuf:"varint,17,opt,name=max_swap_amount,json=maxSwapAmount,proto3" json:"max_swap_amount,omitempty"`
}

func (x *Config) Reset() {
	*x = Config{}
	if protoimpl.UnsafeEnabled {
		mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Config) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Config) ProtoMessage() {}

func (x *Config) ProtoReflect() protoreflect.Message {
	mi := &file_autoswaprpc_autoswaprpc_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Config.ProtoReflect.Descriptor instead.
func (*Config) Descriptor() ([]byte, []int) {
	return file_autoswaprpc_autoswaprpc_proto_rawDescGZIP(), []int{8}
}

func (x *Config) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

func (x *Config) GetChannelPollInterval() uint64 {
	if x != nil {
		return x.ChannelPollInterval
	}
	return 0
}

func (x *Config) GetStaticAddress() string {
	if x != nil {
		return x.StaticAddress
	}
	return ""
}

func (x *Config) GetMaxBalance() uint64 {
	if x != nil {
		return x.MaxBalance
	}
	return 0
}

func (x *Config) GetMinBalance() uint64 {
	if x != nil {
		return x.MinBalance
	}
	return 0
}

func (x *Config) GetMaxBalancePercent() float32 {
	if x != nil {
		return x.MaxBalancePercent
	}
	return 0
}

func (x *Config) GetMinBalancePercent() float32 {
	if x != nil {
		return x.MinBalancePercent
	}
	return 0
}

func (x *Config) GetMaxFeePercent() float32 {
	if x != nil {
		return x.MaxFeePercent
	}
	return 0
}

func (x *Config) GetAcceptZeroConf() bool {
	if x != nil {
		return x.AcceptZeroConf
	}
	return false
}

func (x *Config) GetFailureBackoff() uint64 {
	if x != nil {
		return x.FailureBackoff
	}
	return 0
}

func (x *Config) GetBudget() uint64 {
	if x != nil {
		return x.Budget
	}
	return 0
}

func (x *Config) GetBudgetInterval() uint64 {
	if x != nil {
		return x.BudgetInterval
	}
	return 0
}

func (x *Config) GetCurrency() boltzrpc.Currency {
	if x != nil {
		return x.Currency
	}
	return boltzrpc.Currency(0)
}

func (x *Config) GetSwapType() string {
	if x != nil {
		return x.SwapType
	}
	return ""
}

func (x *Config) GetPerChannel() bool {
	if x != nil {
		return x.PerChannel
	}
	return false
}

func (x *Config) GetWallet() string {
	if x != nil {
		return x.Wallet
	}
	return ""
}

func (x *Config) GetMaxSwapAmount() uint64 {
	if x != nil {
		return x.MaxSwapAmount
	}
	return 0
}

var File_autoswaprpc_autoswaprpc_proto protoreflect.FileDescriptor

var file_autoswaprpc_autoswaprpc_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2f, 0x61, 0x75,
	0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x0b, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x1a, 0x1b, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d,
	0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x0e, 0x62, 0x6f, 0x6c, 0x74, 0x7a,
	0x72, 0x70, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x58, 0x0a, 0x1d, 0x47, 0x65, 0x74,
	0x53, 0x77, 0x61, 0x70, 0x52, 0x65, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x64, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x26, 0x0a, 0x0c, 0x6e, 0x6f,
	0x5f, 0x64, 0x69, 0x73, 0x6d, 0x69, 0x73, 0x73, 0x65, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08,
	0x48, 0x00, 0x52, 0x0b, 0x6e, 0x6f, 0x44, 0x69, 0x73, 0x6d, 0x69, 0x73, 0x73, 0x65, 0x64, 0x88,
	0x01, 0x01, 0x42, 0x0f, 0x0a, 0x0d, 0x5f, 0x6e, 0x6f, 0x5f, 0x64, 0x69, 0x73, 0x6d, 0x69, 0x73,
	0x73, 0x65, 0x64, 0x22, 0xc6, 0x01, 0x0a, 0x12, 0x53, 0x77, 0x61, 0x70, 0x52, 0x65, 0x63, 0x6f,
	0x6d, 0x6d, 0x65, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79,
	0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06,
	0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x34, 0x0a, 0x07, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65,
	0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x62, 0x6f, 0x6c, 0x74, 0x7a, 0x72,
	0x70, 0x63, 0x2e, 0x4c, 0x69, 0x67, 0x68, 0x74, 0x6e, 0x69, 0x6e, 0x67, 0x43, 0x68, 0x61, 0x6e,
	0x6e, 0x65, 0x6c, 0x52, 0x07, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x12, 0x21, 0x0a, 0x0c,
	0x66, 0x65, 0x65, 0x5f, 0x65, 0x73, 0x74, 0x69, 0x6d, 0x61, 0x74, 0x65, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x0b, 0x66, 0x65, 0x65, 0x45, 0x73, 0x74, 0x69, 0x6d, 0x61, 0x74, 0x65, 0x12,
	0x2b, 0x0a, 0x11, 0x64, 0x69, 0x73, 0x6d, 0x69, 0x73, 0x73, 0x65, 0x64, 0x5f, 0x72, 0x65, 0x61,
	0x73, 0x6f, 0x6e, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x09, 0x52, 0x10, 0x64, 0x69, 0x73, 0x6d,
	0x69, 0x73, 0x73, 0x65, 0x64, 0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x73, 0x22, 0x76, 0x0a, 0x06,
	0x42, 0x75, 0x64, 0x67, 0x65, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x12, 0x1c, 0x0a, 0x09,
	0x72, 0x65, 0x6d, 0x61, 0x69, 0x6e, 0x69, 0x6e, 0x67, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52,
	0x09, 0x72, 0x65, 0x6d, 0x61, 0x69, 0x6e, 0x69, 0x6e, 0x67, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x74,
	0x61, 0x72, 0x74, 0x5f, 0x64, 0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09,
	0x73, 0x74, 0x61, 0x72, 0x74, 0x44, 0x61, 0x74, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x65, 0x6e, 0x64,
	0x5f, 0x64, 0x61, 0x74, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x65, 0x6e, 0x64,
	0x44, 0x61, 0x74, 0x65, 0x22, 0x57, 0x0a, 0x1e, 0x47, 0x65, 0x74, 0x53, 0x77, 0x61, 0x70, 0x52,
	0x65, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x35, 0x0a, 0x05, 0x73, 0x77, 0x61, 0x70, 0x73, 0x18,
	0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70,
	0x72, 0x70, 0x63, 0x2e, 0x53, 0x77, 0x61, 0x70, 0x52, 0x65, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e,
	0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x05, 0x73, 0x77, 0x61, 0x70, 0x73, 0x22, 0x12, 0x0a,
	0x10, 0x47, 0x65, 0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x22, 0xd6, 0x01, 0x0a, 0x11, 0x47, 0x65, 0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x72, 0x75, 0x6e, 0x6e, 0x69,
	0x6e, 0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x72, 0x75, 0x6e, 0x6e, 0x69, 0x6e,
	0x67, 0x12, 0x1a, 0x0a, 0x08, 0x73, 0x74, 0x72, 0x61, 0x74, 0x65, 0x67, 0x79, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x73, 0x74, 0x72, 0x61, 0x74, 0x65, 0x67, 0x79, 0x12, 0x14, 0x0a,
	0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x72,
	0x72, 0x6f, 0x72, 0x12, 0x2e, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x73, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x13, 0x2e, 0x62, 0x6f, 0x6c, 0x74, 0x7a, 0x72, 0x70, 0x63, 0x2e, 0x53, 0x77,
	0x61, 0x70, 0x53, 0x74, 0x61, 0x74, 0x73, 0x48, 0x00, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74, 0x73,
	0x88, 0x01, 0x01, 0x12, 0x30, 0x0a, 0x06, 0x62, 0x75, 0x64, 0x67, 0x65, 0x74, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70,
	0x63, 0x2e, 0x42, 0x75, 0x64, 0x67, 0x65, 0x74, 0x48, 0x01, 0x52, 0x06, 0x62, 0x75, 0x64, 0x67,
	0x65, 0x74, 0x88, 0x01, 0x01, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x73, 0x42,
	0x09, 0x0a, 0x07, 0x5f, 0x62, 0x75, 0x64, 0x67, 0x65, 0x74, 0x22, 0x3f, 0x0a, 0x15, 0x53, 0x65,
	0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x31, 0x0a, 0x10, 0x47,
	0x65, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x15, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x03,
	0x6b, 0x65, 0x79, 0x88, 0x01, 0x01, 0x42, 0x06, 0x0a, 0x04, 0x5f, 0x6b, 0x65, 0x79, 0x22, 0x89,
	0x05, 0x0a, 0x06, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x18, 0x0a, 0x07, 0x65, 0x6e, 0x61,
	0x62, 0x6c, 0x65, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x65, 0x6e, 0x61, 0x62,
	0x6c, 0x65, 0x64, 0x12, 0x32, 0x0a, 0x15, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x5f, 0x70,
	0x6f, 0x6c, 0x6c, 0x5f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x13, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x50, 0x6f, 0x6c, 0x6c, 0x49,
	0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x12, 0x25, 0x0a, 0x0e, 0x73, 0x74, 0x61, 0x74, 0x69,
	0x63, 0x5f, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0d, 0x73, 0x74, 0x61, 0x74, 0x69, 0x63, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x1f,
	0x0a, 0x0b, 0x6d, 0x61, 0x78, 0x5f, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x04, 0x52, 0x0a, 0x6d, 0x61, 0x78, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x12,
	0x1f, 0x0a, 0x0b, 0x6d, 0x69, 0x6e, 0x5f, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x0a, 0x6d, 0x69, 0x6e, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65,
	0x12, 0x2e, 0x0a, 0x13, 0x6d, 0x61, 0x78, 0x5f, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x5f,
	0x70, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x02, 0x52, 0x11, 0x6d,
	0x61, 0x78, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x50, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x12, 0x2e, 0x0a, 0x13, 0x6d, 0x69, 0x6e, 0x5f, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x5f,
	0x70, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x02, 0x52, 0x11, 0x6d,
	0x69, 0x6e, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x50, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x12, 0x26, 0x0a, 0x0f, 0x6d, 0x61, 0x78, 0x5f, 0x66, 0x65, 0x65, 0x5f, 0x70, 0x65, 0x72, 0x63,
	0x65, 0x6e, 0x74, 0x18, 0x08, 0x20, 0x01, 0x28, 0x02, 0x52, 0x0d, 0x6d, 0x61, 0x78, 0x46, 0x65,
	0x65, 0x50, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x12, 0x28, 0x0a, 0x10, 0x61, 0x63, 0x63, 0x65,
	0x70, 0x74, 0x5f, 0x7a, 0x65, 0x72, 0x6f, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x18, 0x09, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x0e, 0x61, 0x63, 0x63, 0x65, 0x70, 0x74, 0x5a, 0x65, 0x72, 0x6f, 0x43, 0x6f,
	0x6e, 0x66, 0x12, 0x27, 0x0a, 0x0f, 0x66, 0x61, 0x69, 0x6c, 0x75, 0x72, 0x65, 0x5f, 0x62, 0x61,
	0x63, 0x6b, 0x6f, 0x66, 0x66, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0e, 0x66, 0x61, 0x69,
	0x6c, 0x75, 0x72, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x6f, 0x66, 0x66, 0x12, 0x16, 0x0a, 0x06, 0x62,
	0x75, 0x64, 0x67, 0x65, 0x74, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06, 0x62, 0x75, 0x64,
	0x67, 0x65, 0x74, 0x12, 0x27, 0x0a, 0x0f, 0x62, 0x75, 0x64, 0x67, 0x65, 0x74, 0x5f, 0x69, 0x6e,
	0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0e, 0x62, 0x75,
	0x64, 0x67, 0x65, 0x74, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x12, 0x2e, 0x0a, 0x08,
	0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x63, 0x79, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x12,
	0x2e, 0x62, 0x6f, 0x6c, 0x74, 0x7a, 0x72, 0x70, 0x63, 0x2e, 0x43, 0x75, 0x72, 0x72, 0x65, 0x6e,
	0x63, 0x79, 0x52, 0x08, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x63, 0x79, 0x12, 0x1b, 0x0a, 0x09,
	0x73, 0x77, 0x61, 0x70, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x73, 0x77, 0x61, 0x70, 0x54, 0x79, 0x70, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x70, 0x65, 0x72,
	0x5f, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0a,
	0x70, 0x65, 0x72, 0x43, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x12, 0x16, 0x0a, 0x06, 0x77, 0x61,
	0x6c, 0x6c, 0x65, 0x74, 0x18, 0x10, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x77, 0x61, 0x6c, 0x6c,
	0x65, 0x74, 0x12, 0x26, 0x0a, 0x0f, 0x6d, 0x61, 0x78, 0x5f, 0x73, 0x77, 0x61, 0x70, 0x5f, 0x61,
	0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x11, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0d, 0x6d, 0x61, 0x78,
	0x53, 0x77, 0x61, 0x70, 0x41, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x32, 0x85, 0x04, 0x0a, 0x08, 0x41,
	0x75, 0x74, 0x6f, 0x53, 0x77, 0x61, 0x70, 0x12, 0x71, 0x0a, 0x16, 0x47, 0x65, 0x74, 0x53, 0x77,
	0x61, 0x70, 0x52, 0x65, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x12, 0x2a, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e,
	0x47, 0x65, 0x74, 0x53, 0x77, 0x61, 0x70, 0x52, 0x65, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x64,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2b, 0x2e,
	0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e, 0x47, 0x65, 0x74, 0x53,
	0x77, 0x61, 0x70, 0x52, 0x65, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4a, 0x0a, 0x09, 0x47, 0x65,
	0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x1d, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77,
	0x61, 0x70, 0x72, 0x70, 0x63, 0x2e, 0x47, 0x65, 0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61,
	0x70, 0x72, 0x70, 0x63, 0x2e, 0x47, 0x65, 0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x3a, 0x0a, 0x0b, 0x52, 0x65, 0x73, 0x65, 0x74, 0x43,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x13, 0x2e,
	0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e, 0x43, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x12, 0x35, 0x0a, 0x09, 0x53, 0x65, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12,
	0x13, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x1a, 0x13, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72,
	0x70, 0x63, 0x2e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x49, 0x0a, 0x0e, 0x53, 0x65, 0x74,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x22, 0x2e, 0x61, 0x75,
	0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e, 0x53, 0x65, 0x74, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x13, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x12, 0x3f, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x12, 0x1d, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e,
	0x47, 0x65, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x13, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e, 0x43,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x3b, 0x0a, 0x0c, 0x52, 0x65, 0x6c, 0x6f, 0x61, 0x64, 0x43,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x13, 0x2e,
	0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63, 0x2e, 0x43, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x42, 0x3c, 0x5a, 0x3a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x42, 0x6f, 0x6c, 0x74, 0x7a, 0x45, 0x78, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x2f, 0x62,
	0x6f, 0x6c, 0x74, 0x7a, 0x2d, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x2f, 0x62, 0x6f, 0x6c, 0x74,
	0x7a, 0x72, 0x70, 0x63, 0x2f, 0x61, 0x75, 0x74, 0x6f, 0x73, 0x77, 0x61, 0x70, 0x72, 0x70, 0x63,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_autoswaprpc_autoswaprpc_proto_rawDescOnce sync.Once
	file_autoswaprpc_autoswaprpc_proto_rawDescData = file_autoswaprpc_autoswaprpc_proto_rawDesc
)

func file_autoswaprpc_autoswaprpc_proto_rawDescGZIP() []byte {
	file_autoswaprpc_autoswaprpc_proto_rawDescOnce.Do(func() {
		file_autoswaprpc_autoswaprpc_proto_rawDescData = protoimpl.X.CompressGZIP(file_autoswaprpc_autoswaprpc_proto_rawDescData)
	})
	return file_autoswaprpc_autoswaprpc_proto_rawDescData
}

var file_autoswaprpc_autoswaprpc_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_autoswaprpc_autoswaprpc_proto_goTypes = []interface{}{
	(*GetSwapRecommendationsRequest)(nil),  // 0: autoswaprpc.GetSwapRecommendationsRequest
	(*SwapRecommendation)(nil),             // 1: autoswaprpc.SwapRecommendation
	(*Budget)(nil),                         // 2: autoswaprpc.Budget
	(*GetSwapRecommendationsResponse)(nil), // 3: autoswaprpc.GetSwapRecommendationsResponse
	(*GetStatusRequest)(nil),               // 4: autoswaprpc.GetStatusRequest
	(*GetStatusResponse)(nil),              // 5: autoswaprpc.GetStatusResponse
	(*SetConfigValueRequest)(nil),          // 6: autoswaprpc.SetConfigValueRequest
	(*GetConfigRequest)(nil),               // 7: autoswaprpc.GetConfigRequest
	(*Config)(nil),                         // 8: autoswaprpc.Config
	(*boltzrpc.LightningChannel)(nil),      // 9: boltzrpc.LightningChannel
	(*boltzrpc.SwapStats)(nil),             // 10: boltzrpc.SwapStats
	(boltzrpc.Currency)(0),                 // 11: boltzrpc.Currency
	(*emptypb.Empty)(nil),                  // 12: google.protobuf.Empty
}
var file_autoswaprpc_autoswaprpc_proto_depIdxs = []int32{
	9,  // 0: autoswaprpc.SwapRecommendation.channel:type_name -> boltzrpc.LightningChannel
	1,  // 1: autoswaprpc.GetSwapRecommendationsResponse.swaps:type_name -> autoswaprpc.SwapRecommendation
	10, // 2: autoswaprpc.GetStatusResponse.stats:type_name -> boltzrpc.SwapStats
	2,  // 3: autoswaprpc.GetStatusResponse.budget:type_name -> autoswaprpc.Budget
	11, // 4: autoswaprpc.Config.currency:type_name -> boltzrpc.Currency
	0,  // 5: autoswaprpc.AutoSwap.GetSwapRecommendations:input_type -> autoswaprpc.GetSwapRecommendationsRequest
	4,  // 6: autoswaprpc.AutoSwap.GetStatus:input_type -> autoswaprpc.GetStatusRequest
	12, // 7: autoswaprpc.AutoSwap.ResetConfig:input_type -> google.protobuf.Empty
	8,  // 8: autoswaprpc.AutoSwap.SetConfig:input_type -> autoswaprpc.Config
	6,  // 9: autoswaprpc.AutoSwap.SetConfigValue:input_type -> autoswaprpc.SetConfigValueRequest
	7,  // 10: autoswaprpc.AutoSwap.GetConfig:input_type -> autoswaprpc.GetConfigRequest
	12, // 11: autoswaprpc.AutoSwap.ReloadConfig:input_type -> google.protobuf.Empty
	3,  // 12: autoswaprpc.AutoSwap.GetSwapRecommendations:output_type -> autoswaprpc.GetSwapRecommendationsResponse
	5,  // 13: autoswaprpc.AutoSwap.GetStatus:output_type -> autoswaprpc.GetStatusResponse
	8,  // 14: autoswaprpc.AutoSwap.ResetConfig:output_type -> autoswaprpc.Config
	8,  // 15: autoswaprpc.AutoSwap.SetConfig:output_type -> autoswaprpc.Config
	8,  // 16: autoswaprpc.AutoSwap.SetConfigValue:output_type -> autoswaprpc.Config
	8,  // 17: autoswaprpc.AutoSwap.GetConfig:output_type -> autoswaprpc.Config
	8,  // 18: autoswaprpc.AutoSwap.ReloadConfig:output_type -> autoswaprpc.Config
	12, // [12:19] is the sub-list for method output_type
	5,  // [5:12] is the sub-list for method input_type
	5,  // [5:5] is the sub-list for extension type_name
	5,  // [5:5] is the sub-list for extension extendee
	0,  // [0:5] is the sub-list for field type_name
}

func init() { file_autoswaprpc_autoswaprpc_proto_init() }
func file_autoswaprpc_autoswaprpc_proto_init() {
	if File_autoswaprpc_autoswaprpc_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_autoswaprpc_autoswaprpc_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetSwapRecommendationsRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_autoswaprpc_autoswaprpc_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SwapRecommendation); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_autoswaprpc_autoswaprpc_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Budget); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_autoswaprpc_autoswaprpc_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetSwapRecommendationsResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_autoswaprpc_autoswaprpc_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetStatusRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_autoswaprpc_autoswaprpc_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetStatusResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_autoswaprpc_autoswaprpc_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetConfigValueRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_autoswaprpc_autoswaprpc_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetConfigRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_autoswaprpc_autoswaprpc_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Config); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_autoswaprpc_autoswaprpc_proto_msgTypes[0].OneofWrappers = []interface{}{}
	file_autoswaprpc_autoswaprpc_proto_msgTypes[5].OneofWrappers = []interface{}{}
	file_autoswaprpc_autoswaprpc_proto_msgTypes[7].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_autoswaprpc_autoswaprpc_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_autoswaprpc_autoswaprpc_proto_goTypes,
		DependencyIndexes: file_autoswaprpc_autoswaprpc_proto_depIdxs,
		MessageInfos:      file_autoswaprpc_autoswaprpc_proto_msgTypes,
	}.Build()
	File_autoswaprpc_autoswaprpc_proto = out.File
	file_autoswaprpc_autoswaprpc_proto_rawDesc = nil
	file_autoswaprpc_autoswaprpc_proto_goTypes = nil
	file_autoswaprpc_autoswaprpc_proto_depIdxs = nil
}
