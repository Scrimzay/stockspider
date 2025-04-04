package event

type Pair struct{
	Exchange string
	Symbol string
}

type MarketStatus struct {
	Pair Pair
	IsOpen bool
	Session string
}

type RecommendationTrends struct {
	Pair Pair
	Buy int64
	Hold int64
	Sell int64
	StrongBuy int64
	StrongSell int64
}

type StockTrade struct {
	Pair Pair
	Price float64
	Qty float64
	IsBuy bool
	Unix int64
}

type Quote struct {
	Pair Pair
	Current float32 // c
	High float32 // h
	Low float32 // l
	Open float32 // o
	PrevClose float32 // pc
	Unix int64 // t
}

type SymbolMetric struct {
	Pair Pair
	TenDayAverageTradingVolume float64
	FiftyTwoWeekHigh float64
	FiftyTwoWeekLow float64
	FiftyTwoWeekPriceReturnDaily float64
}

type Stat struct {
	Pair Pair
	MarkPrice float64
	Funding float64
	Unix int64
}