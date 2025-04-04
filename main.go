package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"github.com/Scrimzay/stockspider/actor/consumer/finnhub"
	"github.com/Scrimzay/stockspider/event"
	"github.com/Scrimzay/stockspider/symbolArray"
	"strings"
	"time"

	FH "github.com/Finnhub-Stock-API/finnhub-go/v2"
	"github.com/Scrimzay/loglogger"
	"github.com/anthdm/hollywood/actor"
	gui "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/joho/godotenv"
)

var log *logger.Logger

func init() {
	var err error
	log, err = logger.New("mainPackage.txt")
	if err != nil {
		log.Fatalf("Could not start new logger in main: %v", err)
	}

	err = godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
}

type FinnhubClientCFG struct {
	Client *FH.DefaultApiService
}

func NewFinnhubClient(apiKey string) (*FinnhubClientCFG, error) {
    if apiKey == "" {
        return nil, fmt.Errorf("API key cannot be empty")
    }

    cfg := FH.NewConfiguration()
    cfg.AddDefaultHeader("X-Finnhub-Token", os.Getenv("API_KEY"))
    
    return &FinnhubClientCFG{
        Client: FH.NewAPIClient(cfg).DefaultApi,
    }, nil
}

type App struct {
	tradeCh chan event.StockTrade
	engine *actor.Engine

	//prevTrade event.StockTrade
	trades map[string][]event.StockTrade
	quotes map[string]event.Quote
	marketStatus map[string]event.MarketStatus
	recommendationTrends map[string]event.RecommendationTrends
	symbolMetrics map[string]event.SymbolMetric

	panel *Panel
	panel2 *Panel
	panel3 *Panel
	panel4 *Panel
	panel5 *Panel

	availableSymbols map[string]string // display name -> full symbol name
	symbolOrder []string  // maintain stable order of symbols
	selectedSymbol string
	finnhubClient *actor.PID // store reference to finnhub actor
	scrollOffset float32
}

func NewApp(tradeCh chan event.StockTrade, engine *actor.Engine) *App {
	// Create a stable order for symbols
	symbolOrder := make([]string, 0, len(symbolArray.Symbols))
	for _, fullSymbol := range symbolArray.Symbols {
		symbolOrder = append(symbolOrder, fullSymbol)
	}
	// Sort to ensure consistent order
	sort.Strings(symbolOrder)

	app := &App{
		trades: make(map[string][]event.StockTrade),
		tradeCh: tradeCh,
		quotes: make(map[string]event.Quote),
		marketStatus: make(map[string]event.MarketStatus),
		recommendationTrends: make(map[string]event.RecommendationTrends),
		symbolMetrics: make(map[string]event.SymbolMetric),
		engine: engine,
		availableSymbols: symbolArray.Symbols,
		symbolOrder: symbolOrder,
		selectedSymbol: "", // DEFAULT
	}

	// i'm pretty sure this allows clicking the symbols in the panel
	// since without it, clicking isnt possible? oh well
	app.panel = NewPanel(10, 80, 300, 700)
	app.panel.title = "Symbols - Finnhub"

	app.panel2 = NewPanel(900, 300, 300, 300)
	app.panel2.title = "Trades - Finnhub"

	app.panel3 = NewPanel(900, 700, 300, 200)
	app.panel3.title = "Quotes - Finnhub"

	app.panel4 = NewPanel(600, 700, 300, 200)
	app.panel4.title = "Recommendation Trends - Finnhub"

	app.panel5 = NewPanel(600, 400, 300, 200)
	app.panel5.title = "Symbol Metrics - Finnhub"

	// yea see its right here (refer to line 90)
	app.panel.onClick = app.handleSymbolClick
	app.panel2.onClick = app.panel2.HandlePanelDrag
	app.panel3.onClick = app.panel3.HandlePanelDrag
	app.panel4.onClick = app.panel4.HandlePanelDrag
	app.panel5.onClick = app.panel5.HandlePanelDrag

	return app
}

func (app *App) start() {
	// handle trades
	go func() {
		for trade := range app.tradeCh {
			symbol := trade.Pair.Symbol
			log.Printf("Received trade for %s: Price %.2f, Qty %.4f", 
                symbol, trade.Price, trade.Qty)

			if app.trades[symbol] == nil {
				app.trades[symbol] = make([]event.StockTrade, 0)
			}
			app.trades[symbol] = append(app.trades[symbol], trade)
			// Keep only last N trades
			if len(app.trades[symbol]) > 100 {
				app.trades[symbol] = app.trades[symbol][1:]
			}
		}
	}()

	go app.handleQuotes()
	go app.handleMarketStatus()
	go app.handleRecommendationTrends()
	go app.handleSymbolMetric()
}

var color = rl.Green

func (app *App) render() {
	rl.BeginDrawing()
    rl.ClearBackground(rl.Black)

	app.panel.update()
	app.panel.render()
	app.handlePanel1Logic()

	app.panel2.update()
	app.panel2.render()
	// DONT TOUCH, this handles the trades, it must be strings.ToLower
	app.handlePanel2Logic(app.trades[strings.ToLower(app.selectedSymbol)])

	app.panel3.update()
	app.panel3.render()
	if quote, ok := app.quotes[app.selectedSymbol]; ok {
		app.handlePanel3Logic(quote)
	}

	// render market status at the top right
	if status, ok := app.marketStatus["US"]; ok {
		statusStr := fmt.Sprintf("Market: %s", 
        map[bool]string{true: "Open", false: "Closed"}[status.IsOpen])
		rl.DrawText(statusStr, 1000, 7, 20, rl.Green)
		if !status.IsOpen {
			rl.DrawText(statusStr, 1000, 7, 20, rl.Red)
		}

		sessionStr := fmt.Sprintf("Session: %s", status.Session)
		rl.DrawText(sessionStr, 980, 25, 20, rl.White)
	}

	app.panel4.update()
	app.panel4.render()
	if trends, ok := app.recommendationTrends[app.selectedSymbol]; ok {
		app.handlePanel4Logic(trends)
	}

	handleMarketTimer()

	app.panel5.update()
	app.panel5.render()
	selectedMetrics, exists := app.symbolMetrics[app.selectedSymbol]
	if exists {
		app.handlePanel5Logic(selectedMetrics)
	}

	rl.EndDrawing()
}

func (app *App) handlePanel1Logic() {
    mouseX, mouseY := rl.GetMousePosition().X, rl.GetMousePosition().Y

    // Height of the panel title
    titleHeight := float32(25)

    // Adjust scroll offset based on mouse wheel movement
    if rl.GetMouseWheelMove() != 0 {
        app.scrollOffset -= rl.GetMouseWheelMove() * 20 // Adjust scroll speed as needed
    }

    // Calculate the total content height
    totalContentHeight := float32(len(app.symbolOrder)*25) + titleHeight

    // Clamp scroll offset to ensure all symbols are visible
    maxOffset := max(0, int(totalContentHeight) - int(app.panel.height))
    if app.scrollOffset < 0 {
        app.scrollOffset = 0
    } else if app.scrollOffset > float32(maxOffset) {
        app.scrollOffset = float32(maxOffset)
    }

    // Begin scissor mode to clip rendering within the panel
    rl.BeginScissorMode(
        int32(app.panel.position.X),
        int32(app.panel.position.Y+titleHeight), // Start below the title
        int32(app.panel.width),
        int32(app.panel.height-titleHeight),    // Adjust height to exclude the title
    )

    // Render symbols with scrolling
    y := app.panel.position.Y + titleHeight - app.scrollOffset

    for _, fullSymbol := range app.symbolOrder {
        color := rl.White
        if fullSymbol == app.selectedSymbol {
            color = rl.Green
        }

        // Render the symbol if it's within the visible area
        if y >= app.panel.position.Y+titleHeight && y <= app.panel.position.Y+app.panel.height {
            rl.DrawText(fullSymbol, int32(app.panel.position.X+10), int32(y), 17, color)
        }

        // Check for click events on each symbol
        if rl.IsMouseButtonPressed(rl.MouseLeftButton) &&
            mouseX >= app.panel.position.X && mouseX <= app.panel.position.X+app.panel.width &&
            mouseY >= y && mouseY <= y+20 { // Assuming 20 is the height of a symbol row
            app.selectedSymbol = fullSymbol
        }

        y += 25
    }

    // End scissor mode
    rl.EndScissorMode()

	// get trades for selected symbol
	// DON'T FUCK WITH THIS, needs to be strings.ToLower or it borks
	//symbolTrades := app.trades[strings.ToLower(app.selectedSymbol)]

	// This is previous trade renderer, dont need it anymore
	// keeping in this folder incase i find a use for it
	// if len(symbolTrades) < 2 {
	// 	return
	// }
	// lastTrade := symbolTrades[len(symbolTrades)-1]
	// lorem := symbolTrades[len(symbolTrades)-2]
	// if lastTrade.Price < lorem.Price {
	// 	color = rl.Red
	// } else {
	// 	color = rl.Green
	// }
	// lastTradeStr := fmt.Sprint(lastTrade.Price)
	// rl.DrawText(lastTradeStr, 20, 20, 40, rl.Yellow)

	// displays the current ticker for ease of view
	currentTicker := fmt.Sprint(app.selectedSymbol)
	rl.DrawText(currentTicker, 20, 20, 40, rl.Yellow)
}

func (app *App) handlePanel2Logic(symbolTrades []event.StockTrade) {
	if len(symbolTrades) > 0 {
		// Get panel2's position and bounds
		panelX := app.panel2.position.X
		panelY := app.panel2.position.Y
		y := panelY + 30 // start below panel title

		for i := len(symbolTrades) - 1; i >= max(0, len(symbolTrades)-10); i-- {
			trade := symbolTrades[i]
			tradeStr := fmt.Sprintf("%.4f @ %.2f", trade.Qty, trade.Price)
			color = rl.Green
			if !trade.IsBuy {
				color = rl.Red
			}
			rl.DrawText(tradeStr, int32(panelX + 20), int32(y), 20, color)
			y += 25
		}
	}
}

func (app *App) handlePanel3Logic(quote event.Quote) {
    panelX := app.panel3.position.X
    panelY := app.panel3.position.Y
    y := panelY + 30 // Start below panel title

    // Current price with color based on comparison to PrevClose
    currentColor := rl.White
    if quote.Current > quote.PrevClose {
        currentColor = rl.Green
    } else if quote.Current < quote.PrevClose {
        currentColor = rl.Red
    }

    // Draw the current price
    currentStr := fmt.Sprintf("Current: %.2f", quote.Current)
    rl.DrawText(currentStr, int32(panelX+20), int32(y), 24, currentColor)
    y += 35

    // Draw other quote information
    stats := []struct {
        label string
        value float64
        color rl.Color
    }{
        {"High", float64(quote.High), rl.Green},
        {"Low", float64(quote.Low), rl.Red},
        {"Open", float64(quote.Open), rl.White},
        {"Prev Close", float64(quote.PrevClose), rl.White},
    }

    for _, stat := range stats {
        statStr := fmt.Sprintf("%s: %.2f", stat.label, stat.value)
        rl.DrawText(statStr, int32(panelX+20), int32(y), 20, stat.color)
        y += 25
    }

    // Calculate and display price change
    change := quote.Current - quote.PrevClose
    changePercent := (change / quote.PrevClose) * 100
    changeColor := rl.White
    if change > 0 {
        changeColor = rl.Green
    } else if change < 0 {
        changeColor = rl.Red
    }

    changeStr := fmt.Sprintf("Change: %.2f (%.2f%%)", change, changePercent)
    y += 10 // Add space before the change
    rl.DrawText(changeStr, int32(panelX+20), int32(y), 20, changeColor)
}

func (app *App) handlePanel4Logic(trends event.RecommendationTrends) {
    panelX := app.panel4.position.X
    panelY := app.panel4.position.Y
    y := panelY + 30 // Start below panel title

    currentColor := rl.White
    if trends.Buy > trends.Sell {
        currentColor = rl.Green
    } else if trends.Buy < trends.Sell {
        currentColor = rl.Red
    } else if trends.StrongBuy > trends.StrongSell {
        currentColor = rl.Green
    } else if trends.StrongBuy < trends.StrongSell {
        currentColor = rl.Red
    }

    // Render dominant trend summary
    rl.DrawText("Recommendations", int32(panelX+20), int32(y), 24, currentColor)
    y += 35

    // render individual trend stats
    stats := []struct {
        label string
        value int64
        color rl.Color
    }{
        {"Buy", trends.Buy, rl.DarkGreen},
        {"Sell", trends.Sell, rl.Red},
        {"Hold", trends.Hold, rl.White},
        {"StrongBuy", trends.StrongBuy, rl.Green},
        {"StrongSell", trends.StrongSell, rl.Red},
    }

    for _, stat := range stats {
        statStr := fmt.Sprintf("%s: %d", stat.label, stat.value)
        rl.DrawText(statStr, int32(panelX+20), int32(y), 20, stat.color)
        y += 25
    }
}

func (app *App) handlePanel5Logic(metrics event.SymbolMetric) {
	panelX := app.panel5.position.X
    panelY := app.panel5.position.Y
    y := panelY + 50 // Start below panel title

	rl.DrawText("Basic Financials", int32(panelX + 10), int32(panelY + 25), 20, rl.Yellow)

	metricData := []struct{
		Label string
		Value string
	}{
		{"10-Day Avg. Volume", fmt.Sprintf("%.2f", metrics.TenDayAverageTradingVolume)},
        {"52-Week High", fmt.Sprintf("%.2f", metrics.FiftyTwoWeekHigh)},
        {"52-Week Low", fmt.Sprintf("%.2f", metrics.FiftyTwoWeekLow)},
        {"52-Week Price Return", fmt.Sprintf("%.2f%%", metrics.FiftyTwoWeekPriceReturnDaily)},
	}

	for _, data := range metricData {
		rl.DrawText(fmt.Sprintf("%s: %s", data.Label, data.Value), int32(panelX + 10), int32(y), 17, rl.White)
		y += 25
	}
}

func handleMarketTimer() {
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("Failed to load location: %v", err)
	}

	// define market open and close times
	now := time.Now().In(nyLoc)
	marketOpen := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, nyLoc)
	marketClose := time.Date(now.Year(), now.Month(), now.Day(), 16, 0, 0, 0, nyLoc)

	var marketTimer string
	var color rl.Color

	// check if markets open
	if now.Before(marketOpen) {
		marketTimer = "Market is not open yet."
		color = rl.Red
	} else if now.After(marketClose) {
		marketTimer = "Market is now closed."
		color = rl.Red
	} else {
		elapsed := now.Sub(marketOpen)
		totalMarketTime := marketClose.Sub(marketOpen)
		remaining := totalMarketTime - elapsed

		marketTimer = fmt.Sprintf("Time until close: %02d:%02d:%02d",
		int(remaining.Hours()),
		int(remaining.Minutes())%60,
		int(remaining.Seconds())%60,
		)
		color = rl.Green
	}

	rl.DrawText(marketTimer, 440, 20, 35, color)
}

func (app *App) handleSymbolClick(x, y float32) {
	// Adjust for scrolling offset and panel title height
    adjustedY := y + app.scrollOffset - 30 // 30 is the panel title height

    // Calculate which symbol was clicked based on adjusted Y position
    symbolIndex := int(adjustedY / 25)
    
    // Ensure the index is within bounds
    if symbolIndex >= 0 && symbolIndex < len(app.symbolOrder) {
        newSymbol := app.symbolOrder[symbolIndex]
        if newSymbol != app.selectedSymbol {
            log.Printf("Switching from %s to %s", app.selectedSymbol, newSymbol)
            app.selectedSymbol = newSymbol
            app.engine.Send(app.finnhubClient, newSymbol)
            delete(app.trades, strings.ToLower(newSymbol))
        }
    }
}

// i got lazy and annoyed trying to handle quotes so i just used
// the finnhub package cause they already did it so why not
func (app *App) handleQuotes() {
	client, err := NewFinnhubClient(os.Getenv("API_KEY"))
    if err != nil {
        log.Fatalf("Failed to initialize Finnhub client: %v", err)
    }

	for {
        if app.selectedSymbol != "" {
            fullSymbol, exists := app.availableSymbols[app.selectedSymbol]
            if exists {
                // Get quote from Finnhub
                quote, _, err := client.Client.Quote(context.Background()).Symbol(fullSymbol).Execute()

                if err != nil {
                    log.Printf("Error fetching quote for %s: %v", fullSymbol, err)
                    continue
                }

                // Update the quote in app state
                app.quotes[app.selectedSymbol] = event.Quote{
                    Pair: event.Pair{
                        Exchange: "finnhub",
                        Symbol:   app.selectedSymbol,
                    },
                    Current:   quote.GetC(),
                    High:      quote.GetH(),
                    Low:       quote.GetL(),
                    Open:      quote.GetO(),
                    PrevClose: quote.GetPc(),
                }
            } else {
                log.Printf("Symbol %s not found in available symbols map", app.selectedSymbol)
            }
        }
        
        time.Sleep(2 * time.Second)
    }
}

func (app *App) handleMarketStatus() {
	client, err := NewFinnhubClient(os.Getenv("API_KEY"))
    if err != nil {
        log.Fatalf("Failed to initialize Finnhub client: %v", err)
    }

	for {
		res, _, err := client.Client.MarketStatus(context.Background()).Exchange("US").Execute()
		if err != nil {
			log.Printf("Could not get market status from Exchange: %v", err)
		}

		// update market status in app state
		app.marketStatus["US"] = event.MarketStatus{
			Pair: event.Pair{
				Exchange: "finnhub",
				Symbol: "MarketStatus",
			},
			IsOpen: res.GetIsOpen(),
			Session: res.GetSession(),
		}

		time.Sleep(2 * time.Second)
	}
}

func (app *App) handleRecommendationTrends() {
	client, err := NewFinnhubClient(os.Getenv("API_KEY"))
    if err != nil {
        log.Fatalf("Failed to initialize Finnhub client: %v", err)
    }

	for {
		if app.selectedSymbol != "" {
			fullSymbol, exists := app.availableSymbols[app.selectedSymbol]
			if exists {
				trends, _, err := client.Client.RecommendationTrends(context.Background()).Symbol(fullSymbol).Execute()
				if err != nil {
                    log.Printf("Error fetching trends for %s: %v", fullSymbol, err)
                    continue
                }

				for _, trend := range trends {
					app.recommendationTrends[app.selectedSymbol] = event.RecommendationTrends{
						Pair: event.Pair{
							Exchange: "finnhub",
							Symbol: app.selectedSymbol,
						},
						Buy: trend.GetBuy(),
						Sell: trend.GetSell(),
						Hold: trend.GetHold(),
						StrongBuy: trend.GetStrongBuy(),
						StrongSell: trend.GetStrongSell(),
					}
				}
			} else {
				log.Printf("Symbol %s not found in available symbols map", app.selectedSymbol)
			}
		}

		time.Sleep(2 * time.Second)
	}
}

func (app *App) handleSymbolMetric() {
	client, err := NewFinnhubClient(os.Getenv("API_KEY"))
	if err != nil {
		log.Print("Error connecting to finnhub client in symbol metric: %v", err)
		return
	}

	for {
		if app.selectedSymbol != "" {
			fullSymbol, exists := app.availableSymbols[app.selectedSymbol]
			if exists {
				res, _, err := client.Client.CompanyBasicFinancials(context.Background()).Symbol(fullSymbol).Metric("all").Execute()
				if err != nil {
					log.Print("Error getting company basic financials: %v", err)
					return
				}

				// parse and store the metrics
				if res.Metric != nil {
					metricsMap := *res.Metric // deref the pointer to access the map

					// parse and store metrics
					metrics := event.SymbolMetric{
						Pair: event.Pair{
							Exchange: "finnhub",
							Symbol: app.selectedSymbol,
						},
						TenDayAverageTradingVolume: getFloatFromMap(metricsMap, "10DayAverageTradingVolume"),
                        FiftyTwoWeekHigh:           getFloatFromMap(metricsMap, "52WeekHigh"),
                        FiftyTwoWeekLow:            getFloatFromMap(metricsMap, "52WeekLow"),
                        FiftyTwoWeekPriceReturnDaily: getFloatFromMap(metricsMap, "52WeekPriceReturnDaily"),
					}

					// store metrics in the apps map
					app.symbolMetrics[app.selectedSymbol] = metrics
					//log.Printf("Updated metrics for %s: %+v", app.selectedSymbol, metrics)
				}
			} else {
				log.Printf("Symbol %s not found in symbols map", app.selectedSymbol)
			}
		}
		time.Sleep(2 * time.Second)
	}
}

// helper func for symbolMetrics
func getFloatFromMap(metrics map[string]interface{}, key string) float64 {
	if val, ok := metrics[key]; ok {
		if floatVal, valid := val.(float64); valid {
			return floatVal
		}
	}

	return 0
}

// helper func (idk wtf it does)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type Panel struct {
    position rl.Vector2
    width, height float32
    title string
	isDragging bool // tracks panel being dragged
	dragOffset rl.Vector2 // keeps the offset of the mouse relative to the panels top-left corner
    onClick func(x, y float32) // add click handler
}

func NewPanel(x, y, width, height float32) *Panel {
    return &Panel{
        position: rl.NewVector2(x, y),
        width: width,
        height: height,
		isDragging: false,
		dragOffset: rl.NewVector2(0, 0),
        onClick: nil, // will be set later
    }
}

// generic panel move func since update was being annoying
func (p *Panel) HandlePanelDrag(x, y float32) {
    mouseX := float32(rl.GetMouseX())
    mouseY := float32(rl.GetMouseY())

    if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
        // Check if click is within panel bounds
        if mouseX >= p.position.X && mouseX <= p.position.X+p.width &&
            mouseY >= p.position.Y && mouseY <= p.position.Y+p.height {
            p.isDragging = true
            p.dragOffset = rl.NewVector2(mouseX-p.position.X, mouseY-p.position.Y)
        }
    }

    if p.isDragging {
        p.position.X = mouseX - p.dragOffset.X
        p.position.Y = mouseY - p.dragOffset.Y
    }

    if rl.IsMouseButtonReleased(rl.MouseLeftButton) {
        p.isDragging = false
    }

    // Ensure the panel stays within the window bounds
    if p.position.X < 0 {
        p.position.X = 0
    }
    if p.position.Y < 0 {
        p.position.Y = 0
    }
    if p.position.X+p.width > float32(rl.GetScreenWidth()) {
        p.position.X = float32(rl.GetScreenWidth()) - p.width
    }
    if p.position.Y+p.height > float32(rl.GetScreenHeight()) {
        p.position.Y = float32(rl.GetScreenHeight()) - p.height
    }
}

func (p *Panel) update() {
	mouseX := float32(rl.GetMouseX())
	mouseY := float32(rl.GetMouseY())

    if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
        // Check if click is within panel bounds
        if mouseX >= p.position.X && mouseX <= p.position.X+p.width &&
           mouseY >= p.position.Y && mouseY <= p.position.Y+p.height {
			p.onClick(mouseX - p.position.X, mouseY - p.position.Y)
            p.isDragging = true
            p.dragOffset = rl.NewVector2(mouseX-p.position.X, mouseY-p.position.Y)
        }
    }

	if p.isDragging {
		p.position.X = mouseX - p.dragOffset.X
		p.position.Y = mouseY - p.dragOffset.Y
	}

	if rl.IsMouseButtonReleased(rl.MouseLeftButton) {
		p.isDragging = false
	}

	if p.position.X < 0 {
		p.position.X = 0
	}
	if p.position.Y < 0 {
		p.position.Y = 0
	}
	if p.position.X+p.width > float32(rl.GetScreenWidth()) {
		p.position.X = float32(rl.GetScreenWidth()) - p.width
	}
	if p.position.Y+p.height > float32(rl.GetScreenHeight()) {
		p.position.Y = float32(rl.GetScreenHeight()) - p.height
	}
}

func (p *Panel) getDrawPos(x, y float32) rl.Vector2 {
    return rl.NewVector2(p.position.X+x, p.position.Y+24+y+p.height)
}

func (p *Panel) render() {
    // Draw solid black background
    rl.DrawRectangle(int32(p.position.X), int32(p.position.Y), int32(p.width), int32(p.height), rl.Black)

    // Render the panel with its title
    gui.Panel(rl.NewRectangle(p.position.X, p.position.Y, p.width, p.height), p.title)

    // Optional: Add a visual marker (e.g., red dot) for debugging
    pos := p.getDrawPos(10, 10)
    rl.DrawCircle(int32(pos.X), int32(pos.Y), 2, rl.Red)
}

func main() {
    e, err := actor.NewEngine(actor.NewEngineConfig())
    if err != nil {
        log.Fatal(err)
    }
    tradeCh := make(chan event.StockTrade)
    app := NewApp(tradeCh, e)
    go app.start()

    // store the pid when spawning
    finnhubPID := e.Spawn(finnhub.New(tradeCh), "finnhub")
    app.finnhubClient = finnhubPID

    rl.InitWindow(1200, 800, "Stock Spider")
    defer rl.CloseWindow()
    rl.SetTargetFPS(60)
    gui.SetStyle(0, gui.BACKGROUND_COLOR, 0x000000ff)

	// panel (panel1) is the one that contains the clickable symbols, leave it
    panel := NewPanel(10, 80, 300, 700)
    panel.title = "Quick Pick Symbols - Finnhub"
    panel.onClick = app.handleSymbolClick

    for !rl.WindowShouldClose() {
        panel.update()
        panel.render()
        app.render()
    }
	// close trade chan for cleanup, hope it does at least
    close(tradeCh)
}