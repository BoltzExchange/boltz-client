// Code generated by mockery v2.42.2. DO NOT EDIT.

package onchain

import mock "github.com/stretchr/testify/mock"

// MockTxProvider is an autogenerated mock type for the TxProvider type
type MockTxProvider struct {
	mock.Mock
}

type MockTxProvider_Expecter struct {
	mock *mock.Mock
}

func (_m *MockTxProvider) EXPECT() *MockTxProvider_Expecter {
	return &MockTxProvider_Expecter{mock: &_m.Mock}
}

// BroadcastTransaction provides a mock function with given fields: txHex
func (_m *MockTxProvider) BroadcastTransaction(txHex string) (string, error) {
	ret := _m.Called(txHex)

	if len(ret) == 0 {
		panic("no return value specified for BroadcastTransaction")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (string, error)); ok {
		return rf(txHex)
	}
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(txHex)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(txHex)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTxProvider_BroadcastTransaction_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BroadcastTransaction'
type MockTxProvider_BroadcastTransaction_Call struct {
	*mock.Call
}

// BroadcastTransaction is a helper method to define mock.On call
//   - txHex string
func (_e *MockTxProvider_Expecter) BroadcastTransaction(txHex interface{}) *MockTxProvider_BroadcastTransaction_Call {
	return &MockTxProvider_BroadcastTransaction_Call{Call: _e.mock.On("BroadcastTransaction", txHex)}
}

func (_c *MockTxProvider_BroadcastTransaction_Call) Run(run func(txHex string)) *MockTxProvider_BroadcastTransaction_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockTxProvider_BroadcastTransaction_Call) Return(_a0 string, _a1 error) *MockTxProvider_BroadcastTransaction_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTxProvider_BroadcastTransaction_Call) RunAndReturn(run func(string) (string, error)) *MockTxProvider_BroadcastTransaction_Call {
	_c.Call.Return(run)
	return _c
}

// GetRawTransaction provides a mock function with given fields: txId
func (_m *MockTxProvider) GetRawTransaction(txId string) (string, error) {
	ret := _m.Called(txId)

	if len(ret) == 0 {
		panic("no return value specified for GetRawTransaction")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (string, error)); ok {
		return rf(txId)
	}
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(txId)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(txId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTxProvider_GetRawTransaction_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRawTransaction'
type MockTxProvider_GetRawTransaction_Call struct {
	*mock.Call
}

// GetRawTransaction is a helper method to define mock.On call
//   - txId string
func (_e *MockTxProvider_Expecter) GetRawTransaction(txId interface{}) *MockTxProvider_GetRawTransaction_Call {
	return &MockTxProvider_GetRawTransaction_Call{Call: _e.mock.On("GetRawTransaction", txId)}
}

func (_c *MockTxProvider_GetRawTransaction_Call) Run(run func(txId string)) *MockTxProvider_GetRawTransaction_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockTxProvider_GetRawTransaction_Call) Return(_a0 string, _a1 error) *MockTxProvider_GetRawTransaction_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTxProvider_GetRawTransaction_Call) RunAndReturn(run func(string) (string, error)) *MockTxProvider_GetRawTransaction_Call {
	_c.Call.Return(run)
	return _c
}

// IsTransactionConfirmed provides a mock function with given fields: txId
func (_m *MockTxProvider) IsTransactionConfirmed(txId string) (bool, error) {
	ret := _m.Called(txId)

	if len(ret) == 0 {
		panic("no return value specified for IsTransactionConfirmed")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (bool, error)); ok {
		return rf(txId)
	}
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(txId)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(txId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTxProvider_IsTransactionConfirmed_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'IsTransactionConfirmed'
type MockTxProvider_IsTransactionConfirmed_Call struct {
	*mock.Call
}

// IsTransactionConfirmed is a helper method to define mock.On call
//   - txId string
func (_e *MockTxProvider_Expecter) IsTransactionConfirmed(txId interface{}) *MockTxProvider_IsTransactionConfirmed_Call {
	return &MockTxProvider_IsTransactionConfirmed_Call{Call: _e.mock.On("IsTransactionConfirmed", txId)}
}

func (_c *MockTxProvider_IsTransactionConfirmed_Call) Run(run func(txId string)) *MockTxProvider_IsTransactionConfirmed_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockTxProvider_IsTransactionConfirmed_Call) Return(_a0 bool, _a1 error) *MockTxProvider_IsTransactionConfirmed_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTxProvider_IsTransactionConfirmed_Call) RunAndReturn(run func(string) (bool, error)) *MockTxProvider_IsTransactionConfirmed_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockTxProvider creates a new instance of MockTxProvider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockTxProvider(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockTxProvider {
	mock := &MockTxProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
