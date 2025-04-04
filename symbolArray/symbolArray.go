package symbolArray

var Symbols = map[string]string{
	"BTC/USDT": "BINANCE:BTCUSDT",
	"ETH":      "BINANCE:ETHUSDT",
	"LTC":      "BINANCE:LTCUSDT",
	"DOGE":     "BINANCE:DOGEUSDT",
	"ADA":      "BINANCE:ADAUSDT",
	"BNB":      "BINANCE:BNBUSDT",
	"SOL":      "BINANCE:SOLUSDT",
	"XRP": 		"BINANCE:XRPUSDT",
	"SUI":      "BINANCE:SUIUSDT",
	"LINK":      "BINANCE:LINKUSDT",
	"TON":     "BINANCE:TONUSDT",
	"SHIB":      "BINANCE:SHIBUSDT",
	"AAVE":      "BINANCE:AAVEUSDT",
	"AVAX":      "BINANCE:AVAXUSDT",
	"AAPL":     "AAPL",
	"MSFT":     "MSFT",
	"AMZN":     "AMZN",
	"GOOGL":    "GOOGL",
	"NFLX":     "NFLX",
	"META":     "META",
	"AMD":      "AMD",
	"MZDAY":    "MZDAY",
	"TSLA":     "TSLA",
	"NVDA":     "NVDA",
	"PLTR":     "PLTR",
	"INTC":     "INTC",
	"SNAP":     "SNAP",
	"KO":       "KO",
	"MCD":      "MCD",
	"RBLX":     "RBLX",
	"GE":       "GE",
	"AAL":      "AAL",
	"BAC":      "BAC",
	"UBER":     "UBER",
	"F":        "F",
	"BB":       "BB",
	"TM":       "TM",
	"^SPX": "^SPX",
	"^DJI": "^DJI",
	"QYLD": "QYLD",
	"RYLD": "RYLD",
}

// Add NASDAQ 100 stocks
var nasdaq100 = []string{
	"AAPL", "MSFT", "AMZN", "NVDA", "GOOGL", "GOOG", "META", "TSLA", "PEP", "AVGO",
	"ADBE", "COST", "CMCSA", "CSCO", "AMD", "NFLX", "TXN", "INTC", "HON", "QCOM",
	"AMGN", "PYPL", "INTU", "SBUX", "AMAT", "MDLZ", "ADI", "REGN", "MU", "BKNG",
	"MRNA", "GILD", "ISRG", "LRCX", "VRTX", "ASML", "KLAC", "ADP", "SNPS", "CDNS",
	"CSX", "PDD", "MAR", "FTNT", "PANW", "EA", "CTAS", "CHTR", "KDP", "EXC",
	"MNST", "WDAY", "ORLY", "XEL", "PCAR", "FAST", "TEAM", "ROST", "ODFL", "BIDU",
	"IDXX", "LCID", "ZS", "DXCM", "BMRN", "SGEN", "MELI", "CRWD", "ABNB", "VRSK",
	"MTCH", "DDOG", "DOCU", "WBD", "BIIB", "OKTA", "CPRT", "NXPI", "PAYC", "ENPH",
	"CEG", "MRVL", "ALGN", "SIRI", "PDDU", "NTES", "CDW", "GFS", "ANSS", "DXC",
	"ZSAN", "HBAN", "TTD", "EBAY", "LULU", "HSIC", "TTWO", "TROW", "FOX", "FOXA",
}

func init() {
	for _, symbol := range nasdaq100 {
		if _, exists := Symbols[symbol]; !exists {
			Symbols[symbol] = symbol
		}
	}
}