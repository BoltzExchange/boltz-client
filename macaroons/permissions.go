package macaroons

import (
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"gopkg.in/macaroon-bakery.v2/bakery"
)

var (
	EntityReadPermissions = []bakery.Op{
		{
			Entity: "info",
			Action: "read",
		},
		{
			Entity: "swap",
			Action: "read",
		},
		{
			Entity: "wallet",
			Action: "read",
		},
	}
	EntityWritePermissions = []bakery.Op{
		{
			Entity: "info",
			Action: "write",
		},
		{
			Entity: "swap",
			Action: "write",
		},
		{
			Entity: "wallet",
			Action: "write",
		},
	}
	ReadPermissions = append([]bakery.Op{
		{
			Entity: "admin",
			Action: "read",
		},
		{
			Entity: "autoswap",
			Action: "read",
		},
	}, EntityReadPermissions...)

	WritePermissions = append([]bakery.Op{
		{
			Entity: "admin",
			Action: "write",
		},
		{
			Entity: "autoswap",
			Action: "write",
		},
	}, EntityWritePermissions...)

	RPCServerPermissions = map[string][]bakery.Op{
		"/boltzrpc.Boltz/GetInfo": {{
			Entity: "info",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetServiceInfo": {{
			Entity: "info",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetSubmarinePair": {{
			Entity: "info",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetReversePair": {{
			Entity: "info",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetPairs": {{
			Entity: "info",
			Action: "read",
		}},
		"/boltzrpc.Boltz/ListSwaps": {{
			Entity: "swap",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetSwapInfo": {{
			Entity: "swap",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetSwapInfoStream": {{
			Entity: "swap",
			Action: "read",
		}},
		"/boltzrpc.Boltz/Deposit": {{
			Entity: "swap",
			Action: "write",
		}},
		"/boltzrpc.Boltz/CreateSwap": {{
			Entity: "swap",
			Action: "write",
		}},
		"/boltzrpc.Boltz/RefundSwap": {{
			Entity: "swap",
			Action: "write",
		}},
		"/boltzrpc.Boltz/CreateChannel": {{
			Entity: "swap",
			Action: "write",
		}},
		"/boltzrpc.Boltz/CreateReverseSwap": {{
			Entity: "swap",
			Action: "write",
		}},
		"/boltzrpc.Boltz/CreateWallet": {{
			Entity: "wallet",
			Action: "write",
		}},
		"/boltzrpc.Boltz/ImportWallet": {{
			Entity: "wallet",
			Action: "write",
		}},
		"/boltzrpc.Boltz/SetSubaccount": {{
			Entity: "wallet",
			Action: "write",
		}},
		"/boltzrpc.Boltz/GetSubaccounts": {{
			Entity: "wallet",
			Action: "read",
		}},
		"/boltzrpc.Boltz/RemoveWallet": {{
			Entity: "wallet",
			Action: "write",
		}},
		"/boltzrpc.Boltz/GetWalletCredentials": {{
			Entity: "wallet",
			Action: "write",
		}},
		"/boltzrpc.Boltz/GetWallets": {{
			Entity: "wallet",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetWallet": {{
			Entity: "wallet",
			Action: "read",
		}},
		"/boltzrpc.Boltz/Stop": {{
			Entity: "admin",
			Action: "write",
		}},
		"/boltzrpc.Boltz/Unlock": {{
			Entity: "admin",
			Action: "write",
		}},
		"/boltzrpc.Boltz/ChangeWalletPassword": {{
			Entity: "admin",
			Action: "write",
		}},
		"/boltzrpc.Boltz/VerifyWalletPassword": {{
			Entity: "admin",
			Action: "read",
		}},
		"/boltzrpc.Boltz/CreateEntity": {{
			Entity: "admin",
			Action: "write",
		}},
		"/boltzrpc.Boltz/ListEntities": {{
			Entity: "admin",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetEntity": {{
			Entity: "admin",
			Action: "read",
		}},
		"/boltzrpc.Boltz/BakeMacaroon": {{
			Entity: "admin",
			Action: "write",
		}},
		"/autoswaprpc.AutoSwap/GetSwapRecommendations": {{
			Entity: "autoswap",
			Action: "read",
		}},
		"/autoswaprpc.AutoSwap/GetStatus": {{
			Entity: "autoswap",
			Action: "read",
		}},
		"/autoswaprpc.AutoSwap/GetConfig": {{
			Entity: "autoswap",
			Action: "read",
		}},
		"/autoswaprpc.AutoSwap/ResetConfig": {{
			Entity: "autoswap",
			Action: "write",
		}},
		"/autoswaprpc.AutoSwap/ReloadConfig": {{
			Entity: "autoswap",
			Action: "write",
		}},
		"/autoswaprpc.AutoSwap/SetConfig": {{
			Entity: "autoswap",
			Action: "write",
		}},
		"/autoswaprpc.AutoSwap/SetConfigValue": {{
			Entity: "autoswap",
			Action: "write",
		}},
	}
)

func AdminPermissions() []bakery.Op {
	admin := make([]bakery.Op, len(ReadPermissions)+len(WritePermissions))
	copy(admin, ReadPermissions)
	copy(admin[len(ReadPermissions):], WritePermissions)

	return admin
}

func GetPermissions(isEntity bool, permissions []*boltzrpc.MacaroonPermissions) (result []bakery.Op) {
	for _, permission := range permissions {
		if permission.Action == boltzrpc.MacaroonAction_READ {
			if isEntity {
				result = append(result, EntityReadPermissions...)
			} else {
				result = append(result, ReadPermissions...)
			}
		} else if permission.Action == boltzrpc.MacaroonAction_WRITE {
			if isEntity {
				result = append(result, EntityWritePermissions...)
			} else {
				result = append(result, WritePermissions...)
			}
		}
	}
	return result
}
