package mempool

import (
	"errors"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

type lndFeeMock struct {
	mock.Mock
}

func (m *lndFeeMock) EstimateFee(confTarget int32) (*walletrpc.EstimateFeeResponse, error) {
	args := m.Called(confTarget)

	err := args.Error(1)
	firstArg := args.Get(0)

	if firstArg == nil {
		return nil, err
	}

	return firstArg.(*walletrpc.EstimateFeeResponse), err
}

func TestGetFeeEstimation(t *testing.T) {
	fee, err := Init(new(lndFeeMock), mempoolEndpoint).GetFeeEstimation()
	require.Nil(t, err)
	assert.GreaterOrEqual(t, fee, int64(2))
	assert.NotEqual(t, fee, int64(0))
}

func TestGetFeeEstimationFailed(t *testing.T) {
	lfm := new(lndFeeMock)
	lfm.On("EstimateFee", mock.Anything).Return(&walletrpc.EstimateFeeResponse{SatPerKw: 3000}, nil)

	fee, err := Init(lfm, "some incorrect mempool endpoint").GetFeeEstimation()
	require.Nil(t, err)
	assert.Equal(t, fee, int64(3))

	lfm.AssertCalled(t, "EstimateFee", int32(2))
	lfm.AssertExpectations(t)
}

func TestGetLndFeeEstimation(t *testing.T) {
	lfm := new(lndFeeMock)
	lfm.On("EstimateFee", mock.Anything).Return(&walletrpc.EstimateFeeResponse{SatPerKw: 5023}, nil)

	fee, err := Init(lfm, mempoolEndpoint).getLndFeeEstimation()
	require.Nil(t, err)
	assert.Equal(t, fee, int64(5))

	lfm.AssertCalled(t, "EstimateFee", int32(2))
	lfm.AssertExpectations(t)
}

func TestGetLndFeeEstimationFloor(t *testing.T) {
	lfm := new(lndFeeMock)
	lfm.On("EstimateFee", mock.Anything).Return(&walletrpc.EstimateFeeResponse{SatPerKw: 1321}, nil)

	fee, err := Init(lfm, mempoolEndpoint).getLndFeeEstimation()
	require.Nil(t, err)
	assert.Equal(t, fee, int64(2))

	lfm.AssertCalled(t, "EstimateFee", int32(2))
	lfm.AssertExpectations(t)
}

func TestGetLndFeeEstimationError(t *testing.T) {
	lfm := new(lndFeeMock)

	expectedErr := errors.New("some LND error")
	lfm.On("EstimateFee", mock.Anything).Return(nil, expectedErr)

	fee, err := Init(lfm, mempoolEndpoint).getLndFeeEstimation()
	assert.Equal(t, err, expectedErr)
	assert.Equal(t, fee, int64(0))
}

func TestMaxInt64(t *testing.T) {
	assert.Equal(t, maxInt64(1, 2), int64(2))
	assert.Equal(t, maxInt64(2, 1), int64(2))
	assert.Equal(t, maxInt64(420, 13), int64(420))
}
