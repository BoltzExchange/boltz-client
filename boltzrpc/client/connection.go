package client

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"os"
	"strconv"
)

type Connection struct {
	*grpc.ClientConn
	Host string
	Port int

	TlsCertPath string

	NoMacaroons  bool
	MacaroonPath string

	Ctx context.Context
}

func (connection *Connection) Connect() error {
	creds, err := credentials.NewClientTLSFromFile(connection.TlsCertPath, "")

	if err != nil {
		return errors.New(fmt.Sprint("could not read connection certificate: ", err))
	}
	con, err := grpc.Dial(connection.Host+":"+strconv.Itoa(connection.Port), grpc.WithTransportCredentials(creds))

	if err != nil {
		return err
	}

	connection.ClientConn = con

	if connection.Ctx == nil {
		connection.Ctx = context.Background()

		if !connection.NoMacaroons {
			macaroonFile, err := os.ReadFile(connection.MacaroonPath)

			if err != nil {
				return errors.New(fmt.Sprint("could not read connection macaroon: ", err))
			}

			macaroon := metadata.Pairs("macaroon", hex.EncodeToString(macaroonFile))
			connection.Ctx = metadata.NewOutgoingContext(connection.Ctx, macaroon)
		}
	}

	return nil
}
