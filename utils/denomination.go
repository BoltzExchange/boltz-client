package utils

func GetSmallestUnitName(symbol string) string {
	switch symbol {
	case "LTC":
		return "litoshi"

	default:
		return "satoshi"
	}
}
