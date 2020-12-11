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
	}

	WritePermissions = []bakery.Op{
		{
			Entity: "info",
			Action: "write",
		},
		{
			Entity: "swap",
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
		"/boltzrpc.Boltz/ListSwaps": {{
			Entity: "swap",
			Action: "read",
		}},
		"/boltzrpc.Boltz/GetSwapInfo": {{
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
		"/boltzrpc.Boltz/CreateChannel": {{
			Entity: "swap",
			Action: "write",
		}},
		"/boltzrpc.Boltz/CreateReverseSwap": {{
			Entity: "swap",
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
