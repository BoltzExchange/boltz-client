// Code generated by mockery v2.42.2. DO NOT EDIT.

package onchain

import (
	context "context"

	onchain "github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	mock "github.com/stretchr/testify/mock"
)

// MockBlockProvider is an autogenerated mock type for the BlockProvider type
type MockBlockProvider struct {
	mock.Mock
}

type MockBlockProvider_Expecter struct {
	mock *mock.Mock
}

func (_m *MockBlockProvider) EXPECT() *MockBlockProvider_Expecter {
	return &MockBlockProvider_Expecter{mock: &_m.Mock}
}

// Disconnect provides a mock function with given fields:
func (_m *MockBlockProvider) Disconnect() {
	_m.Called()
}

// MockBlockProvider_Disconnect_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Disconnect'
type MockBlockProvider_Disconnect_Call struct {
	*mock.Call
}

// Disconnect is a helper method to define mock.On call
func (_e *MockBlockProvider_Expecter) Disconnect() *MockBlockProvider_Disconnect_Call {
	return &MockBlockProvider_Disconnect_Call{Call: _e.mock.On("Disconnect")}
}

func (_c *MockBlockProvider_Disconnect_Call) Run(run func()) *MockBlockProvider_Disconnect_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockBlockProvider_Disconnect_Call) Return() *MockBlockProvider_Disconnect_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockBlockProvider_Disconnect_Call) RunAndReturn(run func()) *MockBlockProvider_Disconnect_Call {
	_c.Call.Return(run)
	return _c
}

// EstimateFee provides a mock function with given fields:
func (_m *MockBlockProvider) EstimateFee() (float64, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for EstimateFee")
	}

	var r0 float64
	var r1 error
	if rf, ok := ret.Get(0).(func() (float64, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() float64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(float64)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockBlockProvider_EstimateFee_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'EstimateFee'
type MockBlockProvider_EstimateFee_Call struct {
	*mock.Call
}

// EstimateFee is a helper method to define mock.On call
func (_e *MockBlockProvider_Expecter) EstimateFee() *MockBlockProvider_EstimateFee_Call {
	return &MockBlockProvider_EstimateFee_Call{Call: _e.mock.On("EstimateFee")}
}

func (_c *MockBlockProvider_EstimateFee_Call) Run(run func()) *MockBlockProvider_EstimateFee_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockBlockProvider_EstimateFee_Call) Return(_a0 float64, _a1 error) *MockBlockProvider_EstimateFee_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockBlockProvider_EstimateFee_Call) RunAndReturn(run func() (float64, error)) *MockBlockProvider_EstimateFee_Call {
	_c.Call.Return(run)
	return _c
}

// GetBlockHeight provides a mock function with given fields:
func (_m *MockBlockProvider) GetBlockHeight() (uint32, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetBlockHeight")
	}

	var r0 uint32
	var r1 error
	if rf, ok := ret.Get(0).(func() (uint32, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() uint32); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint32)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockBlockProvider_GetBlockHeight_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetBlockHeight'
type MockBlockProvider_GetBlockHeight_Call struct {
	*mock.Call
}

// GetBlockHeight is a helper method to define mock.On call
func (_e *MockBlockProvider_Expecter) GetBlockHeight() *MockBlockProvider_GetBlockHeight_Call {
	return &MockBlockProvider_GetBlockHeight_Call{Call: _e.mock.On("GetBlockHeight")}
}

func (_c *MockBlockProvider_GetBlockHeight_Call) Run(run func()) *MockBlockProvider_GetBlockHeight_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockBlockProvider_GetBlockHeight_Call) Return(_a0 uint32, _a1 error) *MockBlockProvider_GetBlockHeight_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockBlockProvider_GetBlockHeight_Call) RunAndReturn(run func() (uint32, error)) *MockBlockProvider_GetBlockHeight_Call {
	_c.Call.Return(run)
	return _c
}

// GetUnspentOutputs provides a mock function with given fields: address
func (_m *MockBlockProvider) GetUnspentOutputs(address string) ([]*onchain.Output, error) {
	ret := _m.Called(address)

	if len(ret) == 0 {
		panic("no return value specified for GetUnspentOutputs")
	}

	var r0 []*onchain.Output
	var r1 error
	if rf, ok := ret.Get(0).(func(string) ([]*onchain.Output, error)); ok {
		return rf(address)
	}
	if rf, ok := ret.Get(0).(func(string) []*onchain.Output); ok {
		r0 = rf(address)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*onchain.Output)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(address)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockBlockProvider_GetUnspentOutputs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetUnspentOutputs'
type MockBlockProvider_GetUnspentOutputs_Call struct {
	*mock.Call
}

// GetUnspentOutputs is a helper method to define mock.On call
//   - address string
func (_e *MockBlockProvider_Expecter) GetUnspentOutputs(address interface{}) *MockBlockProvider_GetUnspentOutputs_Call {
	return &MockBlockProvider_GetUnspentOutputs_Call{Call: _e.mock.On("GetUnspentOutputs", address)}
}

func (_c *MockBlockProvider_GetUnspentOutputs_Call) Run(run func(address string)) *MockBlockProvider_GetUnspentOutputs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockBlockProvider_GetUnspentOutputs_Call) Return(_a0 []*onchain.Output, _a1 error) *MockBlockProvider_GetUnspentOutputs_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockBlockProvider_GetUnspentOutputs_Call) RunAndReturn(run func(string) ([]*onchain.Output, error)) *MockBlockProvider_GetUnspentOutputs_Call {
	_c.Call.Return(run)
	return _c
}

// RegisterBlockListener provides a mock function with given fields: ctx, channel
func (_m *MockBlockProvider) RegisterBlockListener(ctx context.Context, channel chan<- *onchain.BlockEpoch) error {
	ret := _m.Called(ctx, channel)

	if len(ret) == 0 {
		panic("no return value specified for RegisterBlockListener")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, chan<- *onchain.BlockEpoch) error); ok {
		r0 = rf(ctx, channel)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockBlockProvider_RegisterBlockListener_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RegisterBlockListener'
type MockBlockProvider_RegisterBlockListener_Call struct {
	*mock.Call
}

// RegisterBlockListener is a helper method to define mock.On call
//   - ctx context.Context
//   - channel chan<- *onchain.BlockEpoch
func (_e *MockBlockProvider_Expecter) RegisterBlockListener(ctx interface{}, channel interface{}) *MockBlockProvider_RegisterBlockListener_Call {
	return &MockBlockProvider_RegisterBlockListener_Call{Call: _e.mock.On("RegisterBlockListener", ctx, channel)}
}

func (_c *MockBlockProvider_RegisterBlockListener_Call) Run(run func(ctx context.Context, channel chan<- *onchain.BlockEpoch)) *MockBlockProvider_RegisterBlockListener_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(chan<- *onchain.BlockEpoch))
	})
	return _c
}

func (_c *MockBlockProvider_RegisterBlockListener_Call) Return(_a0 error) *MockBlockProvider_RegisterBlockListener_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockBlockProvider_RegisterBlockListener_Call) RunAndReturn(run func(context.Context, chan<- *onchain.BlockEpoch) error) *MockBlockProvider_RegisterBlockListener_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockBlockProvider creates a new instance of MockBlockProvider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockBlockProvider(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockBlockProvider {
	mock := &MockBlockProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
