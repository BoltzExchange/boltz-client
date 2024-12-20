// Code generated by mockery v2.42.2. DO NOT EDIT.

package onchain

import (
	onchain "github.com/BoltzExchange/boltz-client/v2/onchain"
	mock "github.com/stretchr/testify/mock"
)

// MockWallet is an autogenerated mock type for the Wallet type
type MockWallet struct {
	mock.Mock
}

type MockWallet_Expecter struct {
	mock *mock.Mock
}

func (_m *MockWallet) EXPECT() *MockWallet_Expecter {
	return &MockWallet_Expecter{mock: &_m.Mock}
}

// Disconnect provides a mock function with given fields:
func (_m *MockWallet) Disconnect() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Disconnect")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockWallet_Disconnect_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Disconnect'
type MockWallet_Disconnect_Call struct {
	*mock.Call
}

// Disconnect is a helper method to define mock.On call
func (_e *MockWallet_Expecter) Disconnect() *MockWallet_Disconnect_Call {
	return &MockWallet_Disconnect_Call{Call: _e.mock.On("Disconnect")}
}

func (_c *MockWallet_Disconnect_Call) Run(run func()) *MockWallet_Disconnect_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWallet_Disconnect_Call) Return(_a0 error) *MockWallet_Disconnect_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockWallet_Disconnect_Call) RunAndReturn(run func() error) *MockWallet_Disconnect_Call {
	_c.Call.Return(run)
	return _c
}

// GetBalance provides a mock function with given fields:
func (_m *MockWallet) GetBalance() (*onchain.Balance, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetBalance")
	}

	var r0 *onchain.Balance
	var r1 error
	if rf, ok := ret.Get(0).(func() (*onchain.Balance, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *onchain.Balance); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*onchain.Balance)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWallet_GetBalance_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetBalance'
type MockWallet_GetBalance_Call struct {
	*mock.Call
}

// GetBalance is a helper method to define mock.On call
func (_e *MockWallet_Expecter) GetBalance() *MockWallet_GetBalance_Call {
	return &MockWallet_GetBalance_Call{Call: _e.mock.On("GetBalance")}
}

func (_c *MockWallet_GetBalance_Call) Run(run func()) *MockWallet_GetBalance_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWallet_GetBalance_Call) Return(_a0 *onchain.Balance, _a1 error) *MockWallet_GetBalance_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWallet_GetBalance_Call) RunAndReturn(run func() (*onchain.Balance, error)) *MockWallet_GetBalance_Call {
	_c.Call.Return(run)
	return _c
}

// GetTransactions provides a mock function with given fields: limit, offset
func (_m *MockWallet) GetTransactions(limit uint64, offset uint64) ([]*onchain.WalletTransaction, error) {
	ret := _m.Called(limit, offset)

	if len(ret) == 0 {
		panic("no return value specified for GetTransactions")
	}

	var r0 []*onchain.WalletTransaction
	var r1 error
	if rf, ok := ret.Get(0).(func(uint64, uint64) ([]*onchain.WalletTransaction, error)); ok {
		return rf(limit, offset)
	}
	if rf, ok := ret.Get(0).(func(uint64, uint64) []*onchain.WalletTransaction); ok {
		r0 = rf(limit, offset)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*onchain.WalletTransaction)
		}
	}

	if rf, ok := ret.Get(1).(func(uint64, uint64) error); ok {
		r1 = rf(limit, offset)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWallet_GetTransactions_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetTransactions'
type MockWallet_GetTransactions_Call struct {
	*mock.Call
}

// GetTransactions is a helper method to define mock.On call
//   - limit uint64
//   - offset uint64
func (_e *MockWallet_Expecter) GetTransactions(limit interface{}, offset interface{}) *MockWallet_GetTransactions_Call {
	return &MockWallet_GetTransactions_Call{Call: _e.mock.On("GetTransactions", limit, offset)}
}

func (_c *MockWallet_GetTransactions_Call) Run(run func(limit uint64, offset uint64)) *MockWallet_GetTransactions_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(uint64), args[1].(uint64))
	})
	return _c
}

func (_c *MockWallet_GetTransactions_Call) Return(_a0 []*onchain.WalletTransaction, _a1 error) *MockWallet_GetTransactions_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWallet_GetTransactions_Call) RunAndReturn(run func(uint64, uint64) ([]*onchain.WalletTransaction, error)) *MockWallet_GetTransactions_Call {
	_c.Call.Return(run)
	return _c
}

// GetWalletInfo provides a mock function with given fields:
func (_m *MockWallet) GetWalletInfo() onchain.WalletInfo {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetWalletInfo")
	}

	var r0 onchain.WalletInfo
	if rf, ok := ret.Get(0).(func() onchain.WalletInfo); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(onchain.WalletInfo)
	}

	return r0
}

// MockWallet_GetWalletInfo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetWalletInfo'
type MockWallet_GetWalletInfo_Call struct {
	*mock.Call
}

// GetWalletInfo is a helper method to define mock.On call
func (_e *MockWallet_Expecter) GetWalletInfo() *MockWallet_GetWalletInfo_Call {
	return &MockWallet_GetWalletInfo_Call{Call: _e.mock.On("GetWalletInfo")}
}

func (_c *MockWallet_GetWalletInfo_Call) Run(run func()) *MockWallet_GetWalletInfo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWallet_GetWalletInfo_Call) Return(_a0 onchain.WalletInfo) *MockWallet_GetWalletInfo_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockWallet_GetWalletInfo_Call) RunAndReturn(run func() onchain.WalletInfo) *MockWallet_GetWalletInfo_Call {
	_c.Call.Return(run)
	return _c
}

// NewAddress provides a mock function with given fields:
func (_m *MockWallet) NewAddress() (string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for NewAddress")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func() (string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWallet_NewAddress_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NewAddress'
type MockWallet_NewAddress_Call struct {
	*mock.Call
}

// NewAddress is a helper method to define mock.On call
func (_e *MockWallet_Expecter) NewAddress() *MockWallet_NewAddress_Call {
	return &MockWallet_NewAddress_Call{Call: _e.mock.On("NewAddress")}
}

func (_c *MockWallet_NewAddress_Call) Run(run func()) *MockWallet_NewAddress_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWallet_NewAddress_Call) Return(_a0 string, _a1 error) *MockWallet_NewAddress_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWallet_NewAddress_Call) RunAndReturn(run func() (string, error)) *MockWallet_NewAddress_Call {
	_c.Call.Return(run)
	return _c
}

// Ready provides a mock function with given fields:
func (_m *MockWallet) Ready() bool {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Ready")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// MockWallet_Ready_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Ready'
type MockWallet_Ready_Call struct {
	*mock.Call
}

// Ready is a helper method to define mock.On call
func (_e *MockWallet_Expecter) Ready() *MockWallet_Ready_Call {
	return &MockWallet_Ready_Call{Call: _e.mock.On("Ready")}
}

func (_c *MockWallet_Ready_Call) Run(run func()) *MockWallet_Ready_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWallet_Ready_Call) Return(_a0 bool) *MockWallet_Ready_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockWallet_Ready_Call) RunAndReturn(run func() bool) *MockWallet_Ready_Call {
	_c.Call.Return(run)
	return _c
}

// SendToAddress provides a mock function with given fields: address, amount, satPerVbyte, sendAll
func (_m *MockWallet) SendToAddress(address string, amount uint64, satPerVbyte float64, sendAll bool) (string, error) {
	ret := _m.Called(address, amount, satPerVbyte, sendAll)

	if len(ret) == 0 {
		panic("no return value specified for SendToAddress")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(string, uint64, float64, bool) (string, error)); ok {
		return rf(address, amount, satPerVbyte, sendAll)
	}
	if rf, ok := ret.Get(0).(func(string, uint64, float64, bool) string); ok {
		r0 = rf(address, amount, satPerVbyte, sendAll)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(string, uint64, float64, bool) error); ok {
		r1 = rf(address, amount, satPerVbyte, sendAll)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWallet_SendToAddress_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SendToAddress'
type MockWallet_SendToAddress_Call struct {
	*mock.Call
}

// SendToAddress is a helper method to define mock.On call
//   - address string
//   - amount uint64
//   - satPerVbyte float64
//   - sendAll bool
func (_e *MockWallet_Expecter) SendToAddress(address interface{}, amount interface{}, satPerVbyte interface{}, sendAll interface{}) *MockWallet_SendToAddress_Call {
	return &MockWallet_SendToAddress_Call{Call: _e.mock.On("SendToAddress", address, amount, satPerVbyte, sendAll)}
}

func (_c *MockWallet_SendToAddress_Call) Run(run func(address string, amount uint64, satPerVbyte float64, sendAll bool)) *MockWallet_SendToAddress_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(uint64), args[2].(float64), args[3].(bool))
	})
	return _c
}

func (_c *MockWallet_SendToAddress_Call) Return(_a0 string, _a1 error) *MockWallet_SendToAddress_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWallet_SendToAddress_Call) RunAndReturn(run func(string, uint64, float64, bool) (string, error)) *MockWallet_SendToAddress_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockWallet creates a new instance of MockWallet. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockWallet(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockWallet {
	mock := &MockWallet{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
