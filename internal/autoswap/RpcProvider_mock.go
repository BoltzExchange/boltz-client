// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package autoswap

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/database"
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	mock "github.com/stretchr/testify/mock"
)

// NewMockRpcProvider creates a new instance of MockRpcProvider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockRpcProvider(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRpcProvider {
	mock := &MockRpcProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// MockRpcProvider is an autogenerated mock type for the RpcProvider type
type MockRpcProvider struct {
	mock.Mock
}

type MockRpcProvider_Expecter struct {
	mock *mock.Mock
}

func (_m *MockRpcProvider) EXPECT() *MockRpcProvider_Expecter {
	return &MockRpcProvider_Expecter{mock: &_m.Mock}
}

// CreateAutoChainSwap provides a mock function for the type MockRpcProvider
func (_mock *MockRpcProvider) CreateAutoChainSwap(tenant *database.Tenant, request *boltzrpc.CreateChainSwapRequest) error {
	ret := _mock.Called(tenant, request)

	if len(ret) == 0 {
		panic("no return value specified for CreateAutoChainSwap")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(*database.Tenant, *boltzrpc.CreateChainSwapRequest) error); ok {
		r0 = returnFunc(tenant, request)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// MockRpcProvider_CreateAutoChainSwap_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateAutoChainSwap'
type MockRpcProvider_CreateAutoChainSwap_Call struct {
	*mock.Call
}

// CreateAutoChainSwap is a helper method to define mock.On call
//   - tenant *database.Tenant
//   - request *boltzrpc.CreateChainSwapRequest
func (_e *MockRpcProvider_Expecter) CreateAutoChainSwap(tenant interface{}, request interface{}) *MockRpcProvider_CreateAutoChainSwap_Call {
	return &MockRpcProvider_CreateAutoChainSwap_Call{Call: _e.mock.On("CreateAutoChainSwap", tenant, request)}
}

func (_c *MockRpcProvider_CreateAutoChainSwap_Call) Run(run func(tenant *database.Tenant, request *boltzrpc.CreateChainSwapRequest)) *MockRpcProvider_CreateAutoChainSwap_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 *database.Tenant
		if args[0] != nil {
			arg0 = args[0].(*database.Tenant)
		}
		var arg1 *boltzrpc.CreateChainSwapRequest
		if args[1] != nil {
			arg1 = args[1].(*boltzrpc.CreateChainSwapRequest)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *MockRpcProvider_CreateAutoChainSwap_Call) Return(err error) *MockRpcProvider_CreateAutoChainSwap_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockRpcProvider_CreateAutoChainSwap_Call) RunAndReturn(run func(tenant *database.Tenant, request *boltzrpc.CreateChainSwapRequest) error) *MockRpcProvider_CreateAutoChainSwap_Call {
	_c.Call.Return(run)
	return _c
}

// CreateAutoReverseSwap provides a mock function for the type MockRpcProvider
func (_mock *MockRpcProvider) CreateAutoReverseSwap(tenant *database.Tenant, request *boltzrpc.CreateReverseSwapRequest) error {
	ret := _mock.Called(tenant, request)

	if len(ret) == 0 {
		panic("no return value specified for CreateAutoReverseSwap")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(*database.Tenant, *boltzrpc.CreateReverseSwapRequest) error); ok {
		r0 = returnFunc(tenant, request)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// MockRpcProvider_CreateAutoReverseSwap_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateAutoReverseSwap'
type MockRpcProvider_CreateAutoReverseSwap_Call struct {
	*mock.Call
}

// CreateAutoReverseSwap is a helper method to define mock.On call
//   - tenant *database.Tenant
//   - request *boltzrpc.CreateReverseSwapRequest
func (_e *MockRpcProvider_Expecter) CreateAutoReverseSwap(tenant interface{}, request interface{}) *MockRpcProvider_CreateAutoReverseSwap_Call {
	return &MockRpcProvider_CreateAutoReverseSwap_Call{Call: _e.mock.On("CreateAutoReverseSwap", tenant, request)}
}

func (_c *MockRpcProvider_CreateAutoReverseSwap_Call) Run(run func(tenant *database.Tenant, request *boltzrpc.CreateReverseSwapRequest)) *MockRpcProvider_CreateAutoReverseSwap_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 *database.Tenant
		if args[0] != nil {
			arg0 = args[0].(*database.Tenant)
		}
		var arg1 *boltzrpc.CreateReverseSwapRequest
		if args[1] != nil {
			arg1 = args[1].(*boltzrpc.CreateReverseSwapRequest)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *MockRpcProvider_CreateAutoReverseSwap_Call) Return(err error) *MockRpcProvider_CreateAutoReverseSwap_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockRpcProvider_CreateAutoReverseSwap_Call) RunAndReturn(run func(tenant *database.Tenant, request *boltzrpc.CreateReverseSwapRequest) error) *MockRpcProvider_CreateAutoReverseSwap_Call {
	_c.Call.Return(run)
	return _c
}

// CreateAutoSwap provides a mock function for the type MockRpcProvider
func (_mock *MockRpcProvider) CreateAutoSwap(tenant *database.Tenant, request *boltzrpc.CreateSwapRequest) error {
	ret := _mock.Called(tenant, request)

	if len(ret) == 0 {
		panic("no return value specified for CreateAutoSwap")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(*database.Tenant, *boltzrpc.CreateSwapRequest) error); ok {
		r0 = returnFunc(tenant, request)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// MockRpcProvider_CreateAutoSwap_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateAutoSwap'
type MockRpcProvider_CreateAutoSwap_Call struct {
	*mock.Call
}

// CreateAutoSwap is a helper method to define mock.On call
//   - tenant *database.Tenant
//   - request *boltzrpc.CreateSwapRequest
func (_e *MockRpcProvider_Expecter) CreateAutoSwap(tenant interface{}, request interface{}) *MockRpcProvider_CreateAutoSwap_Call {
	return &MockRpcProvider_CreateAutoSwap_Call{Call: _e.mock.On("CreateAutoSwap", tenant, request)}
}

func (_c *MockRpcProvider_CreateAutoSwap_Call) Run(run func(tenant *database.Tenant, request *boltzrpc.CreateSwapRequest)) *MockRpcProvider_CreateAutoSwap_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 *database.Tenant
		if args[0] != nil {
			arg0 = args[0].(*database.Tenant)
		}
		var arg1 *boltzrpc.CreateSwapRequest
		if args[1] != nil {
			arg1 = args[1].(*boltzrpc.CreateSwapRequest)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *MockRpcProvider_CreateAutoSwap_Call) Return(err error) *MockRpcProvider_CreateAutoSwap_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockRpcProvider_CreateAutoSwap_Call) RunAndReturn(run func(tenant *database.Tenant, request *boltzrpc.CreateSwapRequest) error) *MockRpcProvider_CreateAutoSwap_Call {
	_c.Call.Return(run)
	return _c
}

// GetAutoSwapPairInfo provides a mock function for the type MockRpcProvider
func (_mock *MockRpcProvider) GetAutoSwapPairInfo(swapType boltzrpc.SwapType, pair *boltzrpc.Pair) (*boltzrpc.PairInfo, error) {
	ret := _mock.Called(swapType, pair)

	if len(ret) == 0 {
		panic("no return value specified for GetAutoSwapPairInfo")
	}

	var r0 *boltzrpc.PairInfo
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(boltzrpc.SwapType, *boltzrpc.Pair) (*boltzrpc.PairInfo, error)); ok {
		return returnFunc(swapType, pair)
	}
	if returnFunc, ok := ret.Get(0).(func(boltzrpc.SwapType, *boltzrpc.Pair) *boltzrpc.PairInfo); ok {
		r0 = returnFunc(swapType, pair)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*boltzrpc.PairInfo)
		}
	}
	if returnFunc, ok := ret.Get(1).(func(boltzrpc.SwapType, *boltzrpc.Pair) error); ok {
		r1 = returnFunc(swapType, pair)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockRpcProvider_GetAutoSwapPairInfo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetAutoSwapPairInfo'
type MockRpcProvider_GetAutoSwapPairInfo_Call struct {
	*mock.Call
}

// GetAutoSwapPairInfo is a helper method to define mock.On call
//   - swapType boltzrpc.SwapType
//   - pair *boltzrpc.Pair
func (_e *MockRpcProvider_Expecter) GetAutoSwapPairInfo(swapType interface{}, pair interface{}) *MockRpcProvider_GetAutoSwapPairInfo_Call {
	return &MockRpcProvider_GetAutoSwapPairInfo_Call{Call: _e.mock.On("GetAutoSwapPairInfo", swapType, pair)}
}

func (_c *MockRpcProvider_GetAutoSwapPairInfo_Call) Run(run func(swapType boltzrpc.SwapType, pair *boltzrpc.Pair)) *MockRpcProvider_GetAutoSwapPairInfo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 boltzrpc.SwapType
		if args[0] != nil {
			arg0 = args[0].(boltzrpc.SwapType)
		}
		var arg1 *boltzrpc.Pair
		if args[1] != nil {
			arg1 = args[1].(*boltzrpc.Pair)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *MockRpcProvider_GetAutoSwapPairInfo_Call) Return(pairInfo *boltzrpc.PairInfo, err error) *MockRpcProvider_GetAutoSwapPairInfo_Call {
	_c.Call.Return(pairInfo, err)
	return _c
}

func (_c *MockRpcProvider_GetAutoSwapPairInfo_Call) RunAndReturn(run func(swapType boltzrpc.SwapType, pair *boltzrpc.Pair) (*boltzrpc.PairInfo, error)) *MockRpcProvider_GetAutoSwapPairInfo_Call {
	_c.Call.Return(run)
	return _c
}

// GetBlockUpdates provides a mock function for the type MockRpcProvider
func (_mock *MockRpcProvider) GetBlockUpdates(currency boltz.Currency) (<-chan *onchain.BlockEpoch, func()) {
	ret := _mock.Called(currency)

	if len(ret) == 0 {
		panic("no return value specified for GetBlockUpdates")
	}

	var r0 <-chan *onchain.BlockEpoch
	var r1 func()
	if returnFunc, ok := ret.Get(0).(func(boltz.Currency) (<-chan *onchain.BlockEpoch, func())); ok {
		return returnFunc(currency)
	}
	if returnFunc, ok := ret.Get(0).(func(boltz.Currency) <-chan *onchain.BlockEpoch); ok {
		r0 = returnFunc(currency)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan *onchain.BlockEpoch)
		}
	}
	if returnFunc, ok := ret.Get(1).(func(boltz.Currency) func()); ok {
		r1 = returnFunc(currency)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(func())
		}
	}
	return r0, r1
}

// MockRpcProvider_GetBlockUpdates_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetBlockUpdates'
type MockRpcProvider_GetBlockUpdates_Call struct {
	*mock.Call
}

// GetBlockUpdates is a helper method to define mock.On call
//   - currency boltz.Currency
func (_e *MockRpcProvider_Expecter) GetBlockUpdates(currency interface{}) *MockRpcProvider_GetBlockUpdates_Call {
	return &MockRpcProvider_GetBlockUpdates_Call{Call: _e.mock.On("GetBlockUpdates", currency)}
}

func (_c *MockRpcProvider_GetBlockUpdates_Call) Run(run func(currency boltz.Currency)) *MockRpcProvider_GetBlockUpdates_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 boltz.Currency
		if args[0] != nil {
			arg0 = args[0].(boltz.Currency)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *MockRpcProvider_GetBlockUpdates_Call) Return(blockEpochCh <-chan *onchain.BlockEpoch, fn func()) *MockRpcProvider_GetBlockUpdates_Call {
	_c.Call.Return(blockEpochCh, fn)
	return _c
}

func (_c *MockRpcProvider_GetBlockUpdates_Call) RunAndReturn(run func(currency boltz.Currency) (<-chan *onchain.BlockEpoch, func())) *MockRpcProvider_GetBlockUpdates_Call {
	_c.Call.Return(run)
	return _c
}

// GetLightningChannels provides a mock function for the type MockRpcProvider
func (_mock *MockRpcProvider) GetLightningChannels() ([]*lightning.LightningChannel, error) {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetLightningChannels")
	}

	var r0 []*lightning.LightningChannel
	var r1 error
	if returnFunc, ok := ret.Get(0).(func() ([]*lightning.LightningChannel, error)); ok {
		return returnFunc()
	}
	if returnFunc, ok := ret.Get(0).(func() []*lightning.LightningChannel); ok {
		r0 = returnFunc()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*lightning.LightningChannel)
		}
	}
	if returnFunc, ok := ret.Get(1).(func() error); ok {
		r1 = returnFunc()
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockRpcProvider_GetLightningChannels_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetLightningChannels'
type MockRpcProvider_GetLightningChannels_Call struct {
	*mock.Call
}

// GetLightningChannels is a helper method to define mock.On call
func (_e *MockRpcProvider_Expecter) GetLightningChannels() *MockRpcProvider_GetLightningChannels_Call {
	return &MockRpcProvider_GetLightningChannels_Call{Call: _e.mock.On("GetLightningChannels")}
}

func (_c *MockRpcProvider_GetLightningChannels_Call) Run(run func()) *MockRpcProvider_GetLightningChannels_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockRpcProvider_GetLightningChannels_Call) Return(lightningChannels []*lightning.LightningChannel, err error) *MockRpcProvider_GetLightningChannels_Call {
	_c.Call.Return(lightningChannels, err)
	return _c
}

func (_c *MockRpcProvider_GetLightningChannels_Call) RunAndReturn(run func() ([]*lightning.LightningChannel, error)) *MockRpcProvider_GetLightningChannels_Call {
	_c.Call.Return(run)
	return _c
}

// WalletSendFee provides a mock function for the type MockRpcProvider
func (_mock *MockRpcProvider) WalletSendFee(request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error) {
	ret := _mock.Called(request)

	if len(ret) == 0 {
		panic("no return value specified for WalletSendFee")
	}

	var r0 *boltzrpc.WalletSendFee
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(*boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error)); ok {
		return returnFunc(request)
	}
	if returnFunc, ok := ret.Get(0).(func(*boltzrpc.WalletSendRequest) *boltzrpc.WalletSendFee); ok {
		r0 = returnFunc(request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*boltzrpc.WalletSendFee)
		}
	}
	if returnFunc, ok := ret.Get(1).(func(*boltzrpc.WalletSendRequest) error); ok {
		r1 = returnFunc(request)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockRpcProvider_WalletSendFee_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'WalletSendFee'
type MockRpcProvider_WalletSendFee_Call struct {
	*mock.Call
}

// WalletSendFee is a helper method to define mock.On call
//   - request *boltzrpc.WalletSendRequest
func (_e *MockRpcProvider_Expecter) WalletSendFee(request interface{}) *MockRpcProvider_WalletSendFee_Call {
	return &MockRpcProvider_WalletSendFee_Call{Call: _e.mock.On("WalletSendFee", request)}
}

func (_c *MockRpcProvider_WalletSendFee_Call) Run(run func(request *boltzrpc.WalletSendRequest)) *MockRpcProvider_WalletSendFee_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 *boltzrpc.WalletSendRequest
		if args[0] != nil {
			arg0 = args[0].(*boltzrpc.WalletSendRequest)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *MockRpcProvider_WalletSendFee_Call) Return(walletSendFee *boltzrpc.WalletSendFee, err error) *MockRpcProvider_WalletSendFee_Call {
	_c.Call.Return(walletSendFee, err)
	return _c
}

func (_c *MockRpcProvider_WalletSendFee_Call) RunAndReturn(run func(request *boltzrpc.WalletSendRequest) (*boltzrpc.WalletSendFee, error)) *MockRpcProvider_WalletSendFee_Call {
	_c.Call.Return(run)
	return _c
}
