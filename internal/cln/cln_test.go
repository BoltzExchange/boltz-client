package cln

import (
	"context"
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/internal/cln/protos"
	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type mockNodeClient struct {
	protos.NodeClient
	xpayCalled bool
}

func (m *mockNodeClient) Xpay(ctx context.Context, in *protos.XpayRequest, opts ...grpc.CallOption) (*protos.XpayResponse, error) {
	m.xpayCalled = true
	return &protos.XpayResponse{
		AmountMsat: &protos.Amount{
			Msat: 100000,
		},
		AmountSentMsat: &protos.Amount{
			Msat: 101000,
		},
	}, nil
}

func TestCln_PayInvoice_ChanIdsWarning(t *testing.T) {
	mockClient := &mockNodeClient{}
	clnNode := &Cln{
		Client: mockClient,
	}

	// Passing non-empty chanIds should log a warning and proceed with Xpay rather than returning an error
	chanIds := []lightning.ChanId{12345}
	res, err := clnNode.PayInvoice(context.Background(), "lnbc...", 1000, 30, chanIds)

	require.NoError(t, err)
	require.True(t, mockClient.xpayCalled)
	require.NotNil(t, res)
	require.Equal(t, uint(1000), res.FeeMsat)
}
