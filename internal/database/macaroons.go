package database

import (
	"database/sql"
	"encoding/hex"
	"errors"
)

type Macaroon struct {
	Id      []byte
	RootKey []byte
}

func parseMacaroon(rows *sql.Rows) (*Macaroon, error) {
	var macaroon Macaroon

	var id string
	var rootKey string

	err := scanRow(
		rows,
		map[string]interface{}{
			"id":      &id,
			"rootKey": &rootKey,
		},
	)

	if err != nil {
		return nil, err
	}

	macaroon.Id, err = hex.DecodeString(id)

	if err != nil {
		return nil, err
	}

	macaroon.RootKey, err = hex.DecodeString(rootKey)

	if err != nil {
		return nil, err
	}

	return &macaroon, err
}

func (database *Database) QueryMacaroon(id []byte) (macaroon *Macaroon, err error) {
	database.lock.RLock()
	defer database.lock.RUnlock()
	rows, err := database.Query("SELECT * FROM macaroons WHERE id = '" + hex.EncodeToString(id) + "'")

	if err != nil {
		return macaroon, err
	}

	defer rows.Close()

	if rows.Next() {
		macaroon, err = parseMacaroon(rows)

		if err != nil {
			return macaroon, err
		}
	} else {
		return macaroon, errors.New("could not find Macaroon: " + hex.EncodeToString(id))
	}

	return macaroon, err
}

func (database *Database) CreateMacaroon(macaroon Macaroon) error {
	_, err := database.Exec(
		"INSERT INTO macaroons (id, rootKey) VALUES (?, ?)",
		hex.EncodeToString(macaroon.Id),
		hex.EncodeToString(macaroon.RootKey),
	)
	return err
}
