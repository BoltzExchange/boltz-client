package database

type SwapMnemonic struct {
	Mnemonic     string
	LastKeyIndex uint32
}

func (db *Database) GetSwapMnemonic() (*SwapMnemonic, error) {
	row := db.QueryRow("SELECT mnemonic, lastKeyIndex FROM swapMnemonic")
	var mnemonic SwapMnemonic
	err := row.Scan(&mnemonic.Mnemonic, &mnemonic.LastKeyIndex)
	if err != nil {
		return nil, err
	}
	return &mnemonic, nil
}

func (tx *Transaction) SetSwapMnemonic(mnemonic *SwapMnemonic) error {
	_, err := tx.Exec("DELETE FROM swapMnemonic")
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO swapMnemonic (mnemonic, lastKeyIndex) VALUES (?, ?)", mnemonic.Mnemonic, mnemonic.LastKeyIndex)
	return err
}
