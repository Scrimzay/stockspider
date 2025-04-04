package finnhub

import (
	"errors"
	"fmt"
	"net"
	"os"

	//"os"
	"github.com/Scrimzay/stockspider/actor/symbol"
	"github.com/Scrimzay/stockspider/event"
	"strings"

	"github.com/Scrimzay/loglogger"
	"github.com/anthdm/hollywood/actor"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/valyala/fastjson"
)

const wsEndpoint = "wss://ws.finnhub.io?token="

// this is intertwined into the code and makes it work so i wont
// touch it and neither should you, just leave it empty
// why empty? if it has a symbol, it spams the symbol nonstop
// symbols are handled in symbolArray and in the main file
var symbols = []string{
	"",
}
var log *logger.Logger

func init() {
	var err error
	log, err = logger.New("finnhubPackage.txt")
	if err != nil {
		log.Fatalf("Error starting logger in Finnhub package: %v", err)
	}

	err = godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
}

type FinnhubClient struct {
	ws *websocket.Conn
	symbols map[string]*actor.PID
	c *actor.Context
	tradeCh chan event.StockTrade
	currentSymbol string
}

func (f *FinnhubClient) Receive(c *actor.Context) {
	switch msg :=  c.Message().(type) {
	case actor.Started:
		f.start(c)
		f.c = c
	case string: // handle symbol change messages
		f.ChangeSymbol(msg)
	}
}

func New(tradeCh chan event.StockTrade) actor.Producer {
	return func() actor.Receiver {
		return &FinnhubClient{
			symbols: make(map[string]*actor.PID),
			tradeCh: tradeCh,
		}
	}
}

func (f *FinnhubClient) start(c *actor.Context) {
	// init symbol actors as children
	for _, sym := range symbols {
		pair := event.Pair{
			Exchange: "finnhub",
			Symbol: strings.ToLower(sym),
		}
		pid := c.SpawnChild(symbol.New(pair), "symbol")
		f.symbols[pair.Symbol] = pid
	}
	ws, _, err := websocket.DefaultDialer.Dial(createWsEndpoint(), nil)
	if err != nil {
		log.Fatal(err)
	}
	f.ws = ws

    // Make sure you're subscribing to trades
    for _, sym := range symbols {
		msg := struct {
			Type string `json:"type"`
			Symbol string `json:"symbol"`
		}{
			Type: "subscribe",
			Symbol: sym,
		}
		//log.Printf("Subscribing to symbol: %s", msg.Symbol)
		if err := f.ws.WriteJSON(msg); err != nil {
			log.Printf("Subscribe error for %s: %v", sym, err)
		}
	}
    log.Printf("Subscribed to symbol %s for trades", f.currentSymbol)

    go f.wsLoop()
}

func (f *FinnhubClient) wsLoop() {
	var lastPrices = make(map[string]float64)

	for {
		_, msg, err := f.ws.ReadMessage()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}
			log.Printf("Error reading from ws connection: %v", err)
			continue
		}

		parser := fastjson.Parser{}
		v, err := parser.ParseBytes(msg)
		if err != nil {
			log.Printf("Failed to parse msg: %v", err)
			continue
		}

		// Handle different types of messages
		msgType := string(v.GetStringBytes("type"))
		log.Printf("Message type: %s", msgType)
		switch msgType {
		case "trade":
			f.handleTrades(v.Get("data"), lastPrices)
		case "ping":
			// response to keep-alive
			f.ws.WriteJSON(struct {
				Type string `json:"type"`
			}{
				Type: "pong",
			})
		default:
			log.Printf("Unknown message type: %s", msgType)
		}
	}
}

func (f *FinnhubClient) ChangeSymbol(newSymbol string) {
	log.Printf("Changing symbol from %s to %s", f.currentSymbol, newSymbol)

	// unsub from current symbol
	if f.currentSymbol != "" {
		unsubMsg := struct {
			Type string `json:"type"`
			Symbol string `json:"symbol"`
		}{
			Type: "unsubscribe",
			Symbol: f.currentSymbol,
		}
		if err := f.ws.WriteJSON(unsubMsg); err != nil {
            log.Printf("Error unsubscribing: %v", err)
        }
	}

	// Subscribe to trades
    subTradeMsg := struct {
        Type string `json:"type"`
        Symbol string `json:"symbol"`
    }{
        Type: "subscribe",
        Symbol: newSymbol,
    }
    if err := f.ws.WriteJSON(subTradeMsg); err != nil {
        log.Printf("Error subscribing: %v", err)
		return
    }

    f.currentSymbol = newSymbol
}

func (f *FinnhubClient) handleTrades(data *fastjson.Value, lastPrices map[string]float64) {
	if data == nil {
		return
	}

	// get the array of trades
	trades, err := data.Array()
	if err != nil {
		log.Printf("Error getting trades array: %v", err)
		return
	}

	// iterate over each trade in the array
	for _, trade := range trades {
        // Get the values using the correct Finnhub field names
        symbolRaw := string(trade.GetStringBytes("s"))
        
        // These might be different field names in Finnhub's API
        priceVal := trade.Get("p")
        qtyVal := trade.Get("v")
        
        if priceVal == nil || qtyVal == nil {
            log.Printf("Missing price or quantity data in trade: %s", trade.String())
            continue
        }

        price := priceVal.GetFloat64()
        qty := qtyVal.GetFloat64()

        symbol := strings.ToLower(symbolRaw)

		// determine if buy or sell based on price movement
		isBuy := true
		if lastPrice, ok := lastPrices[symbol]; ok {
			isBuy = price >= lastPrice
		}
		lastPrices[symbol] = price

		stockTrade := event.StockTrade{
			Price: price,
			Qty: qty,
			IsBuy: isBuy, // gonna need to implement my own logic here
			Unix: trade.GetInt64("t"),
			Pair: event.Pair{
				Exchange: "finnhub",
				Symbol: symbol,
			},
		}

		f.tradeCh <- stockTrade

		// forward to symbol actor if it exists
		if symbolPID, ok := f.symbols[symbol]; ok {
			f.c.Send(symbolPID, stockTrade)

			// also create and send a stat event
			// Note: finnhub doesnt provide funding rate, so we set to 0
			stat := event.Stat{
				Pair: stockTrade.Pair,
				MarkPrice: price,
				Funding: 0,
				Unix: stockTrade.Unix,
			}
			f.c.Send(symbolPID, stat)
		}
	}
}

func createWsEndpoint() string {
	apiKey := os.Getenv("API_KEY")
	return fmt.Sprintf("%s%s", wsEndpoint, apiKey)
}