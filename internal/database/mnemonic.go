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

func (db *Database) IncrementSwapMnemonicKey(mnemonic string) error {
	_, err := db.Exec("UPDATE swapMnemonic SET lastKeyIndex = lastKeyIndex + 1 WHERE mnemonic = ?", mnemonic)
	return err
}

func (tx *Transaction) SetSwapMnemonic(mnemonic string) error {
	_, err := tx.Exec("DELETE FROM swapMnemonic")
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO swapMnemonic (mnemonic) VALUES (?)", mnemonic)
	return err
}
