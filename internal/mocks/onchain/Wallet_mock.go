// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package onchain

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	mock "github.com/stretchr/testify/mock"
)

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

// BumpTransactionFee provides a mock function for the type MockWallet
func (_mock *MockWallet) BumpTransactionFee(txId string, satPerVbyte float64) (string, error) {
	ret := _mock.Called(txId, satPerVbyte)

	if len(ret) == 0 {
		panic("no return value specified for BumpTransactionFee")
	}

	var r0 string
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(string, float64) (string, error)); ok {
		return returnFunc(txId, satPerVbyte)
	}
	if returnFunc, ok := ret.Get(0).(func(string, float64) string); ok {
		r0 = returnFunc(txId, satPerVbyte)
	} else {
		r0 = ret.Get(0).(string)
	}
	if returnFunc, ok := ret.Get(1).(func(string, float64) error); ok {
		r1 = returnFunc(txId, satPerVbyte)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockWallet_BumpTransactionFee_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BumpTransactionFee'
type MockWallet_BumpTransactionFee_Call struct {
	*mock.Call
}

// BumpTransactionFee is a helper method to define mock.On call
//   - txId string
//   - satPerVbyte float64
func (_e *MockWallet_Expecter) BumpTransactionFee(txId interface{}, satPerVbyte interface{}) *MockWallet_BumpTransactionFee_Call {
	return &MockWallet_BumpTransactionFee_Call{Call: _e.mock.On("BumpTransactionFee", txId, satPerVbyte)}
}

func (_c *MockWallet_BumpTransactionFee_Call) Run(run func(txId string, satPerVbyte float64)) *MockWallet_BumpTransactionFee_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 string
		if args[0] != nil {
			arg0 = args[0].(string)
		}
		var arg1 float64
		if args[1] != nil {
			arg1 = args[1].(float64)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *MockWallet_BumpTransactionFee_Call) Return(s string, err error) *MockWallet_BumpTransactionFee_Call {
	_c.Call.Return(s, err)
	return _c
}

func (_c *MockWallet_BumpTransactionFee_Call) RunAndReturn(run func(txId string, satPerVbyte float64) (string, error)) *MockWallet_BumpTransactionFee_Call {
	_c.Call.Return(run)
	return _c
}

// Disconnect provides a mock function for the type MockWallet
func (_mock *MockWallet) Disconnect() error {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Disconnect")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func() error); ok {
		r0 = returnFunc()
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

func (_c *MockWallet_Disconnect_Call) Return(err error) *MockWallet_Disconnect_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockWallet_Disconnect_Call) RunAndReturn(run func() error) *MockWallet_Disconnect_Call {
	_c.Call.Return(run)
	return _c
}

// GetBalance provides a mock function for the type MockWallet
func (_mock *MockWallet) GetBalance() (*onchain.Balance, error) {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetBalance")
	}

	var r0 *onchain.Balance
	var r1 error
	if returnFunc, ok := ret.Get(0).(func() (*onchain.Balance, error)); ok {
		return returnFunc()
	}
	if returnFunc, ok := ret.Get(0).(func() *onchain.Balance); ok {
		r0 = returnFunc()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*onchain.Balance)
		}
	}
	if returnFunc, ok := ret.Get(1).(func() error); ok {
		r1 = returnFunc()
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

func (_c *MockWallet_GetBalance_Call) Return(balance *onchain.Balance, err error) *MockWallet_GetBalance_Call {
	_c.Call.Return(balance, err)
	return _c
}

func (_c *MockWallet_GetBalance_Call) RunAndReturn(run func() (*onchain.Balance, error)) *MockWallet_GetBalance_Call {
	_c.Call.Return(run)
	return _c
}

// GetOutputs provides a mock function for the type MockWallet
func (_mock *MockWallet) GetOutputs(address string) ([]*onchain.Output, error) {
	ret := _mock.Called(address)

	if len(ret) == 0 {
		panic("no return value specified for GetOutputs")
	}

	var r0 []*onchain.Output
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(string) ([]*onchain.Output, error)); ok {
		return returnFunc(address)
	}
	if returnFunc, ok := ret.Get(0).(func(string) []*onchain.Output); ok {
		r0 = returnFunc(address)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*onchain.Output)
		}
	}
	if returnFunc, ok := ret.Get(1).(func(string) error); ok {
		r1 = returnFunc(address)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockWallet_GetOutputs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetOutputs'
type MockWallet_GetOutputs_Call struct {
	*mock.Call
}

// GetOutputs is a helper method to define mock.On call
//   - address string
func (_e *MockWallet_Expecter) GetOutputs(address interface{}) *MockWallet_GetOutputs_Call {
	return &MockWallet_GetOutputs_Call{Call: _e.mock.On("GetOutputs", address)}
}

func (_c *MockWallet_GetOutputs_Call) Run(run func(address string)) *MockWallet_GetOutputs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 string
		if args[0] != nil {
			arg0 = args[0].(string)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *MockWallet_GetOutputs_Call) Return(outputs []*onchain.Output, err error) *MockWallet_GetOutputs_Call {
	_c.Call.Return(outputs, err)
	return _c
}

func (_c *MockWallet_GetOutputs_Call) RunAndReturn(run func(address string) ([]*onchain.Output, error)) *MockWallet_GetOutputs_Call {
	_c.Call.Return(run)
	return _c
}

// GetSendFee provides a mock function for the type MockWallet
func (_mock *MockWallet) GetSendFee(args onchain.WalletSendArgs) (uint64, uint64, error) {
	ret := _mock.Called(args)

	if len(ret) == 0 {
		panic("no return value specified for GetSendFee")
	}

	var r0 uint64
	var r1 uint64
	var r2 error
	if returnFunc, ok := ret.Get(0).(func(onchain.WalletSendArgs) (uint64, uint64, error)); ok {
		return returnFunc(args)
	}
	if returnFunc, ok := ret.Get(0).(func(onchain.WalletSendArgs) uint64); ok {
		r0 = returnFunc(args)
	} else {
		r0 = ret.Get(0).(uint64)
	}
	if returnFunc, ok := ret.Get(1).(func(onchain.WalletSendArgs) uint64); ok {
		r1 = returnFunc(args)
	} else {
		r1 = ret.Get(1).(uint64)
	}
	if returnFunc, ok := ret.Get(2).(func(onchain.WalletSendArgs) error); ok {
		r2 = returnFunc(args)
	} else {
		r2 = ret.Error(2)
	}
	return r0, r1, r2
}

// MockWallet_GetSendFee_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetSendFee'
type MockWallet_GetSendFee_Call struct {
	*mock.Call
}

// GetSendFee is a helper method to define mock.On call
//   - args onchain.WalletSendArgs
func (_e *MockWallet_Expecter) GetSendFee(args interface{}) *MockWallet_GetSendFee_Call {
	return &MockWallet_GetSendFee_Call{Call: _e.mock.On("GetSendFee", args)}
}

func (_c *MockWallet_GetSendFee_Call) Run(run func(args onchain.WalletSendArgs)) *MockWallet_GetSendFee_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 onchain.WalletSendArgs
		if args[0] != nil {
			arg0 = args[0].(onchain.WalletSendArgs)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *MockWallet_GetSendFee_Call) Return(send uint64, fee uint64, err error) *MockWallet_GetSendFee_Call {
	_c.Call.Return(send, fee, err)
	return _c
}

func (_c *MockWallet_GetSendFee_Call) RunAndReturn(run func(args onchain.WalletSendArgs) (uint64, uint64, error)) *MockWallet_GetSendFee_Call {
	_c.Call.Return(run)
	return _c
}

// GetTransactions provides a mock function for the type MockWallet
func (_mock *MockWallet) GetTransactions(limit uint64, offset uint64) ([]*onchain.WalletTransaction, error) {
	ret := _mock.Called(limit, offset)

	if len(ret) == 0 {
		panic("no return value specified for GetTransactions")
	}

	var r0 []*onchain.WalletTransaction
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(uint64, uint64) ([]*onchain.WalletTransaction, error)); ok {
		return returnFunc(limit, offset)
	}
	if returnFunc, ok := ret.Get(0).(func(uint64, uint64) []*onchain.WalletTransaction); ok {
		r0 = returnFunc(limit, offset)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*onchain.WalletTransaction)
		}
	}
	if returnFunc, ok := ret.Get(1).(func(uint64, uint64) error); ok {
		r1 = returnFunc(limit, offset)
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
		var arg0 uint64
		if args[0] != nil {
			arg0 = args[0].(uint64)
		}
		var arg1 uint64
		if args[1] != nil {
			arg1 = args[1].(uint64)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *MockWallet_GetTransactions_Call) Return(walletTransactions []*onchain.WalletTransaction, err error) *MockWallet_GetTransactions_Call {
	_c.Call.Return(walletTransactions, err)
	return _c
}

func (_c *MockWallet_GetTransactions_Call) RunAndReturn(run func(limit uint64, offset uint64) ([]*onchain.WalletTransaction, error)) *MockWallet_GetTransactions_Call {
	_c.Call.Return(run)
	return _c
}

// GetWalletInfo provides a mock function for the type MockWallet
func (_mock *MockWallet) GetWalletInfo() onchain.WalletInfo {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetWalletInfo")
	}

	var r0 onchain.WalletInfo
	if returnFunc, ok := ret.Get(0).(func() onchain.WalletInfo); ok {
		r0 = returnFunc()
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

func (_c *MockWallet_GetWalletInfo_Call) Return(walletInfo onchain.WalletInfo) *MockWallet_GetWalletInfo_Call {
	_c.Call.Return(walletInfo)
	return _c
}

func (_c *MockWallet_GetWalletInfo_Call) RunAndReturn(run func() onchain.WalletInfo) *MockWallet_GetWalletInfo_Call {
	_c.Call.Return(run)
	return _c
}

// NewAddress provides a mock function for the type MockWallet
func (_mock *MockWallet) NewAddress() (string, error) {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for NewAddress")
	}

	var r0 string
	var r1 error
	if returnFunc, ok := ret.Get(0).(func() (string, error)); ok {
		return returnFunc()
	}
	if returnFunc, ok := ret.Get(0).(func() string); ok {
		r0 = returnFunc()
	} else {
		r0 = ret.Get(0).(string)
	}
	if returnFunc, ok := ret.Get(1).(func() error); ok {
		r1 = returnFunc()
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

func (_c *MockWallet_NewAddress_Call) Return(s string, err error) *MockWallet_NewAddress_Call {
	_c.Call.Return(s, err)
	return _c
}

func (_c *MockWallet_NewAddress_Call) RunAndReturn(run func() (string, error)) *MockWallet_NewAddress_Call {
	_c.Call.Return(run)
	return _c
}

// Ready provides a mock function for the type MockWallet
func (_mock *MockWallet) Ready() bool {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Ready")
	}

	var r0 bool
	if returnFunc, ok := ret.Get(0).(func() bool); ok {
		r0 = returnFunc()
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

func (_c *MockWallet_Ready_Call) Return(b bool) *MockWallet_Ready_Call {
	_c.Call.Return(b)
	return _c
}

func (_c *MockWallet_Ready_Call) RunAndReturn(run func() bool) *MockWallet_Ready_Call {
	_c.Call.Return(run)
	return _c
}

// SendToAddress provides a mock function for the type MockWallet
func (_mock *MockWallet) SendToAddress(args onchain.WalletSendArgs) (string, error) {
	ret := _mock.Called(args)

	if len(ret) == 0 {
		panic("no return value specified for SendToAddress")
	}

	var r0 string
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(onchain.WalletSendArgs) (string, error)); ok {
		return returnFunc(args)
	}
	if returnFunc, ok := ret.Get(0).(func(onchain.WalletSendArgs) string); ok {
		r0 = returnFunc(args)
	} else {
		r0 = ret.Get(0).(string)
	}
	if returnFunc, ok := ret.Get(1).(func(onchain.WalletSendArgs) error); ok {
		r1 = returnFunc(args)
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
//   - args onchain.WalletSendArgs
func (_e *MockWallet_Expecter) SendToAddress(args interface{}) *MockWallet_SendToAddress_Call {
	return &MockWallet_SendToAddress_Call{Call: _e.mock.On("SendToAddress", args)}
}

func (_c *MockWallet_SendToAddress_Call) Run(run func(args onchain.WalletSendArgs)) *MockWallet_SendToAddress_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 onchain.WalletSendArgs
		if args[0] != nil {
			arg0 = args[0].(onchain.WalletSendArgs)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *MockWallet_SendToAddress_Call) Return(s string, err error) *MockWallet_SendToAddress_Call {
	_c.Call.Return(s, err)
	return _c
}

func (_c *MockWallet_SendToAddress_Call) RunAndReturn(run func(args onchain.WalletSendArgs) (string, error)) *MockWallet_SendToAddress_Call {
	_c.Call.Return(run)
	return _c
}

// Sync provides a mock function for the type MockWallet
func (_mock *MockWallet) Sync() error {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Sync")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func() error); ok {
		r0 = returnFunc()
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// MockWallet_Sync_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Sync'
type MockWallet_Sync_Call struct {
	*mock.Call
}

// Sync is a helper method to define mock.On call
func (_e *MockWallet_Expecter) Sync() *MockWallet_Sync_Call {
	return &MockWallet_Sync_Call{Call: _e.mock.On("Sync")}
}

func (_c *MockWallet_Sync_Call) Run(run func()) *MockWallet_Sync_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWallet_Sync_Call) Return(err error) *MockWallet_Sync_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockWallet_Sync_Call) RunAndReturn(run func() error) *MockWallet_Sync_Call {
	_c.Call.Return(run)
	return _c
}
