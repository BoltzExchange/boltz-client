package macaroons

import "gopkg.in/macaroon-bakery.v2/bakery"

var (
	ReadPermissions = []bakery.Op{
		{
			Entity: "info",
			Action: "read",
		},
		{
			Entity: "swap",
			Action: "read",
		},
		{
			Entity: "liquid",
			Action: "read",
		},
		{
			Entity: "autoswap",
			Action: "read",
		},
	}

	WritePermissions = []bakery.Op{
		{
			Entity: "info",
			Action: "write",
		},
		{
			Entity: "admin",
			Action: "write",
		},
		{
			Entity: "swap",
			Action: "write",
		},
		{
			Entity: "liquid",
			Action: "write",
		},
		{
			Entity: "autoswap",
			Action: "write",
		},
	}

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
			Entity: "liquid",
			Action: "write",
		}},
		"/boltzrpc.Boltz/ImportWallet": {{
			Entity: "liquid",
			Action: "write",
		}},
		"/boltzrpc.Boltz/SetSubaccount": {{
			Entity: "liquid",
			Action: "write",
		}},
		"/boltzrpc.Boltz/GetSubaccounts": {{
			Entity: "liquid",
			Action: "read",
		}},
		"/boltzrpc.Boltz/RemoveWallet": {{
			Entity: "liquid",
			Action: "write",
		}},
		"/boltzrpc.Boltz/GetWalletCredentials": {{
			Entity: "liquid",
			Action: "write",
		}},
		"/boltzrpc.Boltz/GetWallets": {{
			Entity: "liquid",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetWallet": {{
			Entity: "liquid",
			Action: "read",
		}},
		"/boltzrpc.Boltz/Stop": {{
			Entity: "info",
			Action: "write",
		}},
		"/boltzrpc.Boltz/Unlock": {{
			Entity: "info",
			Action: "write",
		}},
		"/boltzrpc.Boltz/ChangeWalletPassword": {{
			Entity: "info",
			Action: "write",
		}},
		"/boltzrpc.Boltz/VerifyWalletPassword": {{
			Entity: "info",
			Action: "read",
		}},
		"/boltzrpc.Boltz/CreateEntity": {{
			Entity: "info",
			Action: "write",
		}},
		"/boltzrpc.Boltz/GetEntities": {{
			Entity: "info",
			Action: "write",
		}},
		"/boltzrpc.Boltz/BakeMacaroon": {{
			Entity: "info",
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
