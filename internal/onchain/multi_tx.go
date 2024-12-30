package onchain

type MultiTxProvider struct {
	TxProvider
	Providers []TxProvider
}

func (m MultiTxProvider) GetRawTransaction(txId string) (hex string, err error) {
	for _, provider := range m.Providers {
		hex, err = provider.GetRawTransaction(txId)
		if err == nil {
			return hex, nil
		}
	}
	return "", err
}

func (m MultiTxProvider) BroadcastTransaction(txHex string) (txId string, err error) {
	for _, provider := range m.Providers {
		txId, err = provider.BroadcastTransaction(txHex)
		if err == nil {
			return txId, nil
		}
	}
	return "", err
}

func (m MultiTxProvider) IsTransactionConfirmed(txId string) (confirmed bool, err error) {
	for _, provider := range m.Providers {
		confirmed, err = provider.IsTransactionConfirmed(txId)
		if err == nil {
			return confirmed, nil
		}
	}
	return false, err
}
