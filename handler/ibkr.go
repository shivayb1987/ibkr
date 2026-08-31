package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"ibkr/model"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
)

type IBKRService interface {
	GetAll(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error)
	Order(ctx context.Context, orders []model.Order) (model.OrderResponse, error)
	Update(ctx context.Context, order model.Order) (model.OrderResponse, error)
	Search(ctx context.Context, query string) ([]model.Symbol, error)
	Security(ctx context.Context, contractIDs []string) ([]model.Security, error)
	Cancel(ctx context.Context, symbol string, orderID string) ([]model.CancelOrder, error)
	Reply(replyID string) (model.OrderResponse, error)
	ListOrders(ctx context.Context) ([]model.ListOrder, []string, error)
	Init()
}

type AlertService interface {
	Create(ctx context.Context, alert model.Alert) (interface{}, error)
	GetAll(ctx context.Context) ([]model.IBKRAlert, error)
}

type PositionService interface {
	GetAll(ctx context.Context) ([]model.Position, []string, float64, error)
	Summary(ctx context.Context) (model.Summary, error)
}

type YahooService interface {
	Similar(ctx context.Context, symbol string) (model.SimilarStockResponse, error)
	GetStock(symbol string, period string, duration string) (model.Result2, model.Meta)
	GetActiveStocks(start, count int, scrID string, region string) ([]model.ActiveQuote, error)
	GetMovingAverage(closes []float64, days int) float64
	IsRisingMA(closes []float64, days int) bool
	ATR(quote model.Quote, days int) float64
	Volatility(symbol string) float64
	CrossIndex(symbol string) (string, float64, int)
}

type FileService interface {
	Write(fileName, content string)
	WriteBytes(fileName string, content []byte)
}

type Ibkr struct {
	ibkrService     IBKRService
	alertService    AlertService
	positionService PositionService
	fileService     FileService
	yahooService    YahooService
	ibdService      IBDService
	polygonService  PolygonService
	nasdaqService   NasdaqService
	allSymbols      []string
	totalEquity     float64
}

func NewIbkr(service IBKRService, alertService AlertService, positionService PositionService, fileService FileService, yahooService YahooService, polygonService PolygonService, nasdaqService NasdaqService, ibdService IBDService, allSymbols []string, totalEquity float64) Ibkr {
	return Ibkr{
		ibkrService:     service,
		alertService:    alertService,
		positionService: positionService,
		fileService:     fileService,
		yahooService:    yahooService,
		polygonService:  polygonService,
		nasdaqService:   nasdaqService,
		allSymbols:      allSymbols,
		totalEquity:     totalEquity,
		ibdService:      ibdService,
	}
}

func (h Ibkr) Positions(w http.ResponseWriter, r *http.Request) {
	//h.ibkrService.Init()
	positions, symbols, totalCost, err := h.positionService.GetAll(r.Context())
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	var totalPL float64
	for _, pos := range positions {
		totalPL += pos.UnrealizedPnl
	}

	positionsWithoutOrders, orphanedOrders, err := h.getPositionsWithoutAttachedOrders(r.Context(), symbols)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	totalPotentialLoss := 0.0
	cadusd := 1.0
	loss := r.URL.Query().Get("loss")
	if loss == "true" {
		cadusd = h.getFX("CADUSD")
		openOrders, err := h.ibkrService.GetAll(r.Context(), map[string]interface{}{"tickers": symbols})
		if err != nil {
			Errors(w, r, 500, err.Error())
			return
		}

		for _, openOrder := range openOrders.Orders {
			potentialLoss, err := strconv.ParseFloat(openOrder.PotentialLoss, 64)
			if err != nil {
				continue
			}
			if openOrder.CashCcy == "CAD" {
				totalPotentialLoss += potentialLoss * cadusd
			} else {
				totalPotentialLoss += potentialLoss
			}
		}
	}

	newTarget := r.URL.Query().Get("target")
	symbol := r.URL.Query().Get("symbol")
	if newTarget != "" && symbol != "" {
		newTargetValue, _ := strconv.ParseFloat(newTarget, 64)
		symbolPos := model.Position{}
		for _, pos := range positions {
			if pos.ContractDesc == symbol {
				symbolPos = pos

				symbolPos.UnrealizedPnl = roundFloat(pos.Position * (newTargetValue - pos.AvgCost))
				break
			}
		}
		positions = []model.Position{symbolPos}
	}

	longs, shorts := make([]string, 0), make([]string, 0)
	for _, pos := range positions {
		if pos.Position < 0 {
			shorts = append(shorts, pos.ContractDesc)
		} else if pos.Position > 0 {
			longs = append(longs, pos.ContractDesc)
		}
	}

	sort.Strings(symbols)
	results := map[string]interface{}{
		"count":              len(symbols),
		"all":                strings.Join(symbols, ","),
		"allCost":            totalCost,
		"positions":          positions,
		"totalUnrealisedPnL": fmt.Sprintf(".2%f", totalPL*100),
		"totalPotentialLoss": fmt.Sprintf("%0.2f", totalPotentialLoss),
		"zzLongs":            strings.Join(longs, ","),
		"zzShorts":           strings.Join(shorts, ","),
	}

	if len(positionsWithoutOrders) > 0 {
		results["zzNoAttachedOrders"] = positionsWithoutOrders
	}
	if len(orphanedOrders) > 0 {
		results["zzOrphanedOrders"] = orphanedOrders
	}

	Success(w, r, results)
	posBytes, _ := json.Marshal(positions)

	if symbol == "" {
		h.fileService.WriteBytes(fmt.Sprintf("Positions_%s", time.Now().Format("2006-01-02")), posBytes)
		h.fileService.Write("Positions", strings.Replace(strings.Join(symbols, ","), "TXG", "TSX:TXG", 1))
	}
}
func (h Ibkr) PnL(w http.ResponseWriter, r *http.Request) {
	//h.ibkrService.Init()
	tickers := r.URL.Query()["ticker"]
	positions, _, _, err := h.positionService.GetAll(r.Context())
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	tickersCheck := make(map[string]bool)
	for _, ticker := range tickers {
		tickersCheck[ticker] = true
	}

	results := make(map[string]string)
	for _, pos := range positions {
		if _, ok := tickersCheck[pos.ContractDesc]; ok {
			if pos.MktValue != 0 {
				results[pos.ContractDesc] = fmt.Sprintf("%0.2f (%0.2f)", roundFloat(100*pos.UnrealizedPnl/math.Abs(pos.MktValue)), pos.UnrealizedPnl)
			}
		}
	}

	Success(w, r, results)
}

func (h Ibkr) getFX(currency string) float64 {
	price := 1.0
	result, _ := h.yahooService.GetStock(fmt.Sprintf("%s=X", currency), "1d", "1w")
	quote := result.Indicators.Quote
	if len(quote) > 0 && len(quote[0].Close) > 0 {
		closes := quote[0].Close
		price = closes[len(closes)-1]
	}
	return price
}

func (h Ibkr) getPositionsWithoutAttachedOrders(ctx context.Context, positions []string) ([]string, []string, error) {
	attachedOrders, err := h.ibkrService.GetAll(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	positionsBySymbol := make(map[string]bool)
	for _, symbol := range positions {
		positionsBySymbol[symbol] = true
	}

	var positionsWithoutOrders []string
	ordersBySymbol := make(map[string][]model.OrderDetails)
	for _, attachedOrder := range attachedOrders.Orders {
		if attachedOrder.Status == "Cancelled" || attachedOrder.OrderDesc == "Pending Cancel" {
			continue
		}
		if _, ok := ordersBySymbol[attachedOrder.Ticker]; ok {
			ordersBySymbol[attachedOrder.Ticker] = append(ordersBySymbol[attachedOrder.Ticker], attachedOrder)
		} else {
			ordersBySymbol[attachedOrder.Ticker] = []model.OrderDetails{attachedOrder}
		}
	}

	var orphanedOrders []string
	for symbol, orders := range ordersBySymbol {
		if len(orders) > 1 {
			noAttachedOrderCount := 0
			for _, attachedOrder := range orders {
				if positionsBySymbol[attachedOrder.Ticker] {
					pattern := `Sell \d+ [A-Z]+ Stop`
					regex := regexp.MustCompile(pattern)

					match := regex.FindStringSubmatch(attachedOrder.OrderDesc)
					if len(match) > 0 {
						noAttachedOrderCount++
					}
					pattern = `Buy \d+ [A-Z]+ Stop`
					regex = regexp.MustCompile(pattern)

					match = regex.FindStringSubmatch(attachedOrder.OrderDesc)
					if len(match) > 0 {
						noAttachedOrderCount++
					}

					pattern = `Sell \d+ [A-Z]+ Limit`
					regex = regexp.MustCompile(pattern)

					match = regex.FindStringSubmatch(attachedOrder.OrderDesc)
					if len(match) > 0 {
						noAttachedOrderCount++
					}
					pattern = `Buy \d+ [A-Z]+ Limit`
					regex = regexp.MustCompile(pattern)

					match = regex.FindStringSubmatch(attachedOrder.OrderDesc)
					if len(match) > 0 {
						noAttachedOrderCount++
					}
				}
			}
			if positionsBySymbol[symbol] && noAttachedOrderCount == 0 {
				positionsWithoutOrders = append(positionsWithoutOrders, symbol)
			}
			continue
		}

		pattern := `Sell \d+`
		regex := regexp.MustCompile(pattern)

		match := regex.FindStringSubmatch(orders[0].OrderDesc)
		if len(match) > 0 {
			orphanedOrders = append(orphanedOrders, symbol)
		}
	}

	for _, position := range positions {
		if _, ok := ordersBySymbol[position]; !ok {
			positionsWithoutOrders = append(positionsWithoutOrders, position)
		}
	}

	return positionsWithoutOrders, orphanedOrders, nil
}

func extractNumber(numStr string) float64 {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(strings.Replace(numStr, ",", "", -1), -1)
	if len(matches) > 0 {
		value, _ := strconv.ParseFloat(matches[0], 64)
		return value
	}

	return 1.0
}

func (h Ibkr) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.positionService.Summary(r.Context())
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	Success(w, r, summary)
}

func (h Ibkr) List(w http.ResponseWriter, r *http.Request) {
	//h.ibkrService.Init()
	orders, symbols, totalOrderAmount, pendingOrders, pendingTickers, workingOrders, potentialLoss, potentialProfit, err := h.getOrders(r.Context())
	if err != nil {
		Errors(w, r, 500, err.Error())

		return
	}

	totalEquity := h.totalEquity
	if h.totalEquity > 0 {
		summary, err := h.positionService.Summary(r.Context())
		if err != nil {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

			return
		}

		//sgdusd := h.getFX("SGDUSD")
		totalEquity = summary.EquityWithLoanValue.Amount
	}
	coreEquity := totalEquity - potentialLoss

	Success(w, r, map[string]interface{}{
		"count":                len(symbols),
		"list":                 strings.Join(symbols, ","),
		"orders":               orders,
		"workingOrders":        workingOrders,
		"workingOrdersCount":   len(workingOrders),
		"newOrders":            pendingOrders,
		"newOrdersList":        pendingTickers,
		"totalOrderAmount":     totalOrderAmount,
		"totalPotentialLoss":   potentialLoss,
		"totalPotentialProfit": potentialProfit,
		"zzCoreEquity":         roundFloat(coreEquity),
	})
}

func (h Ibkr) getOrders(ctx context.Context) ([]model.ListOrder, []string, float64, []string, []string, []string, float64, float64, error) {
	var wg sync.WaitGroup
	workingOrdersByStock := make(map[string]map[string]float64)
	liveTickersReference := make(map[string]bool)

	var orders []model.ListOrder
	var symbols []string
	var listErr error
	var liveTickers []string

	wg.Add(2)
	go func() {
		defer wg.Done()
		orders, symbols, listErr = h.ibkrService.ListOrders(ctx)
	}()
	go func() {
		defer wg.Done()
		_, liveTickers, _, _ = h.positionService.GetAll(ctx)
	}()
	wg.Wait()

	if listErr != nil {
		return nil, nil, 0, nil, nil, nil, 0, 0, listErr
	}
	for _, ticker := range liveTickers {
		liveTickersReference[ticker] = true
	}

	var wg2 sync.WaitGroup
	var m sync.Mutex
	closesByTicker := make(map[string]float64)

	wg2.Add(len(orders))
	for _, order := range orders {
		go func(order model.ListOrder) {
			defer wg2.Done()
			result, meta := h.yahooService.GetStock(order.Ticker, "1d", "5d")
			if len(result.Indicators.Quote) == 0 || len(result.Indicators.Quote[0].Close) == 0 {
				result, meta = h.yahooService.GetStock(fmt.Sprintf("%s.TO", order.Ticker), "1d", "5d")
			}
			if len(result.Indicators.Quote) == 0 || len(result.Indicators.Quote[0].Close) == 0 {
				return
			}
			close := lastValidClose(result, meta)
			if close <= 0 {
				return
			}
			m.Lock()
			closesByTicker[order.Ticker] = close
			m.Unlock()
		}(order)
	}
	wg2.Wait()

	if len(symbols) == 0 {
		return []model.ListOrder{}, []string{}, 0, []string{}, []string{}, []string{}, 0, 0, nil
	}

	type pendingEntry struct {
		entry    float64
		position float64
		cashCcy  string
	}
	// qty → price, so a ticker with multiple brackets keeps the matching exit legs
	stopsByTickerQty := make(map[string]map[float64]float64)
	limitsByTickerQty := make(map[string]map[float64]float64)
	pendingByTicker := make(map[string]pendingEntry)
	var totalOrderAmount float64

	for _, order := range orders {
		if order.Status == "Filled" || order.Status == "Cancelled" || order.Status == "Inactive" {
			continue
		}
		close := closesByTicker[order.Ticker]
		if close <= 0 {
			continue
		}
		if _, ok := workingOrdersByStock[order.Ticker]; !ok {
			workingOrdersByStock[order.Ticker] = map[string]float64{"Close": close}
		} else {
			workingOrdersByStock[order.Ticker]["Close"] = close
		}

		switch order.OrderType {
		case "Stop":
			parts := strings.Split(order.OrderDesc, " ")
			if len(parts) <= 4 {
				continue
			}
			stopLoss, _ := strconv.ParseFloat(strings.Replace(parts[4], ",", "", 1), 64)
			position, _ := strconv.ParseFloat(strings.Replace(parts[1], ",", "", 1), 64)
			if stopLoss <= 0 || position <= 0 {
				continue
			}
			if stopsByTickerQty[order.Ticker] == nil {
				stopsByTickerQty[order.Ticker] = make(map[float64]float64)
			}
			stopsByTickerQty[order.Ticker][position] = stopLoss
			// Live exits: keep latest stop on the ticker for workingOrders summary
			if _, isLive := liveTickersReference[order.Ticker]; isLive {
				workingOrdersByStock[order.Ticker]["StopLoss"] = stopLoss
				workingOrdersByStock[order.Ticker]["Position"] = position
				workingOrdersByStock[order.Ticker]["SL"] = (close / stopLoss) - 1
				workingOrdersByStock[order.Ticker]["PotentialLoss"] = math.Abs(close-stopLoss) * position
			}
		case "Limit":
			target := order.Price
			parts := strings.Split(order.OrderDesc, " ")
			position := 0.0
			if len(parts) > 1 {
				position, _ = strconv.ParseFloat(strings.Replace(parts[1], ",", "", 1), 64)
			}
			if target <= 0 || position <= 0 {
				continue
			}
			if limitsByTickerQty[order.Ticker] == nil {
				limitsByTickerQty[order.Ticker] = make(map[float64]float64)
			}
			limitsByTickerQty[order.Ticker][position] = target
			if _, isLive := liveTickersReference[order.Ticker]; isLive {
				workingOrdersByStock[order.Ticker]["Target"] = (target / close) - 1
				if workingOrdersByStock[order.Ticker]["Position"] <= 0 {
					workingOrdersByStock[order.Ticker]["Position"] = position
				}
				cadusd := h.getFX("CADUSD")
				profit := math.Abs(target-close) * position
				if order.CashCcy == "CAD" {
					profit *= cadusd
				}
				workingOrdersByStock[order.Ticker]["PotentialProfit"] = profit
			}
		case "Stop Limit":
			// Entry parent: "Buy 246 TEVA Stop 37.45 LMT 37.56, GTC" — LMT is entry, not target.
			parts := strings.Split(order.OrderDesc, " ")
			position := 0.0
			entry := order.Price
			if len(parts) > 6 {
				position, _ = strconv.ParseFloat(strings.Replace(parts[1], ",", "", 1), 64)
				entry, _ = strconv.ParseFloat(strings.Replace(parts[6], ",", "", 1), 64)
			}
			if entry <= 0 {
				entry = order.Price
			}
			if position <= 0 || entry <= 0 {
				continue
			}
			isPendingEntry := strings.Contains(order.OrderDesc, "Stop") && strings.Contains(order.OrderDesc, "LMT") &&
				(strings.Contains(order.OrderDesc, "Buy") || strings.Contains(order.OrderDesc, "Sell"))
			if !isPendingEntry {
				continue
			}
			pendingByTicker[order.Ticker] = pendingEntry{entry: entry, position: position, cashCcy: order.CashCcy}
			if strings.Contains(order.OrderDesc, "Buy") {
				totalOrderAmount += entry * position
			} else {
				totalOrderAmount -= entry * position
			}
		}
	}

	workingOrders := make([]string, 0, len(symbols))
	potentialLoss := 0.0
	potentialProfit := 0.0
	for ticker, prices := range workingOrdersByStock {
		if _, ok := liveTickersReference[ticker]; !ok {
			continue
		}
		workingOrders = append(workingOrders, formatOrderSummary(ticker, prices))
		potentialLoss += prices["PotentialLoss"]
		potentialProfit += prices["PotentialProfit"]
	}

	newOrders := make([]string, 0, len(pendingByTicker))
	newOrdersList := make([]string, 0, len(pendingByTicker))
	for ticker, ent := range pendingByTicker {
		newOrdersList = append(newOrdersList, ticker)
		stopLoss := stopsByTickerQty[ticker][ent.position]
		target := limitsByTickerQty[ticker][ent.position]
		ref := ent.entry
		prices := map[string]float64{
			"Close":    ref,
			"Position": ent.position,
			"StopLoss": stopLoss,
			"Target":   0,
		}
		if stopLoss > 0 && ref > 0 {
			prices["SL"] = (ref / stopLoss) - 1
			prices["PotentialLoss"] = math.Abs(ref-stopLoss) * ent.position
			if ent.cashCcy == "CAD" {
				prices["PotentialLoss"] *= h.getFX("CADUSD")
			}
		}
		if target > 0 && ref > 0 {
			prices["Target"] = (target / ref) - 1
			profit := math.Abs(target-ref) * ent.position
			if ent.cashCcy == "CAD" {
				profit *= h.getFX("CADUSD")
			}
			prices["PotentialProfit"] = profit
		}
		newOrders = append(newOrders, formatOrderSummary(ticker, prices))
	}

	sort.Slice(workingOrders, func(i, j int) bool {
		return math.Abs(extractTargetPercentage(workingOrders[i])) < math.Abs(extractTargetPercentage(workingOrders[j]))
	})
	sort.Slice(newOrders, func(i, j int) bool {
		return math.Abs(extractTargetPercentage(newOrders[i])) < math.Abs(extractTargetPercentage(newOrders[j]))
	})
	sort.Strings(newOrdersList)
	return orders, symbols, totalOrderAmount, newOrders, newOrdersList, workingOrders, potentialLoss, potentialProfit, nil
}

func formatOrderSummary(ticker string, prices map[string]float64) string {
	return fmt.Sprintf("%s SL %.2f %.2f%% Target %.2f %.2f%%, (%0.2f/%0.2f)",
		ticker, prices["StopLoss"], 100*prices["SL"],
		(1+prices["Target"])*prices["Close"], 100*prices["Target"],
		prices["PotentialLoss"], prices["Position"]*prices["Close"])
}

// lastValidClose prefers the latest non-zero Yahoo close. When the last bar is
// null (common API issue), it uses the close at regularMarketTime, then
// regularMarketPrice, so portfolio risk is not inflated as |0-stop|*qty.
func lastValidClose(result model.Result2, meta model.Meta) float64 {
	var closes []float64
	if len(result.Indicators.Quote) > 0 {
		closes = result.Indicators.Quote[0].Close
	}
	if n := len(closes); n > 0 && closes[n-1] > 0 {
		return closes[n-1]
	}
	if meta.RegularMarketTime > 0 && len(result.Timestamps) == len(closes) {
		for i, ts := range result.Timestamps {
			if ts == meta.RegularMarketTime && closes[i] > 0 {
				return closes[i]
			}
		}
	}
	for i := len(closes) - 1; i >= 0; i-- {
		if closes[i] > 0 {
			return closes[i]
		}
	}
	return meta.RegularMarketPrice
}

func (h Ibkr) Similar(w http.ResponseWriter, r *http.Request) {
	ticker := strings.TrimSpace(chi.URLParam(r, "ticker"))
	if ticker == "" {
		Errors(w, r, http.StatusInternalServerError, "invalid ticker")

		return
	}

	nasdaqInfo, err := h.nasdaqService.Summary(ticker, "stocks")
	if err != nil {
	}
	activity, err := h.nasdaqService.InsiderActivity(ticker)
	if err != nil {
		fmt.Errorf("cannot get insider activity for %s: %w", ticker, err)
	}

	insiderTrade := ""
	for _, trade := range activity.NumberOfSharesTraded.Rows {
		if trade.InsiderTrade == "Net Activity" {
			insiderTrade = fmt.Sprintf("%s %s", trade.Months3, trade.Months12)
			break
		}
	}

	//tickerDetails, err := h.polygonService.Summary(ticker)
	//if err != nil {
	//}

	similarStocks, err := h.yahooService.Similar(r.Context(), ticker)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	others := make([]string, 0)
	movingAverageByStock := make([]model.Performance, 0)
	count := 0.0
	totalRelativeMACount := 0
	for _, result := range similarStocks.Finance.Results {
		result.RecommendedSymbols = append(result.RecommendedSymbols, model.RecommendedSymbol{Symbol: ticker})

		etfs := []string{"XLV", "XLK", "XLI", "XLY", "XLU", "XLF", "XLB", "XLE", "XLC", "XLP", "XLRE"}
		if strings.Contains(strings.Join(etfs, " "), strings.ToUpper(ticker)) {
			for _, symbol := range etfs {
				if !h.contains(result.RecommendedSymbols, symbol) {
					result.RecommendedSymbols = append(result.RecommendedSymbols, model.RecommendedSymbol{Symbol: symbol})
				}
			}
		}

		for _, recommendedSymbol := range result.RecommendedSymbols {
			others = append(others, recommendedSymbol.Symbol)
			result, meta := h.yahooService.GetStock(recommendedSymbol.Symbol, "1d", "1y")
			fiftyWeekHigh := meta.RegularMarketPrice >= meta.FiftyTwoWeekHigh

			quotes := result.Indicators.Quote
			above200 := meta.RegularMarketPrice >= h.yahooService.GetMovingAverage(quotes[0].Close, 200)
			above150 := meta.RegularMarketPrice >= h.yahooService.GetMovingAverage(quotes[0].Close, 150)
			above50 := meta.RegularMarketPrice >= h.yahooService.GetMovingAverage(quotes[0].Close, 50)
			rising200 := h.yahooService.IsRisingMA(quotes[0].Close, 200)
			rising150 := h.yahooService.IsRisingMA(quotes[0].Close, 150)
			rising50 := h.yahooService.IsRisingMA(quotes[0].Close, 50)

			perf := model.Performance{
				Symbol: recommendedSymbol.Symbol,
			}
			if fiftyWeekHigh {
				perf.FiftyWeekHigh = &fiftyWeekHigh
			} else {
				perf.FiftyWeekHigh = nil
			}

			countMA := 0
			if above200 && rising200 {
				countMA += 1
			}
			if above150 && rising150 {
				countMA += 1
			}
			if above50 && rising50 {
				countMA += 1
			}

			perf.Above200DMA = &countMA
			//perf.Rising200DMA = &risingMA

			volatility := h.yahooService.Volatility(recommendedSymbol.Symbol)
			relativePerformance, returns, relativeMACount := h.yahooService.CrossIndex(recommendedSymbol.Symbol)
			perf.RelativePerformance = relativePerformance
			yearlyHigh := !strings.Contains(relativePerformance, "Below")
			totalRelativeMACount += relativeMACount

			perf.Volatility = roundFloat(volatility)
			perf.Returns = roundFloat(returns)
			perf.FiftyWeekHigh = &yearlyHigh

			details, err := h.nasdaqService.Summary(recommendedSymbol.Symbol, "stocks")
			if err != nil {
				fmt.Errorf("cannot get deatils for %s: %w", recommendedSymbol.Symbol, err)
			}

			marketCap, _ := strconv.ParseFloat(strings.Replace(details.SummaryData.MarketCap.Value, ",", "", -1), 64)

			perf.Industry = details.SummaryData.Industry.Value
			perf.Sector = details.SummaryData.Sector.Value
			perf.MarketCapB = marketCap / math.Pow10(9)
			perf.PERatio = details.SummaryData.PERatio.Value
			perf.ForwardPE1Yr = details.SummaryData.ForwardPE1Yr.Value
			perf.Yield = details.SummaryData.Yield.Value
			perf.EarningsPerShare = details.SummaryData.EarningsPerShare.Value

			close := extractNumber(details.SummaryData.PreviousClose.Value)
			oneYrtarget := extractNumber(details.SummaryData.OneYrTarget.Value)
			targetPerc := (oneYrtarget/close - 1) * 100
			perf.OneYrTarget = fmt.Sprintf("%s (price: %s) %.2f%%", details.SummaryData.OneYrTarget.Value, details.SummaryData.PreviousClose.Value, targetPerc)

			activity, err := h.nasdaqService.InsiderActivity(recommendedSymbol.Symbol)
			if err != nil {
				fmt.Errorf("cannot get insider activity for %s: %w", recommendedSymbol.Symbol, err)
			}
			for _, trade := range activity.NumberOfSharesTraded.Rows {
				if trade.InsiderTrade == "Net Activity" {
					perf.InsiderNetActivity = fmt.Sprintf("%s %s", trade.Months3, trade.Months12)
					break
				}
			}

			movingAverageByStock = append(movingAverageByStock, perf)
			count++
		}
	}
	h.fileService.Write("similar", strings.Join(others, ","))

	totalMA := 0.0
	totalExTicker := 0.0
	tickerAbove200MA := 0.0
	for _, perf := range movingAverageByStock {
		totalMA += float64(*perf.Above200DMA)
		if perf.Symbol != ticker {
			totalExTicker += float64(*perf.Above200DMA)
		} else {
			tickerAbove200MA = float64(*perf.Above200DMA)
		}
	}

	volatility := h.yahooService.Volatility(ticker)
	relativePerformance, _, _ := h.yahooService.CrossIndex(ticker)

	sort.Slice(movingAverageByStock, func(i, j int) bool {
		if movingAverageByStock[i].FiftyWeekHigh != nil && *movingAverageByStock[i].FiftyWeekHigh {
			return true
		}

		fiveDayReturn1 := movingAverageByStock[i].Returns
		fiveDayReturn2 := movingAverageByStock[j].Returns

		return fiveDayReturn1 > fiveDayReturn2
	})

	marketCap, _ := strconv.ParseFloat(strings.Replace(nasdaqInfo.SummaryData.MarketCap.Value, ",", "", -1), 64)

	close := extractNumber(nasdaqInfo.SummaryData.PreviousClose.Value)
	oneYrtarget := extractNumber(nasdaqInfo.SummaryData.OneYrTarget.Value)
	targetPerc := (oneYrtarget/close - 1) * 100

	avgVolume := nasdaqInfo.SummaryData.AverageVolume.Value
	if avgVolume == "" {
		avgVolume = fmt.Sprintf("%s vs. %s", nasdaqInfo.SummaryData.AvgDailyVol20Days.Value, nasdaqInfo.SummaryData.AvgDailyVol65Days.Value)

	}

	Success(w, r, map[string]interface{}{
		//"count": len(others),
		"info": map[string]interface{}{
			//"description":        tickerDetails.Description,
			"aSector":             nasdaqInfo.SummaryData.Sector.Value,
			"aIndustry":           nasdaqInfo.SummaryData.Industry.Value,
			"aInsiderNetActivity": insiderTrade,
			"marketCap($B)":       marketCap / math.Pow10(9),
			"oneYrTarget":         fmt.Sprintf("%s (price: %s) = %.2f%%", nasdaqInfo.SummaryData.OneYrTarget.Value, nasdaqInfo.SummaryData.PreviousClose.Value, targetPerc),
			"PERatio":             nasdaqInfo.SummaryData.PERatio.Value,
			"PEforward1Yr":        nasdaqInfo.SummaryData.ForwardPE1Yr.Value,
			"earningsPerShare":    nasdaqInfo.SummaryData.EarningsPerShare.Value,
			"yield":               nasdaqInfo.SummaryData.Yield.Value,
			//"beta":               nasdaqInfo.SummaryData.Beta.Value,
		},
		"list":                 fmt.Sprintf("%s (%d)", strings.Join(others, ","), len(others)),
		"relativeStrength":     fmt.Sprintf("%.2f%% vs. %.2f%%, totalMAScore: %f", 100*tickerAbove200MA/totalMA, 100*1/count, totalMA),
		"totalRelativeMACount": totalRelativeMACount,
		"trend":                movingAverageByStock,
		"aVolatility":          volatility,
		"relativePerformance":  relativePerformance,
		"averageVolume":        avgVolume,
		//"result":           similarStocks.Finance.Results,
	})
}

func (h Ibkr) contains(symbols []model.RecommendedSymbol, symbol string) bool {
	for _, s := range symbols {
		if symbol == s.Symbol {
			return true
		}
	}

	return false
}

func (h Ibkr) GetAll(w http.ResponseWriter, r *http.Request) {
	//h.ibkrService.Init()
	tickers := r.URL.Query()["symbol"]
	openOrders, err := h.ibkrService.GetAll(r.Context(), map[string]interface{}{"tickers": tickers})
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	tickersMap := make(map[string]bool)
	orderIDs := make(map[string][]int, len(openOrders.Orders))
	for _, order := range openOrders.Orders {
		tickersMap[order.Ticker] = true
		if _, ok := orderIDs[order.OrigOrderType]; ok {
			orderIDs[order.OrigOrderType] = append(orderIDs[order.OrigOrderType], order.OrderId)
		} else {
			orderIDs[order.OrigOrderType] = []int{order.OrderId}
		}
	}
	workingOrders := make([]string, 0)
	for _, openOrder := range openOrders.Orders {
		workingOrders = append(workingOrders, fmt.Sprintf("%d %s", openOrder.OrderId, openOrder.OrderDesc))
	}

	Success(w, r, map[string]interface{}{
		"count":         len(tickersMap),
		"orders":        openOrders.Orders,
		"workingOrders": workingOrders,
		"zzOrderIDs":    orderIDs,
	})
}

func (h Ibkr) Alerts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	alerts, err := h.alertService.GetAll(r.Context())
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	triggeredAlerts := make([]string, 0)
	if status == "triggered" {
		for _, alert := range alerts {
			if alert.AlertTriggered {
				triggeredAlerts = append(triggeredAlerts, alert.AlertName)
			}
		}
		Success(w, r, triggeredAlerts)
		return
	}

	openAlerts := make([]model.IBKRAlert, 0)
	for _, alert := range alerts {
		if alert.AlertTriggered {
			continue
		}

		openAlerts = append(openAlerts, alert)
	}

	Success(w, r, openAlerts)
}

func (h Ibkr) Order(w http.ResponseWriter, r *http.Request) {
	//h.ibkrService.Init()
	ctx := r.Context()
	var request model.IBKROrder

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: bad request", err))

		return
	}

	var responses []model.OrderResponse
	for _, order := range request.Orders {
		quote, err := h.ibkrService.Search(ctx, order.Ticker)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		conID, err := strconv.ParseInt(quote[0].ConID, 10, 32)
		if err != nil {
			fmt.Errorf("cannot parse conID: %w", err)

			return
		}
		buy, stoploss, target, _ := getPrices(order.Prices)
		if order.ConID == 0 {
			order.ConID = int(conID)
		}

		order.AcctID = viper.GetString("ACCOUNT_ID")

		if order.TIF == "" {
			order.TIF = "GTC"
		}

		order.Price = buy
		if order.OrderType == model.StopLimit {
			if order.Side == "BUY" {
				order.AuxPrice = buy
				order.Price = order.AuxPrice + order.AuxPrice*viper.GetFloat64("MARGIN")
			}

			order.AuxPrice = roundFloat(order.AuxPrice)
			order.Price = roundFloat(order.Price)
		}

		position := 0.25
		if order.Position > 0 {
			position = order.Position
		}
		if position > 1 {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("position too large: %f", position))

			return
		}

		if order.Side == "BUY" {
			if order.OrderType == model.Limit {
				order.Price = buy

				order.Price = roundFloat(order.Price)
			}
			if order.OrderType == model.Stop {
				order.Price = buy
			}

			if stoploss <= 0 {
				stoploss = 0.92 * buy
			}

			summary, err := h.positionService.Summary(ctx)
			if err != nil {
				Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

				return
			}
			totalEquity := summary.EquityWithLoanValue.Amount
			if h.totalEquity > 0 {
				totalEquity = h.totalEquity
			}

			qty, qErr := resolveQuantity(order.Quantity, totalEquity, position, buy, stoploss)
			if qErr != nil {
				Errors(w, r, http.StatusBadRequest, qErr.Error())

				return
			}
			order.Quantity = qty

			if err := h.enforceTradeRisk(ctx, totalEquity, order.Quantity, buy, stoploss); err != nil {
				Errors(w, r, http.StatusBadRequest, err.Error())

				return
			}

			order.Price = roundFloat(order.Price)
			res, err := h.ibkrService.Order(ctx, []model.Order{order})
			if err != nil {
				Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot place order for %s: %s", order.Ticker, res[0].OrderStatus))
				return
			}

			responses = append(responses, res)
		}

		if order.Side == "SELL" {
			if buy > 0 {
				totalEquity, eqErr := h.equityAmount(ctx)
				if eqErr != nil {
					Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", eqErr.Error()))
					return
				}
				if stoploss > 0 {
					qty, qErr := resolveQuantity(order.Quantity, totalEquity, position, buy, stoploss)
					if qErr != nil {
						Errors(w, r, http.StatusBadRequest, qErr.Error())
						return
					}
					order.Quantity = qty
					if err := h.enforceTradeRisk(ctx, totalEquity, order.Quantity, buy, stoploss); err != nil {
						Errors(w, r, http.StatusBadRequest, err.Error())
						return
					}
				} else if order.Quantity > 0 {
					if err := validateNotional(totalEquity, float64(order.Quantity), buy); err != nil {
						Errors(w, r, http.StatusBadRequest, err.Error())
						return
					}
				}
			}
			if order.OrderType == model.Limit {
				if target > 0 {
					order.Price = target
					order.Price = roundFloat(order.Price)

					res, err := h.ibkrService.Order(ctx, []model.Order{order})
					if err != nil {
						Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot place order for %s: %s", order.Ticker, res[0].OrderStatus))
						return
					}

					responses = append(responses, res)
				}
			}
			if order.OrderType == model.Stop {
				if stoploss > 0 {
					order.Price = stoploss
					order.Price = roundFloat(order.Price)

					res, err := h.ibkrService.Order(ctx, []model.Order{order})
					if err != nil {
						Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot place order for %s: %s", order.Ticker, res[0].OrderStatus))
						return
					}

					responses = append(responses, res)
				}
			}
		}
	}

	var tickers []string
	for _, order := range request.Orders {
		tickers = append(tickers, order.Ticker)
	}

	h.success(ctx, w, r, tickers, responses)
}

func (h Ibkr) MarketSell(w http.ResponseWriter, r *http.Request) {
	size, err := strconv.ParseFloat(r.URL.Query().Get("position"), 64)
	if err != nil {
		size = 1.0
	}
	if size <= 0 || size > 1 {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("position invalid: %f (use 0 < position <= 1; 1 = full position)", size))

		return
	}
	ctx := r.Context()
	positions, tickers, _, err := h.positionService.GetAll(ctx)
	if err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get positions", err.Error()))

		return
	}

	positionsByTicker := make(map[string]model.Position)
	for _, pos := range positions {
		positionsByTicker[pos.ContractDesc] = pos
	}

	if len(positionsByTicker) == 0 {
		Errors(w, r, http.StatusBadRequest, "no positions")

		return
	}
	symbolsChecklist := make(map[string]string)
	for _, s := range h.allSymbols {
		symbolsChecklist[strings.Split(s, ".")[0]] = s
	}

	var responses []model.OrderResponse
	symbols := r.URL.Query()["symbol"]
	if len(symbols) > 0 && symbols[0] == "ALL" {
		symbols = tickers
	}
	if len(symbols) == 0 {
		Errors(w, r, http.StatusBadRequest, "symbol is required")

		return
	}

	closed := 0
	for _, symbol := range symbols {
		isCoveringShort := false
		pos, ok := positionsByTicker[symbol]
		if !ok || math.Abs(pos.Position) < 1e-9 {
			continue
		}

		held := math.Abs(pos.Position)
		desired := int(math.Round(held * size))
		if desired <= 0 {
			continue
		}

		currentOrders, err := h.ibkrService.GetAll(ctx, map[string]interface{}{
			"tickers": []string{symbol},
		})
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot get order for %s: %s", symbol, err.Error()))
			return
		}

		// Working exit size (OCA stop+limit share size — use max, don't double-count).
		//workingExit := workingExitQuantity(currentOrders.Orders, pos.Position < 0)
		//available := int(math.Floor(held + 1e-9)) - workingExit
		if pos.Position == 0 {
			// Already fully covered by working exits — do not sell more (would short/cover past flat).
			continue
		}

		var order model.Order
		order.Ticker = pos.ContractDesc
		order.Quantity = desired
		if pos.Position < 0 {
			isCoveringShort = true
		}
		order.ConID = pos.Conid
		order.AcctID = pos.AcctId
		order.TIF = "GTC"
		order.Side = "SELL"
		order.OrderType = "LMT"
		if isCoveringShort {
			order.Side = "BUY"
			order.OrderType = "LMT"
		}

		yahooSymbol, ok := symbolsChecklist[strings.Replace(pos.ContractDesc, ".", "-", -1)]
		if !ok {
			yahooSymbol = pos.ContractDesc
		}
		result, _ := h.yahooService.GetStock(yahooSymbol, "1d", "1w")
		quote := result.Indicators.Quote
		if len(quote) == 0 || len(quote[0].Close) == 0 {
			result1, err := h.ibdService.GetStockQuotes(context.Background(), symbol, time.Now())
			if err != nil || len(result1) == 0 {
				msg := "cannot get price quotes"
				if err != nil {
					msg = err.Error()
				}
				Errors(w, r, http.StatusInternalServerError, msg)
				return
			}
			quote = result1
		}

		closes := quote[0].Close
		closingPrice := closes[len(closes)-1]
		if closingPrice <= 0 {
			closingPrice = result.Meta.RegularMarketPrice
		}
		if closingPrice <= 0 {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("invalid closing price for %s: %0.2f", symbol, closingPrice))
			return
		}
		if isCoveringShort {
			order.Price = roundFloat(closingPrice * 1.005)
		} else {
			order.Price = roundFloat(closingPrice * 0.995)
		}

		res, err := h.ibkrService.Order(ctx, []model.Order{order})
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot place order for %s: %s", order.Ticker, err.Error()))
			return
		}

		responses = append(responses, res)
		remaining := held - float64(order.Quantity)
		contractResponse, err := h.updatePosition(ctx, currentOrders, order, remaining)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot update attached orders for %s: %s", order.Ticker, err.Error()))
			return
		}
		responses = append(responses, contractResponse...)
		closed++
	}

	if closed == 0 {
		Errors(w, r, http.StatusBadRequest, "nothing to close")

		return
	}

	h.success(ctx, w, r, symbols, responses)
}

// isExitOrder reports whether o is a working exit leg for the open position.
// coveringShort: exits are BUY covers; otherwise exits are SELL stops/targets.
func isExitOrder(o model.OrderDetails, coveringShort bool) bool {
	status := strings.ToUpper(o.Status)
	if status == "FILLED" || status == "CANCELLED" || status == "CANCELED" || status == "INACTIVE" {
		return false
	}
	desc := strings.ToUpper(strings.TrimSpace(o.OrderDesc))
	side := strings.ToUpper(o.Side)
	isSell := side == "SELL" || side == "S" || strings.HasPrefix(desc, "SELL")
	isBuy := side == "BUY" || side == "B" || strings.HasPrefix(desc, "BUY")
	if coveringShort {
		return isBuy
	}
	return isSell
}

// workingExitQuantity returns shares already spoken for by working exit orders.
// For OCA brackets (stop+limit), both legs share size — take the max so we don't double-count.
func workingExitQuantity(orders []model.OrderDetails, coveringShort bool) int {
	maxExit := 0.0
	for _, o := range orders {
		if !isExitOrder(o, coveringShort) {
			continue
		}
		qty := o.RemainingQuantity
		if qty <= 0 {
			qty = o.TotalSize
		}
		if qty > maxExit {
			maxExit = qty
		}
	}
	return int(math.Floor(maxExit + 1e-9))
}

// updatePosition resizes (or cancels) exit legs after a close/cover. It ignores
// entry/add orders that share the same ConID.
func (h Ibkr) updatePosition(ctx context.Context, orders model.AllOrders, closeOrder model.Order, remaining float64) ([]model.OrderResponse, error) {
	coveringShort := strings.EqualFold(closeOrder.Side, "BUY")
	responses := make([]model.OrderResponse, 0)
	for _, o := range orders.Orders {
		if closeOrder.ConID != o.Conid || !isExitOrder(o, coveringShort) {
			continue
		}
		if o.Ticker == "" {
			return nil, fmt.Errorf("no order found")
		}

		if remaining <= 0 {
			if _, err := h.ibkrService.Cancel(ctx, o.Ticker, fmt.Sprintf("%d", o.OrderId)); err != nil {
				return nil, err
			}
			continue
		}

		contractOrder := o
		if o.Price != "" {
			contractOrder.Price = o.Price
		}
		if o.StopPrice != "" {
			contractOrder.Price = o.StopPrice
		}
		if contractOrder.Side == "" {
			if coveringShort {
				contractOrder.Side = "BUY"
			} else {
				contractOrder.Side = "SELL"
			}
		}
		contractOrder.TotalSize = remaining
		contractResponse, err := h.updateContract(ctx, contractOrder)
		if err != nil {
			return nil, err
		}
		responses = append(responses, contractResponse...)
	}

	return responses, nil
}

func (h Ibkr) ContractID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	exchange := r.URL.Query().Get("exchange")
	ticker := r.URL.Query().Get("ticker")
	quote, err := h.ibkrService.Search(ctx, ticker)
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	conIDStr := quote[0].ConID
	if exchange == "" {
		exchange = quote[0].Description
	}
	for _, q := range quote {
		if q.Description != "VENTURE" {
			conIDStr = q.ConID
		}
	}
	for _, q := range quote {
		if q.Description == strings.ToUpper(exchange) || q.Description == "NYSE" || q.Description == "TSE" || q.Description == "NASDAQ" || q.Description == "AMEX" || q.Description == "ARCA" || q.Description == "BATS" {
			conIDStr = q.ConID
			if exchange == "" {
				exchange = q.Description
			}
			break
		}
	}
	for _, q := range quote {
		if q.Description == strings.ToUpper(exchange) {
			conIDStr = q.ConID
			break
		}
	}

	conID, err := strconv.ParseInt(conIDStr, 10, 32)
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, r, map[string]interface{}{
		"contractID": conID,
		"exchange":   exchange,
	})
}

func (h Ibkr) MarketBuy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	exchange := r.URL.Query().Get("exchange")
	stopLossParam := r.URL.Query().Get("stopLoss")
	targetStr := r.URL.Query().Get("target")
	conID, _ := strconv.ParseInt(r.URL.Query().Get("coID"), 10, 32)

	symbols := r.URL.Query()["symbol"]
	if len(symbols) != 1 {
		Errors(w, r, http.StatusBadRequest, "exactly one symbol is required")

		return
	}
	symbol := symbols[0]

	summary, err := h.positionService.Summary(ctx)
	if err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

		return
	}
	totalEquity := summary.EquityWithLoanValue.Amount
	if h.totalEquity > 0 {
		totalEquity = h.totalEquity
	}

	size, err := strconv.ParseFloat(r.URL.Query().Get("position"), 64)
	if err != nil {
		size = 1.0
	}
	if size > 1 {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("position too large: %f (max 1 = 1%% of equity at risk)", size))

		return
	}

	target, _ := strconv.ParseFloat(targetStr, 64)

	symbolsChecklist := make(map[string]string)
	for _, s := range h.allSymbols {
		symbolsChecklist[strings.Split(s, ".")[0]] = s
	}

	var order model.Order
	order.Ticker = symbol

	if conID <= 0 {
		quote, err := h.ibkrService.Search(ctx, order.Ticker)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		conIDStr := quote[0].ConID
		for _, q := range quote {
			if q.Description != "VENTURE" {
				conIDStr = q.ConID
			}
		}
		for _, q := range quote {
			if q.Description == strings.ToUpper(exchange) || q.Description == "NYSE" || q.Description == "NASDAQ" || q.Description == "ARCA" || q.Description == "AMEX" || q.Description == "TSE" {
				conIDStr = q.ConID
				break
			}
		}
		conID, err = strconv.ParseInt(conIDStr, 10, 32)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())

			return
		}
	}
	order.ConID = int(conID)
	yahooSymbol, ok := symbolsChecklist[strings.Replace(symbol, ".", "-", -1)]
	if !ok {
		yahooSymbol = symbol
	}
	yahooSymbol = strings.Replace(strings.Replace(yahooSymbol, ".NYSE", "", -1), ".NASDAQ", "", -1)
	tickersYahoo := map[string]string{
		"U.U": "U-UN.TO",
	}
	ibkrTicker, ok := tickersYahoo[yahooSymbol]
	if ok {
		yahooSymbol = ibkrTicker
	}
	result, _ := h.yahooService.GetStock(yahooSymbol, "1d", "1m")
	quotes := result.Indicators.Quote
	if len(quotes) == 0 || len(quotes[0].Close) == 0 {
		quotes, err = h.ibdService.GetStockQuotes(context.Background(), symbol, time.Now())
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())

			return
		}
	}

	closes := quotes[0].Close
	closingPrice := closes[len(closes)-1]
	if closingPrice <= 0 {
		closingPrice = result.Meta.RegularMarketPrice
	}
	if closingPrice <= 0 {
		Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("invalid closing price for %s: %0.2f", symbol, closingPrice))

		return
	}
	buy := roundFloat(closingPrice * 1.003)

	atr := h.yahooService.ATR(quotes[0], 22)
	sl := 3 * atr
	stoploss := roundFloat(buy - sl)
	if stopLossParam != "" {
		sl, _ = strconv.ParseFloat(stopLossParam, 64)
		stoploss = roundFloat(buy * (1 - (sl / 100)))
	}

	if target <= 0 {
		target = roundFloat(buy * 1.3)
	}
	target = roundFloat(target)

	order.CoID = fmt.Sprintf("Parent_%s_%s", order.Ticker, time.Now().Format("2006-01-02 15:04:05"))
	order.AcctID = viper.GetString("ACCOUNT_ID")
	order.Side = "BUY"
	order.OrderType = model.Limit
	order.TIF = "GTC"
	order.Quantity = int(math.Round((totalEquity * size / 100) / math.Abs(buy-stoploss)))
	if err := h.enforceTradeRisk(ctx, totalEquity, order.Quantity, buy, stoploss); err != nil {
		Errors(w, r, http.StatusBadRequest, err.Error())

		return
	}
	order.Price = roundFloat(buy)

	takeProfit := order
	takeProfit.ParentID = order.CoID
	takeProfit.Side = "SELL"
	takeProfit.OrderType = "LMT"
	takeProfit.Price = target
	takeProfit.CoID = ""

	stopLossOrder := order
	stopLossOrder.ParentID = order.CoID
	stopLossOrder.Side = "SELL"
	stopLossOrder.OrderType = "STP"
	stopLossOrder.Price = stoploss
	stopLossOrder.CoID = ""

	res, err := h.ibkrService.Order(ctx, []model.Order{order, takeProfit, stopLossOrder})
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot place order for %s", order.Ticker))
		return
	}

	h.success(ctx, w, r, []string{symbol}, []model.OrderResponse{res})
}
func (h Ibkr) Preview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	exchange := r.URL.Query().Get("exchange")
	stopLossParam := r.URL.Query().Get("stopLoss")
	entryParam := r.URL.Query().Get("entry")
	side := r.URL.Query().Get("side")
	entry, _ := strconv.ParseFloat(entryParam, 64)
	entry = roundFloat(entry)
	conID, _ := strconv.ParseInt(r.URL.Query().Get("coID"), 10, 32)

	symbols := r.URL.Query()["symbol"]
	if len(symbols) != 1 {
		Errors(w, r, http.StatusBadRequest, "exactly one symbol is required")

		return
	}
	symbol := symbols[0]

	summary, err := h.positionService.Summary(ctx)
	if err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

		return
	}
	totalEquity := summary.EquityWithLoanValue.Amount
	if h.totalEquity > 0 {
		totalEquity = h.totalEquity
	}

	size, err := strconv.ParseFloat(r.URL.Query().Get("position"), 64)
	if err != nil {
		size = 1.0
	}
	if size > 1 {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("position too large: %f (max 1 = 1%% of equity at risk)", size))

		return
	}

	positionSize := totalEquity * 0.01 * size
	quantity := 0.0
	total := 0.0
	stopLoss := 0.0

	symbolsChecklist := make(map[string]string)
	for _, s := range h.allSymbols {
		symbolsChecklist[strings.Split(s, ".")[0]] = s
	}

	var order model.Order
	order.Ticker = symbol

	if conID <= 0 {
		tickersIBKR := map[string]string{
			"U-UN": "U.UN",
		}

		ibkrTicker, ok := tickersIBKR[order.Ticker]
		if ok {
			order.Ticker = ibkrTicker
		}
		quote, err := h.ibkrService.Search(ctx, order.Ticker)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		conIDStr := quote[0].ConID
		for _, q := range quote {
			if q.Description != "VENTURE" {
				conIDStr = q.ConID
			}
		}
		for _, q := range quote {
			if q.Description == strings.ToUpper(exchange) || q.Description == "NYSE" || q.Description == "TSE" || q.Description == "NASDAQ" || q.Description == "AMEX" || q.Description == "ARCA" || q.Description == "BATS" {
				conIDStr = q.ConID
				break
			}
		}
		for _, q := range quote {
			if q.Description == strings.ToUpper(exchange) {
				conIDStr = q.ConID
				break
			}
		}
		conID, err = strconv.ParseInt(conIDStr, 10, 32)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())

			return
		}
	}
	order.ConID = int(conID)

	buy := 0.0
	if entry > 0 {
		buy = roundFloat(entry)
	}

	stopLossPercent := 0.0
	if stopLossParam != "" {
		parsed, parseErr := strconv.ParseFloat(stopLossParam, 64)
		if parseErr != nil || parsed <= 0 {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("invalid stopLoss: %s", stopLossParam))

			return
		}
		stopLossPercent = parsed
	}

	// When entry + stopLoss% are both provided, size from those alone (no Yahoo/ATR).
	needQuotes := buy <= 0 || stopLossPercent <= 0
	var quotes []model.Quote
	if needQuotes {
		yahooSymbol, ok := symbolsChecklist[strings.Replace(symbol, ".", "-", -1)]
		if !ok {
			yahooSymbol = symbol
		}
		yahooSymbol = strings.Replace(strings.Replace(yahooSymbol, ".NYSE", "", -1), ".NASDAQ", "", -1)
		tickersYahoo := map[string]string{
			"U.UN": "U-UN.TO",
			"U-UN": "U-UN.TO",
		}
		if ibkrTicker, ok := tickersYahoo[yahooSymbol]; ok {
			yahooSymbol = ibkrTicker
		}

		result, _ := h.yahooService.GetStock(yahooSymbol, "1d", "1y")
		quotes = result.Indicators.Quote
		if len(quotes) == 0 || len(quotes[0].Close) == 0 {
			quotes, err = h.ibdService.GetStockQuotes(context.Background(), symbol, time.Now())
			if err != nil {
				Errors(w, r, http.StatusInternalServerError, err.Error())

				return
			}
		}

		closes := quotes[0].Close
		closingPrice := closes[len(closes)-1]
		if closingPrice <= 0 {
			closingPrice = result.Meta.RegularMarketPrice
		}
		if buy <= 0 {
			buy = roundFloat(closingPrice * 1.003)
		}
	}

	if stopLossPercent > 0 {
		if strings.ToLower(side) == "sell" {
			stopLoss = roundFloat(buy * (1 + (stopLossPercent / 100)))
		} else {
			stopLoss = roundFloat(buy * (1 - (stopLossPercent / 100)))
		}
	} else {
		if len(quotes) == 0 {
			Errors(w, r, http.StatusBadRequest, "stopLoss is required when quotes are unavailable")

			return
		}
		atr := h.yahooService.ATR(quotes[0], 22)
		sl := 3 * atr
		if strings.ToLower(side) == "sell" {
			stopLoss = roundFloat(buy + sl)
		} else {
			stopLoss = roundFloat(buy - sl)
		}
		if buy > 0 {
			stopLossPercent = 100 * math.Abs(buy-stopLoss) / buy
		}
	}

	if buy <= 0 || stopLoss <= 0 || buy == stopLoss {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("invalid entry/stop: entry=%0.2f stop=%0.2f", buy, stopLoss))

		return
	}

	if strings.ToLower(side) == "sell" {
		quantity = math.Round(positionSize / (stopLoss - buy))
	} else {
		quantity = math.Round(positionSize / (buy - stopLoss))
	}
	total = quantity * buy

	Success(w, r, map[string]interface{}{
		"quantity":        quantity,
		"entry":           buy,
		"stopLoss":        stopLoss,
		"stopLossPercent": roundFloat(stopLossPercent),
		"position":        fmt.Sprintf("Amount: %0.2f, loss %0.2f, 1%% loss %0.2f stopLevel: %0.2f (%0.2f%%) ", total, positionSize, total*0.01, stopLoss, stopLossPercent),
		"record":          fmt.Sprintf("recAll %s %0.0f %0.2f %s", symbol, math.Round(quantity), total/quantity, time.Now().Format("2006/01/02")),
	})
}
func (h Ibkr) MarketSell2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	exchange := r.URL.Query().Get("exchange")
	stopLossParam := r.URL.Query().Get("stopLoss")
	summary, err := h.positionService.Summary(ctx)
	if err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

		return
	}

	//sgdusd := h.getFX("SGDUSD")
	totalEquity := summary.EquityWithLoanValue.Amount
	_, _, _, _, _, _, potentialLoss, _, err := h.getOrders(r.Context())
	if err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get working orders", err.Error()))

		return
	}

	coreEquity := totalEquity - potentialLoss

	size, err := strconv.ParseFloat(r.URL.Query().Get("position"), 64)
	if err != nil {
		size = 1.0
	}
	if size > 1 {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("position too large: %f (max 1 = 1%% of equity at risk)", size))

		return
	}

	positionSize := coreEquity * 0.01 * size

	symbolsChecklist := make(map[string]string)
	for _, s := range h.allSymbols {
		symbolsChecklist[strings.Split(s, ".")[0]] = s
	}

	var responses []model.OrderResponse
	symbols := r.URL.Query()["symbol"]
	for _, symbol := range symbols {
		var order model.Order
		order.Ticker = symbol

		quote, err := h.ibkrService.Search(ctx, order.Ticker)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		conIDStr := quote[0].ConID
		for _, q := range quote {
			if q.Description != "VENTURE" {
				conIDStr = q.ConID
			}
		}
		for _, q := range quote {
			if q.Description == strings.ToUpper(exchange) || q.Description == "NYSE" || q.Description == "TSE" || q.Description == "NASDAQ" {
				conIDStr = q.ConID
				break
			}
		}
		conID, err := strconv.ParseInt(conIDStr, 10, 32)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())

			return
		}
		order.ConID = int(conID)
		yahooSymbol, ok := symbolsChecklist[strings.Replace(symbol, ".", "-", -1)]
		if !ok {
			yahooSymbol = symbol
			//continue
		}
		yahooSymbol = strings.Replace(strings.Replace(yahooSymbol, ".NYSE", "", -1), ".NASDAQ", "", -1)
		result, _ := h.yahooService.GetStock(yahooSymbol, "1d", "1w")
		quotes := result.Indicators.Quote
		if len(quotes) == 0 || len(quotes[0].Close) == 0 {
			Errors(w, r, http.StatusInternalServerError, "cannot get price")

			return
		}

		closes := quotes[0].Close
		closingPrice := closes[len(closes)-1]
		if closingPrice <= 0 {
			closingPrice = result.Meta.RegularMarketPrice
		}
		if closingPrice <= 0 {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("invalid closing price for %s: %0.2f", symbol, closingPrice))

			return
		}
		buy := roundFloat(closingPrice * 0.997)
		sl := 4.0
		stoploss := roundFloat(buy * (1 + (sl / 100)))
		if stopLossParam != "" {
			sl, _ = strconv.ParseFloat(stopLossParam, 64)
			stoploss = roundFloat(buy * (1 + (sl / 100)))
		}
		target := roundFloat(buy * 0.8)

		// buy
		order.CoID = fmt.Sprintf("Parent_%s_%s", order.Ticker, time.Now().Format("2006-01-02 15:04:05"))
		order.AcctID = viper.GetString("ACCOUNT_ID")
		order.Side = "SELL"
		order.OrderType = model.Limit
		order.TIF = "GTC"
		order.Quantity = -int(math.Round(positionSize / (buy - stoploss)))
		if err := h.enforceTradeRisk(ctx, coreEquity, order.Quantity, buy, stoploss); err != nil {
			Errors(w, r, http.StatusBadRequest, err.Error())

			return
		}
		//order.AuxPrice = roundFloat(buy) // stop
		//order.Price = roundFloat(order.AuxPrice + order.AuxPrice*viper.GetFloat64("MARGIN"))
		order.Price = roundFloat(buy)

		// target
		takeProfit := order
		takeProfit.ParentID = order.CoID
		takeProfit.Side = "BUY"
		takeProfit.OrderType = "LMT"
		takeProfit.Price = target
		takeProfit.CoID = ""

		// stop
		stopLossOrder := order
		stopLossOrder.ParentID = order.CoID
		stopLossOrder.Side = "BUY"
		stopLossOrder.OrderType = "STP"
		stopLossOrder.Price = stoploss
		stopLossOrder.CoID = ""

		res, err := h.ibkrService.Order(ctx, []model.Order{order, takeProfit, stopLossOrder})
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot place order for %s", order.Ticker))
			return
		}

		responses = append(responses, res)
	}

	h.success(ctx, w, r, symbols, responses)
}

func (h Ibkr) Reply(w http.ResponseWriter, r *http.Request) {
	replyID := strings.TrimSpace(chi.URLParam(r, "replyID"))
	if replyID == "" {
		Errors(w, r, http.StatusInternalServerError, "invalid replyID")

		return
	}
	res, err := h.ibkrService.Reply(replyID)
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())

		return
	}

	Success(w, r, res)
}

func (h Ibkr) BracketOrder(w http.ResponseWriter, r *http.Request) {
	//h.ibkrService.Init()
	ctx := r.Context()
	var request model.IBKROrder

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())

		return
	}

	urlPosition := 0.0
	if p := r.URL.Query().Get("position"); p != "" {
		parsed, err := strconv.ParseFloat(p, 64)
		if err != nil || parsed <= 0 {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("invalid position: %s", p))

			return
		}
		urlPosition = parsed
	}

	var responses []model.OrderResponse
	for _, order := range request.Orders {
		exchange := ""
		ticker := order.Ticker
		if strings.Contains(order.Ticker, ".") {
			exchange = strings.Split(order.Ticker, ".")[len(strings.Split(order.Ticker, "."))-1]
			parts := strings.Split(order.Ticker, ".")[0 : len(strings.Split(order.Ticker, "."))-1]
			ticker = strings.Join(parts, "")
		}
		tickersIBKR := map[string]string{
			"U-UN": "U.UN",
		}

		ibkrTicker, ok := tickersIBKR[order.Ticker]
		if ok {
			ticker = ibkrTicker
		}

		quote, err := h.ibkrService.Search(ctx, ticker)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		conIDStr := fmt.Sprintf("%d", order.ConID)

		if order.ConID == 0 || conIDStr == "" {
			for _, q := range quote {
				if q.Description != "VENTURE" {
					conIDStr = q.ConID
				}
			}
			for _, q := range quote {
				if q.Description == "NYSE" || q.Description == "TSE" || q.Description == "NASDAQ" || q.Description == "AMEX" || q.Description == "ARCA" || q.Description == "BATS" {
					conIDStr = q.ConID
					break
				}
			}
			for _, q := range quote {
				if q.Description == strings.ToUpper(exchange) {
					conIDStr = q.ConID
				}
			}
		}
		conID, err := strconv.ParseInt(conIDStr, 10, 32)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot parse conID: %v", err))

			return
		}
		if order.ConID > 0 {
			conID = int64(order.ConID)
		}

		ibkrTicker, ok = tickersIBKR[order.Ticker]
		if ok {
			order.Ticker = ibkrTicker
		}

		existingOrders, err := h.ibkrService.GetAll(ctx, map[string]interface{}{
			"tickers": []string{order.Ticker},
		})
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot find error: %v", err))

			return
		}
		for _, existingOrder := range existingOrders.Orders {
			if strings.Contains(existingOrder.OrderRef, fmt.Sprintf("Parent_%s", order.Ticker)) && existingOrder.Status != "Filled" {
				Errors(w, r, http.StatusBadRequest, "order already exists")

				return
			}
		}
		buy, stopLoss, target, target2 := getPrices(order.Prices)
		position := 0.25
		if urlPosition > 0 {
			position = urlPosition
		} else if order.Position > 0 {
			position = order.Position
		}
		if position > 1 {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("position too large: %f (max 1 = 1%% of equity at risk)", position))

			return
		}

		summary, err := h.positionService.Summary(ctx)
		if err != nil {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

			return
		}
		totalEquity := summary.EquityWithLoanValue.Amount
		if h.totalEquity > 0 {
			totalEquity = h.totalEquity
		}

		if order.Marketprice {
			symbolsChecklist := make(map[string]string)
			for _, s := range h.allSymbols {
				symbolsChecklist[strings.Split(s, ".")[0]] = s
			}
			yahooSymbol, ok := symbolsChecklist[strings.Replace(order.Ticker, ".", "-", -1)]
			if !ok {
				yahooSymbol = order.Ticker
			}
			tickersYahoo := map[string]string{
				"U.UN":     "U-UN.TO",
				"U-UN.TSE": "U-UN.TO",
			}
			yahooTicker, ok := tickersYahoo[order.Ticker]
			if ok {
				yahooSymbol = yahooTicker
			}
			result, _ := h.yahooService.GetStock(yahooSymbol, "1d", "1w")
			if len(result.Indicators.Quote) > 0 && len(result.Indicators.Quote[0].Close) > 0 {
				closes := result.Indicators.Quote[0].Close
				buy = closes[len(closes)-1]
			}
		}

		if target == 0 {
			if order.Side == "SELL" {
				target = buy * 0.80
			} else {
				target = buy * 1.30
			}
		}

		qty, err := resolveQuantity(order.Quantity, totalEquity, position, buy, stopLoss)
		if err != nil {
			Errors(w, r, http.StatusBadRequest, err.Error())

			return
		}
		order.Quantity = qty

		if err := h.enforceTradeRisk(ctx, totalEquity, order.Quantity, buy, stopLoss); err != nil {
			Errors(w, r, http.StatusBadRequest, err.Error())

			return
		}

		if order.ConID == 0 {
			order.ConID = int(conID)
		}
		order.AcctID = viper.GetString("ACCOUNT_ID")
		order.CoID = fmt.Sprintf("Parent_%s_%s", order.Ticker, time.Now().Format("2006-01-02 15:04:05"))

		if order.TIF == "" {
			order.TIF = "GTC"
		}

		halfQty1 := int(order.Quantity / 2)
		halfQty2 := order.Quantity - halfQty1

		isShort := strings.ToUpper(order.Side) == "SELL"
		exitSide := "SELL"
		if isShort {
			exitSide = "BUY"
		}

		takeProfit := order
		takeProfit.ParentID = order.CoID
		takeProfit.Side = exitSide
		takeProfit.OrderType = "LMT"
		takeProfit.Price = target
		takeProfit.CoID = ""

		takeProfit2 := takeProfit
		// Long: target2 above target; short: target2 below target (further profit).
		hasSecondTarget := target2 > 0 && ((isShort && target2 < target) || (!isShort && target2 > target))
		if hasSecondTarget {
			takeProfit2.ParentID = order.CoID
			takeProfit2.Price = target2
			takeProfit2.Quantity = halfQty2
			takeProfit.Quantity = halfQty1
		}

		stopLossOrder := order
		stopLossOrder.ParentID = order.CoID
		stopLossOrder.Side = exitSide
		stopLossOrder.OrderType = "STP"
		stopLossOrder.Price = stopLoss
		stopLossOrder.CoID = ""

		order.Price = buy
		if order.OrderType == model.StopLimit {
			if order.Side == "BUY" {
				order.AuxPrice = buy // stop
				order.Price = order.AuxPrice + order.AuxPrice*viper.GetFloat64("MARGIN")
			} else {
				order.AuxPrice = buy
				order.Price = order.AuxPrice - order.AuxPrice*viper.GetFloat64("MARGIN")
			}
		}
		if order.OrderType == model.Limit {
			order.Price = buy
		}

		order.AuxPrice = roundFloat(order.AuxPrice)
		order.Price = roundFloat(order.Price)
		stopLossOrder.Price = roundFloat(stopLossOrder.Price)
		takeProfit.Price = roundFloat(takeProfit.Price)
		takeProfit2.Price = roundFloat(takeProfit2.Price)

		var res model.OrderResponse
		if hasSecondTarget {
			res, err = h.ibkrService.Order(ctx, []model.Order{order, takeProfit, takeProfit2, stopLossOrder})
		} else {
			res, err = h.ibkrService.Order(ctx, []model.Order{order, takeProfit, stopLossOrder})
		}
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())

			return
		}

		responses = append(responses, res)
	}

	var tickers []string
	for _, order := range request.Orders {
		tickers = append(tickers, order.Ticker)
	}
	h.success(ctx, w, r, tickers, responses)
}

func (h Ibkr) OCA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request model.IBKROrder
	side := r.URL.Query().Get("side")

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())

		return
	}

	var responses []model.OrderResponse
	for _, order := range request.Orders {
		data, err := h.ibkrService.Search(ctx, order.Ticker)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		tickerData := data[0]
		for _, d := range data {
			if strings.Contains(d.Description, "NASDAQ") || strings.Contains(d.Description, "NYSE") || strings.Contains(d.Description, "ARCA") || strings.Contains(d.Description, "TSE") {
				tickerData = d
				break
			}
		}
		conID, err := strconv.ParseInt(tickerData.ConID, 10, 32)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot parse conID: %v", err))

			return
		}
		buy, stopLoss, target, _ := getPrices(order.Prices)
		if order.ConID == 0 {
			order.ConID = int(conID)
		}
		order.AcctID = viper.GetString("ACCOUNT_ID")
		//parentID := fmt.Sprintf("Parent_%s_%s", order.Ticker, time.Now().Format("2006-01-02 15:04:05"))

		if order.TIF == "" {
			order.TIF = "GTC"
		}

		if order.Quantity <= 1 {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("quantity must be > 1 for OCA orders (got %d)", order.Quantity))

			return
		}

		if order.Price <= 0 {
			order.Price = buy
		}
		equity, err := h.equityAmount(ctx)
		if err != nil {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

			return
		}
		if stopLoss > 0 {
			if err := h.enforceTradeRisk(ctx, equity, order.Quantity, order.Price, stopLoss); err != nil {
				Errors(w, r, http.StatusBadRequest, err.Error())

				return
			}
		} else if err := validateNotional(equity, float64(order.Quantity), order.Price); err != nil {
			Errors(w, r, http.StatusBadRequest, err.Error())

			return
		}

		if target <= 0 {
			if strings.ToUpper(side) == "BUY" {
				target = order.Price * 1.20
			} else {
				target = order.Price * 0.80
			}
		}

		takeProfit := order
		takeProfit.Side = "SELL"
		if strings.ToLower(side) == "sell" {
			takeProfit.Side = "BUY"
		}
		takeProfit.OrderType = "LMT"
		//takeProfit.IsSingleGroup = true
		takeProfit.Price = target
		//takeProfit.CoID = parentID

		stopLossOrder := order
		stopLossOrder.Side = "SELL"
		if strings.ToLower(side) == "sell" {
			stopLossOrder.Side = "BUY"
		}
		stopLossOrder.OrderType = "STP"
		//stopLossOrder.IsSingleGroup = true
		stopLossOrder.Price = stopLoss

		//order.AuxPrice = roundFloat(order.AuxPrice)
		//order.Price = roundFloat(order.Price)
		stopLossOrder.Price = roundFloat(stopLossOrder.Price)
		takeProfit.Price = roundFloat(takeProfit.Price)

		res, err := h.ibkrService.Order(ctx, []model.Order{takeProfit})
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot place order for %s: %v", order.Ticker, err))

			return
		}
		responses = append(responses, res)

		res, err = h.ibkrService.Order(ctx, []model.Order{stopLossOrder})
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot place order for %s: %v", order.Ticker, err))

			return
		}

		responses = append(responses, res)
	}

	var tickers []string
	for _, order := range request.Orders {
		tickers = append(tickers, order.Ticker)
	}
	h.success(ctx, w, r, tickers, responses)
}

func (h Ibkr) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request model.UpdateRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		fmt.Errorf("%v: bad request", err)

		return
	}

	var responses []model.OrderResponse
	for _, order := range request.Orders {
		updateOrder := model.Order{}
		//quote := h.ibkrService.Search(ctx, order.Ticker)
		//if len(quote) == 0 {
		//	Errors(w, r, http.StatusInternalServerError, "cannot search symbol")
		//	return
		//}
		//conID, err := strconv.ParseInt(quote[0].ConID, 10, 32)
		//if err != nil {
		//	fmt.Printf("cannot parse conID: %w", err)
		//
		//	return
		//}
		buy, stopLoss, target, _ := getPrices(order.Prices)

		updateOrder.ConID = order.Conid
		updateOrder.AcctID = order.Account
		updateOrder.OrderId = &order.OrderId

		updateOrder.TIF = order.TimeInForce
		if updateOrder.TIF == "" {
			updateOrder.TIF = "GTC"
		}

		updateOrder.Price = buy
		updateOrder.Quantity = int(order.TotalSize)
		updateOrder.Ticker = order.Ticker

		if order.OrderType == "Stop" {
			updateOrder.OrderType = "STP"
			updateOrder.Price = stopLoss
			updateOrder.Side = order.Side
		}
		if order.OrderType == "Limit" {
			updateOrder.OrderType = "LMT"
			updateOrder.Price = target
			updateOrder.Side = order.Side
		}
		if order.OrderType == "Stop Limit" {
			updateOrder.OrderType = model.StopLimit
			updateOrder.Price = buy
			//updateOrder.AuxPrice = order.Aux
			updateOrder.Side = order.Side
		}

		if updateOrder.Quantity > 0 {
			equity, eqErr := h.equityAmount(ctx)
			if eqErr != nil {
				Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", eqErr.Error()))

				return
			}
			entry := buy
			if entry <= 0 {
				entry = updateOrder.Price
			}
			if entry > 0 && stopLoss > 0 {
				if err := h.enforceTradeRisk(ctx, equity, updateOrder.Quantity, entry, stopLoss); err != nil {
					Errors(w, r, http.StatusBadRequest, err.Error())

					return
				}
			} else if updateOrder.Price > 0 {
				if err := validateNotional(equity, float64(updateOrder.Quantity), updateOrder.Price); err != nil {
					Errors(w, r, http.StatusBadRequest, err.Error())

					return
				}
			}
		}
		//if order.OrderType == model.StopLimit {
		//	if order.Side == "BUY" {
		//		order.AuxPrice = buy
		//		order.Price = order.AuxPrice + order.AuxPrice*viper.GetFloat64("MARGIN")
		//	} else if stopLoss > 0 {
		//		order.AuxPrice = stopLoss
		//		order.Price = order.AuxPrice - order.AuxPrice*viper.GetFloat64("MARGIN")
		//	}
		//
		//	order.AuxPrice = roundFloat(order.AuxPrice)
		//	order.Price = roundFloat(order.Price)
		//
		//}
		//if order.OrderType == model.Stop {
		//	if order.Side == "BUY" {
		//		order.Price = buy
		//	} else if stopLoss > 0 {
		//		order.Price = stopLoss
		//	} else {
		//		Errors(w, r, http.StatusBadRequest, "invalid price")
		//		return
		//	}
		//
		//	order.Price = roundFloat(order.Price)
		//
		//}
		//if order.OrderType == model.Limit {
		//	if order.Side == "BUY" {
		//		order.Price = buy
		//	} else if target > 0 {
		//		order.Price = target
		//	} else {
		//		Errors(w, r, http.StatusBadRequest, "invalid price")
		//		return
		//	}
		//
		//	order.Price = roundFloat(order.Price)
		//
		//}

		res, err := h.ibkrService.Update(ctx, updateOrder)
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())

			return
		}

		responses = append(responses, res)

		//if order.OrderType == model.StopLimit {
		//	if order.Side == "SELL" && target > 0 {
		//		order.AuxPrice = target
		//		order.Price = order.AuxPrice - order.AuxPrice*viper.GetFloat64("MARGIN")
		//
		//		order.AuxPrice = roundFloat(order.AuxPrice)
		//		order.Price = roundFloat(order.Price)
		//
		//		res, err = h.ibkrService.Update(ctx, order)
		//		if err != nil {
		//			fmt.Errorf("cannot place order for %s: %s", order.Ticker, res[0].OrderStatus)
		//		}
		//	}
		//}
	}

	var tickers []string
	for _, order := range request.Orders {
		tickers = append(tickers, order.Ticker)
	}

	h.success(ctx, w, r, tickers, responses)
}
func (h Ibkr) Contracts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request model.UpdateContract
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: bad request", err))
		return
	}
	orders, err := h.ibkrService.GetAll(ctx, nil)
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())

		return
	}

	var order model.OrderDetails
	for _, o := range orders.Orders {
		if request.OrderID == o.OrderId {
			order = o
			break
		}
	}

	if order.Ticker == "" {
		Errors(w, r, http.StatusInternalServerError, "no order found")

		return

	}

	order.Price = fmt.Sprintf("%f", request.Price)
	if request.Quantity > 0 {
		order.TotalSize = float64(request.Quantity)
	}
	equity, err := h.equityAmount(ctx)
	if err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

		return
	}
	if err := validateNotional(equity, order.TotalSize, request.Price); err != nil {
		Errors(w, r, http.StatusBadRequest, err.Error())

		return
	}
	responses, err := h.updateContract(ctx, order)
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())

		return
	}

	h.success(ctx, w, r, []string{order.Ticker}, responses)
}

func (h Ibkr) Merge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request model.UpdateContract
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: bad request", err))
		return
	}
	orders, err := h.ibkrService.GetAll(ctx, map[string]interface{}{"tickers": []string{request.Ticker}})
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())

		return
	}

	currentOrders := orders.Orders
	sort.Slice(currentOrders, func(i, j int) bool {
		return currentOrders[i].OrderId < currentOrders[j].OrderId
	})

	totalQuantity := 0.0
	for _, o := range currentOrders {
		if o.OrigOrderType == "LIMIT" {
			totalQuantity += o.RemainingQuantity
		}
	}
	if totalQuantity == 0.0 {
		Errors(w, r, http.StatusInternalServerError, "no quantity to update")

		return
	}

	if len(currentOrders) < 2 {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("need at least 2 open orders to merge, got %d", len(currentOrders)))

		return
	}

	if len(currentOrders) > 2 {
		for i := 2; i < len(currentOrders); i++ {
			_, err := h.ibkrService.Cancel(ctx, request.Ticker, fmt.Sprintf("%d", currentOrders[i].OrderId))
			if err != nil {
				Errors(w, r, http.StatusInternalServerError, err.Error())

				return
			}
		}
	}

	responses := make([]model.OrderResponse, 0)
	for i := 0; i < 2; i++ {
		if currentOrders[i].OrigOrderType == "LIMIT" {
			if request.Target > 0 {
				currentOrders[i].Price = fmt.Sprintf("%0.2f", request.Target)
			}
		} else {
			if request.StopLoss > 0 {
				currentOrders[i].Price = fmt.Sprintf("%0.2f", request.StopLoss)
			}
		}
		currentOrders[i].TotalSize = totalQuantity
		res, err := h.updateContract(ctx, currentOrders[i])
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, err.Error())

			return
		}
		responses = append(responses, res...)
	}

	h.success(ctx, w, r, []string{request.Ticker}, responses)
}

func (h Ibkr) updateContract(ctx context.Context, order model.OrderDetails) ([]model.OrderResponse, error) {
	var responses []model.OrderResponse
	updateOrder := model.Order{}

	updateOrder.ConID = order.Conid
	updateOrder.AcctID = order.Account
	updateOrder.OrderId = &order.OrderId

	updateOrder.TIF = order.TimeInForce
	if updateOrder.TIF == "" {
		updateOrder.TIF = "GTC"
	}

	updateOrder.Price, _ = strconv.ParseFloat(order.Price, 64)
	updateOrder.Quantity = int(order.TotalSize)
	updateOrder.Ticker = order.Ticker

	if order.OrderType == "Stop" {
		updateOrder.OrderType = model.Stop
		updateOrder.Side = order.Side
	}
	if order.OrderType == "Limit" {
		updateOrder.OrderType = model.Limit
		updateOrder.Side = order.Side
	}
	if order.OrderType == "Stop Limit" {
		updateOrder.OrderType = model.StopLimit
		updateOrder.AuxPrice = updateOrder.Price
		updateOrder.Price = updateOrder.AuxPrice + updateOrder.AuxPrice*viper.GetFloat64("MARGIN")
		updateOrder.Side = order.Side
		updateOrder.CoID = order.OrderRef
	}
	updateOrder.Price = roundFloat(updateOrder.Price)
	updateOrder.AuxPrice = roundFloat(updateOrder.AuxPrice)

	res, err := h.ibkrService.Update(ctx, updateOrder)
	if err != nil {
		return nil, err
	}

	responses = append(responses, res)
	return responses, nil
}

func (h Ibkr) success(ctx context.Context, w http.ResponseWriter, r *http.Request, tickers []string, responses []model.OrderResponse) {
	outstandingOrders, _ := h.ibkrService.GetAll(ctx, map[string]interface{}{
		"tickers": tickers,
	})

	activeOrders := make([]model.OrderDetails, 0)
	if len(outstandingOrders.Orders) == 0 {
		outstandingOrders, _ = h.ibkrService.GetAll(ctx, map[string]interface{}{
			"tickers": tickers,
		})
	}
	for _, order := range outstandingOrders.Orders {
		if order.Status != "Cancelled" {
			activeOrders = append(activeOrders, order)
		}
	}

	Success(w, r, map[string]interface{}{
		"responses": responses[0],
		"orders":    model.AllOrders{Orders: activeOrders},
	})
}

func (h Ibkr) Search(w http.ResponseWriter, r *http.Request) {
	symbol := h.paramsForGetRequest(r)

	data, err := h.ibkrService.Search(r.Context(), symbol)
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, r, data)
}

func (h Ibkr) Security(w http.ResponseWriter, r *http.Request) {
	positions, _, _, err := h.positionService.GetAll(r.Context())
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	contractIDs := make([]string, 0)
	for _, position := range positions {
		contractIDs = append(contractIDs, fmt.Sprintf("%d", position.Conid))
	}
	contracts, err := h.ibkrService.Security(r.Context(), contractIDs)
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	contractsBySector := make(map[string]map[string][]string)
	for _, contract := range contracts {
		if contract.Sector == "" {
			contract.Sector = "ETF"
		}
		if contract.SectorGroup == "" {
			contract.SectorGroup = "ETF"
		}
		if _, ok := contractsBySector[contract.Sector]; ok {
			if _, ok := contractsBySector[contract.Sector][contract.SectorGroup]; ok {
				if !isContains(contract.Ticker, contractsBySector[contract.Sector][contract.SectorGroup]) {
					contractsBySector[contract.Sector][contract.SectorGroup] = append(contractsBySector[contract.Sector][contract.SectorGroup], contract.Ticker)
				}
			} else {
				contractsBySector[contract.Sector][contract.SectorGroup] = []string{contract.Ticker}
			}
			contractsBySector[contract.Sector]["All"] = append(contractsBySector[contract.Sector]["All"], contract.Ticker)
		} else {
			contractsBySector[contract.Sector] = map[string][]string{
				contract.SectorGroup: []string{contract.Ticker},
			}
			contractsBySector[contract.Sector]["All"] = []string{contract.Ticker}
		}
	}

	for sector, subSectors := range contractsBySector {
		count := 0
		for _, tickers := range subSectors {
			count += len(tickers)
		}
		contractsBySector[sector]["All"] = []string{
			strings.Join(contractsBySector[sector]["All"], ","),
			fmt.Sprintf("%.2d", 100*count/len(positions)),
		}
	}

	Success(w, r, contractsBySector)
}

func (h Ibkr) Cancel(w http.ResponseWriter, r *http.Request) {
	symbol := h.pathParamsForGetRequest(r)
	if symbol == "" {
		Errors(w, r, http.StatusInternalServerError, "invalid ticker")

		return
	}

	orderID := r.URL.Query().Get("orderID")

	data, err := h.ibkrService.Cancel(r.Context(), symbol, orderID)
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())

		return
	}

	Success(w, r, data)
}

func (h Ibkr) Alert(w http.ResponseWriter, r *http.Request) {
	var request model.Alert
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		fmt.Errorf("%v: bad request", err)
		Errors(w, r, http.StatusBadRequest, err.Error())

		return
	}

	data, err := h.ibkrService.Search(r.Context(), request.Ticker)
	if err != nil {
		Errors(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if len(data) == 0 {
		Errors(w, r, http.StatusBadRequest, "ticker not found")
		return
	}

	tickerData := data[0]
	for _, d := range data {
		if strings.Contains(d.Description, "NASDAQ") || strings.Contains(d.Description, "NYSE") || strings.Contains(d.Description, "TSE") {
			tickerData = d
			break
		}
	}
	if request.Exchange != "" {
		for _, ticker := range data {
			if ticker.Description == request.Exchange {
				tickerData = ticker
			}
		}
	}

	conditions := strings.Split(*request.Condition, " ")
	request.Conditions = []model.Conditions{{
		Conidex:       fmt.Sprintf("%s@%s", tickerData.ConID, tickerData.Description),
		LogicBind:     "n",
		Operator:      conditions[0],
		TriggerMethod: "0",
		Type:          1,
		Value:         conditions[1],
	}}

	request.Condition = nil

	res, err := h.alertService.Create(r.Context(), request)
	if err != nil {
		Errors(w, r, http.StatusInternalServerError, err.Error())

		return
	}

	Success(w, r, res)
}

func (h Ibkr) pathParamsForGetRequest(r *http.Request) string {
	symbol := strings.TrimSpace(chi.URLParam(r, "symbol"))

	return symbol
}

func (h Ibkr) paramsForGetRequest(r *http.Request) string {
	symbol := r.URL.Query().Get("symbol")

	return symbol
}

func getPrices(prices string) (float64, float64, float64, float64) {
	values := strings.Split(prices, "/")
	var floats []float64
	for _, value := range values {
		floatValue, _ := strconv.ParseFloat(value, 64)
		floats = append(floats, floatValue)
	}

	if len(floats) == 1 {
		return floats[0], 0, 0, 0
	}

	if len(floats) > 3 {
		return floats[0], floats[1], floats[2], floats[3]
	}
	if len(floats) > 2 {
		return floats[0], floats[1], floats[2], 0
	}

	return floats[0], floats[1], 0, 0
}

const (
	maxRiskFraction          = 0.01  // max stop risk per trade
	minStopFraction          = 0.005 // min |entry-stop|/entry (blocks tiny-stop fat fingers)
	maxNotionalFraction      = 0.25  // max |qty|*entry / equity
	maxPortfolioRiskFraction = 0.15  // max open stop risk + this trade
	quantityMatchTolerance   = 0.10  // given vs formula-derived qty
)

func deriveQuantity(equity, position, entry, stop float64) int {
	stopDist := math.Abs(entry - stop)
	if equity <= 0 || position <= 0 || stopDist <= 0 {
		return 0
	}
	return int(math.Round((equity * position / 100) / stopDist))
}

// resolveQuantity uses the formula-derived size when given is 0; otherwise requires
// |given - derived| / derived <= 10% to catch fat-finger qty (e.g. clipboard paste).
func resolveQuantity(given int, equity, position, entry, stop float64) (int, error) {
	derived := deriveQuantity(equity, position, entry, stop)
	if derived <= 0 {
		return 0, fmt.Errorf(
			"cannot derive quantity: equity=%0.2f position=%0.2f entry=%0.2f stop=%0.2f",
			equity, position, entry, stop,
		)
	}
	if given == 0 {
		return derived, nil
	}
	givenAbs := math.Abs(float64(given))
	delta := math.Abs(givenAbs-float64(derived)) / float64(derived)
	if delta > quantityMatchTolerance {
		return 0, fmt.Errorf(
			"quantity mismatch: given=%d derived=%d (delta %0.1f%% > %0.0f%%); equity=%0.0f position=%0.2f entry=%0.2f stop=%0.2f",
			given, derived, 100*delta, 100*quantityMatchTolerance, equity, position, entry, stop,
		)
	}
	return given, nil
}

// validateTradeRisk enforces per-trade stop distance, stop risk, and notional caps.
func validateTradeRisk(equity float64, quantity int, entry, stop float64) error {
	if equity <= 0 {
		return fmt.Errorf("cannot validate risk: equity is %0.2f", equity)
	}
	if entry <= 0 {
		return fmt.Errorf("cannot validate risk: invalid entry %0.2f", entry)
	}
	qty := math.Abs(float64(quantity))
	if qty <= 0 {
		return fmt.Errorf("cannot validate risk: quantity is %d", quantity)
	}
	stopDist := math.Abs(entry - stop)
	if stopDist <= 0 {
		return fmt.Errorf("cannot validate risk: entry and stop are equal (%0.2f)", entry)
	}
	stopPct := stopDist / entry
	if stopPct < minStopFraction {
		return fmt.Errorf(
			"stop too tight: %0.2f%% of entry (min %0.2f%%); entry=%0.2f stop=%0.2f",
			100*stopPct, 100*minStopFraction, entry, stop,
		)
	}
	risk := qty * stopDist
	maxRisk := equity * maxRiskFraction
	if risk > maxRisk {
		return fmt.Errorf(
			"risk too large: qty=%d risk=%0.2f exceeds 1%% of equity (%0.2f); entry=%0.2f stop=%0.2f",
			quantity, risk, maxRisk, entry, stop,
		)
	}
	notional := qty * entry
	maxNotional := equity * maxNotionalFraction
	if notional > maxNotional {
		return fmt.Errorf(
			"notional too large: qty=%d notional=%0.2f exceeds %0.0f%% of equity (%0.2f); entry=%0.2f",
			quantity, notional, 100*maxNotionalFraction, maxNotional, entry,
		)
	}
	return nil
}

// validateMaxRisk is kept as an alias for older call sites/tests.
func validateMaxRisk(equity float64, quantity int, entry, stop float64) error {
	return validateTradeRisk(equity, quantity, entry, stop)
}

func (h Ibkr) equityAmount(ctx context.Context) (float64, error) {
	summary, err := h.positionService.Summary(ctx)
	if err != nil {
		return 0, err
	}
	totalEquity := summary.EquityWithLoanValue.Amount
	if h.totalEquity > 0 {
		totalEquity = h.totalEquity
	}
	return totalEquity, nil
}

func (h Ibkr) enforceTradeRisk(ctx context.Context, equity float64, quantity int, entry, stop float64) error {
	if err := validateTradeRisk(equity, quantity, entry, stop); err != nil {
		return err
	}
	_, _, _, _, _, _, openRisk, _, err := h.getOrders(ctx)
	if err != nil {
		return fmt.Errorf("cannot validate portfolio risk: %w", err)
	}
	newRisk := math.Abs(float64(quantity)) * math.Abs(entry-stop)
	maxPortfolio := equity * maxPortfolioRiskFraction
	if openRisk+newRisk > maxPortfolio {
		return fmt.Errorf(
			"portfolio risk too large: open=%0.2f + new=%0.2f exceeds %0.0f%% of equity (%0.2f)",
			openRisk, newRisk, 100*maxPortfolioRiskFraction, maxPortfolio,
		)
	}
	return nil
}

func validateNotional(equity float64, quantity float64, price float64) error {
	if equity <= 0 {
		return fmt.Errorf("cannot validate notional: equity is %0.2f", equity)
	}
	if price <= 0 || quantity == 0 {
		return nil
	}
	notional := math.Abs(quantity) * price
	maxNotional := equity * maxNotionalFraction
	if notional > maxNotional {
		return fmt.Errorf(
			"notional too large: qty=%0.0f notional=%0.2f exceeds %0.0f%% of equity (%0.2f); price=%0.2f",
			quantity, notional, 100*maxNotionalFraction, maxNotional, price,
		)
	}
	return nil
}

func roundFloat(value float64) float64 {
	strValue := fmt.Sprintf("%.2f", value)
	floatValue, _ := strconv.ParseFloat(strValue, 64)

	return math.Round(floatValue*100) / 100
}

func isContains(value interface{}, values interface{}) bool {
	list := values.([]string)
	for _, v := range list {
		if v == value {
			return true
		}
	}

	return false
}

func extractTargetPercentage(order string) float64 {
	re := regexp.MustCompile(`Target [\d.]+ (-?\d+\.\d+)%`)
	matches := re.FindStringSubmatch(order)
	if len(matches) > 1 {
		percentage, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			return percentage
		}
	}
	return 0.0 // default if extraction fails
}
