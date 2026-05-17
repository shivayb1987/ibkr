package handler

import (
	"context"
	"encoding/json"
	"errors"
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
	orders, symbols, totalOrderAmount, pendingOrders, workingOrders, potentialLoss, potentialProfit, err := h.getOrders(r.Context())
	if err != nil {
		Errors(w, r, 500, err.Error())
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
		"totalOrderAmount":     totalOrderAmount,
		"totalPotentialLoss":   potentialLoss,
		"totalPotentialProfit": potentialProfit,
		"zzCoreEquity":         roundFloat(coreEquity),
	})
}

func (h Ibkr) getOrders(ctx context.Context) ([]model.ListOrder, []string, float64, []string, []string, float64, float64, error) {
	var wg sync.WaitGroup
	workingOrdersByStock := make(map[string]map[string]float64)
	liveTickersReference := make(map[string]bool)
	var orders []model.ListOrder
	var symbols []string
	var err error
	var totalOrderAmount float64
	var pendingOrders []string

	wg.Add(2)

	go func() {
		defer wg.Done()

		orders, symbols, err = h.ibkrService.ListOrders(ctx)
		if err != nil {
			return
		}

	}()

	go func() {
		defer wg.Done()

		_, liveTickers, _, _ := h.positionService.GetAll(ctx)
		for _, ticker := range liveTickers {
			liveTickersReference[ticker] = true
		}
	}()
	wg.Wait()

	var wg2 sync.WaitGroup
	var m sync.RWMutex
	pendingOrders = make([]string, 0, len(symbols))

	wg2.Add(len(orders))
	for _, order := range orders {
		go func(order model.ListOrder) {
			defer wg2.Done()
			result, _ := h.yahooService.GetStock(order.Ticker, "1d", "5d")
			if len(result.Indicators.Quote) == 0 || len(result.Indicators.Quote[0].Close) == 0 {
				result, _ = h.yahooService.GetStock(fmt.Sprintf("%s.TO", order.Ticker), "1d", "5d")
			}
			m.Lock()
			if len(result.Indicators.Quote) > 0 {
				closes := result.Indicators.Quote[0].Close
				if len(closes) == 0 {
					return
				}
				close := closes[len(closes)-1]

				if _, ok := workingOrdersByStock[order.Ticker]; !ok {
					workingOrdersByStock[order.Ticker] = make(map[string]float64)
				}
				workingOrdersByStock[order.Ticker]["Close"] = close

				position := 0.0
				stopLoss := 0.0
				if order.OrderType == "Stop" {
					strValue := strings.Split(order.OrderDesc, " ")[4]
					strPos := strings.Split(order.OrderDesc, " ")[1]
					stopLoss, err = strconv.ParseFloat(strings.Replace(strValue, ",", "", 1), 64)
					if err != nil {
						fmt.Errorf("error: %w", err)
					}
					position, err = strconv.ParseFloat(strings.Replace(strPos, ",", "", 1), 64)
					if err != nil {
						fmt.Errorf("error: %w", err)
					}
				}

				target := 0.0
				if order.OrderType == "Limit" {
					target = order.Price
					strPos := strings.Split(order.OrderDesc, " ")[1]
					position, err = strconv.ParseFloat(strings.Replace(strPos, ",", "", 1), 64)
					if err != nil {
						fmt.Errorf("error: %w", err)
					}
				}

				workingOrdersByStock[order.Ticker]["Position"] = position
				stopLossPercent := 0.0
				cadusd := h.getFX("CADUSD")
				if stopLoss > 0 {
					stopLossPercent = (close / stopLoss) - 1
					workingOrdersByStock[order.Ticker]["StopLoss"] = stopLoss
					workingOrdersByStock[order.Ticker]["SL"] = stopLossPercent
					workingOrdersByStock[order.Ticker]["PotentialLoss"] = math.Abs(close-stopLoss) * position
				}

				targetPercent := 0.0
				if target > 0 {
					targetPercent = (target / close) - 1
					workingOrdersByStock[order.Ticker]["Target"] = targetPercent
					if order.CashCcy == "CAD" {
						workingOrdersByStock[order.Ticker]["PotentialProfit"] = math.Abs(target-close) * position * cadusd
					} else {
						workingOrdersByStock[order.Ticker]["PotentialProfit"] = math.Abs(target-close) * position
					}
				}
			}
			if strings.Contains(order.OrderDesc, "Buy") && strings.Contains(order.OrderDesc, "Stop") && strings.Contains(order.OrderDesc, "LMT") && order.Status != "Filled" {
				totalOrderAmount += order.Price * order.Quantity
				pendingOrders = append(pendingOrders, order.Ticker)
			}
			if strings.Contains(order.OrderDesc, "Sell") && strings.Contains(order.OrderDesc, "Stop") && strings.Contains(order.OrderDesc, "LMT") && order.Status != "Filled" {
				totalOrderAmount -= order.Price * order.Quantity
				pendingOrders = append(pendingOrders, order.Ticker)
			}
			m.Unlock()
		}(order)
	}
	//h.fileService.Write("Orders", strings.Join(pendingOrders, ","))
	wg2.Wait()

	if len(symbols) == 0 {
		return nil, nil, 0, nil, nil, 0, 0, errors.New("cannot get orders")
	}

	workingOrders := make([]string, 0, len(symbols))
	potentialLoss := 0.0
	potentialProfit := 0.0
	for ticker, prices := range workingOrdersByStock {
		if _, ok := liveTickersReference[ticker]; ok {
			workingOrder := fmt.Sprintf("%s SL %.2f %.2f%% Target %.2f %.2f%%, (%0.2f/%0.2f)", ticker, prices["StopLoss"], 100*prices["SL"], (1+prices["Target"])*prices["Close"], 100*prices["Target"], prices["PotentialLoss"], prices["Position"]*prices["Close"])
			workingOrders = append(workingOrders, workingOrder)
			potentialLoss += prices["PotentialLoss"]
			potentialProfit += prices["PotentialProfit"]
		}

	}

	sort.Slice(workingOrders, func(i, j int) bool {
		return math.Abs(extractTargetPercentage(workingOrders[i])) < math.Abs(extractTargetPercentage(workingOrders[j]))

	})
	return orders, symbols, totalOrderAmount, pendingOrders, workingOrders, potentialLoss, potentialProfit, nil
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

			if order.Quantity == 0 {
				summary, err := h.positionService.Summary(ctx)
				if err != nil {
					Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

					return
				}

				if stoploss <= 0 {
					stoploss = 0.92 * buy
				}

				totalEquity := summary.EquityWithLoanValue.Amount
				order.Quantity = int(math.Round((totalEquity * position / 100) / math.Abs((buy - stoploss))))
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
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("no positions"))

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
	for _, symbol := range symbols {
		isCoveringShort := false
		pos, ok := positionsByTicker[symbol]
		if !ok {
			continue
		}
		var order model.Order
		order.Ticker = pos.ContractDesc
		order.Quantity = int(pos.Position * size)
		if pos.Position < 0 {
			order.Quantity = -int(pos.Position * size)
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
			//continue
		}
		result, _ := h.yahooService.GetStock(yahooSymbol, "1d", "1w")
		quote := result.Indicators.Quote
		if len(quote) == 0 || len(quote[0].Close) == 0 {
			result1, err := h.ibdService.GetStockQuotes(context.Background(), symbol, time.Now())
			if err != nil || len(result1) == 0 {
				Errors(w, r, http.StatusInternalServerError, err.Error())
			}
			quote = result1
		}

		closes := quote[0].Close
		if isCoveringShort {
			order.Price = roundFloat(closes[len(closes)-1] * 1.005)
		} else {
			order.Price = roundFloat(closes[len(closes)-1] * 0.995)
		}
		currentOrders, err := h.ibkrService.GetAll(ctx, map[string]interface{}{
			"tickers": []string{symbol},
		})
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot get order for %s: %s", symbol, err.Error()))
			return
		}

		if len(currentOrders.Orders) > 0 {
			currentLimitPosition := 0.0
			for _, currentOrder := range currentOrders.Orders {
				if currentOrder.OrderType == "Limit" {
					currentLimitPosition += currentOrder.TotalSize
				}
				if currentOrder.OrderType == "Stop" {
					currentLimitPosition -= currentOrder.TotalSize
				}
			}

			if int(currentLimitPosition) > order.Quantity {
				Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("exceeds current position %s: (%d)", order.Ticker, int(currentLimitPosition)))
				return
			}
		}

		res, err := h.ibkrService.Order(ctx, []model.Order{order})
		if err != nil {
			Errors(w, r, http.StatusInternalServerError, fmt.Sprintf("cannot place order for %s: %s", order.Ticker, err.Error()))
			return
		}

		responses = append(responses, res)
		contractResponse, _ := h.updatePosition(ctx, currentOrders, order)
		responses = append(responses, contractResponse...)
	}

	h.success(ctx, w, r, symbols, responses)
}

func (h Ibkr) updatePosition(ctx context.Context, orders model.AllOrders, order model.Order) ([]model.OrderResponse, error) {
	var contractOrder model.OrderDetails
	responses := make([]model.OrderResponse, 0)
	for _, o := range orders.Orders {
		if order.ConID == o.Conid {
			contractOrder = o

			if contractOrder.Ticker == "" {
				return nil, fmt.Errorf("no order found")

			}

			if o.Price != "" {
				contractOrder.Price = o.Price
			}
			if o.StopPrice != "" {
				contractOrder.Price = o.StopPrice
			}

			contractOrder.TotalSize = float64(order.Quantity)
			contractResponse, err := h.updateContract(ctx, contractOrder)
			if err != nil {
				return nil, err
			}
			responses = append(responses, contractResponse...)
		}
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
	totalEquity := h.totalEquity
	if h.totalEquity <= 0 {
		summary, err := h.positionService.Summary(ctx)
		if err != nil {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

			return
		}

		//sgdusd := h.getFX("SGDUSD")
		totalEquity = summary.EquityWithLoanValue.Amount
	}
	_, _, _, _, _, potentialLoss, _, err := h.getOrders(r.Context())
	if err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get working orders", err.Error()))

		return
	}

	coreEquity := totalEquity - potentialLoss

	size, err := strconv.ParseFloat(r.URL.Query().Get("position"), 64)
	if err != nil {
		size = 1.0
	}

	target, _ := strconv.ParseFloat(targetStr, 64)

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
			//continue
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
			fmt.Printf("cannot get price")
			quotes, err = h.ibdService.GetStockQuotes(context.Background(), symbol, time.Now())
			if err != nil {
				Errors(w, r, http.StatusInternalServerError, err.Error())

				return
			}

		}

		closes := quotes[0].Close
		buy := roundFloat(closes[len(closes)-1] * 1.003)

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

		// buy
		order.CoID = fmt.Sprintf("Parent_%s_%s", order.Ticker, time.Now().Format("2006-01-02 15:04:05"))
		order.AcctID = viper.GetString("ACCOUNT_ID")
		order.Side = "BUY"
		order.OrderType = model.Limit
		order.Side = "BUY"
		order.TIF = "GTC"
		order.Quantity = int(math.Round(positionSize / (buy - stoploss)))
		//order.AuxPrice = roundFloat(buy) // stop
		//order.Price = roundFloat(order.AuxPrice + order.AuxPrice*viper.GetFloat64("MARGIN"))
		order.Price = roundFloat(buy)

		// target
		takeProfit := order
		takeProfit.ParentID = order.CoID
		takeProfit.Side = "SELL"
		takeProfit.OrderType = "LMT"
		takeProfit.Price = target
		takeProfit.CoID = ""

		// stop
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

		//res[0].Total = float64(order.Quantity) * order.Price

		responses = append(responses, res)
	}

	h.success(ctx, w, r, symbols, responses)
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

	totalEquity := h.totalEquity
	if h.totalEquity <= 0 {
		summary, err := h.positionService.Summary(ctx)
		if err != nil {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

			return
		}

		//sgdusd := h.getFX("SGDUSD")
		totalEquity = summary.EquityWithLoanValue.Amount
	}
	//_, _, _, _, _, potentialLoss, _, err := h.getOrders(r.Context())
	//if err != nil {
	//	Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get working orders", err.Error()))
	//
	//	return
	//}

	coreEquity := totalEquity - 0

	size, err := strconv.ParseFloat(r.URL.Query().Get("position"), 64)
	if err != nil {
		size = 1.0
	}

	positionSize := coreEquity * 0.01 * size
	quantity := 0.0
	total := 0.0
	stopLoss := 0.0

	symbolsChecklist := make(map[string]string)
	for _, s := range h.allSymbols {
		symbolsChecklist[strings.Split(s, ".")[0]] = s
	}

	symbols := r.URL.Query()["symbol"]
	for _, symbol := range symbols {
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
		yahooSymbol, ok := symbolsChecklist[strings.Replace(symbol, ".", "-", -1)]
		if !ok {
			yahooSymbol = symbol
			//continue
		}
		yahooSymbol = strings.Replace(strings.Replace(yahooSymbol, ".NYSE", "", -1), ".NASDAQ", "", -1)
		tickersYahoo := map[string]string{
			"U.UN": "U-UN.TO",
			"U-UN": "U-UN.TO",
		}
		ibkrTicker, ok := tickersYahoo[yahooSymbol]
		if ok {
			yahooSymbol = ibkrTicker
		}

		result, _ := h.yahooService.GetStock(yahooSymbol, "1d", "1y")
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

		buy := roundFloat(closingPrice * 1.003)
		if entry > 0 {
			buy = roundFloat(entry)
		}
		atr := h.yahooService.ATR(quotes[0], 22)
		sl := 3 * atr
		stopLoss = roundFloat(buy - sl)
		if stopLossParam != "" {
			sl, _ = strconv.ParseFloat(stopLossParam, 64)
			if strings.ToLower(side) == "sell" {
				stopLoss = roundFloat(buy * (1 + (sl / 100)))
			} else {
				stopLoss = roundFloat(buy * (1 - (sl / 100)))
			}
		}

		if strings.ToLower(side) == "sell" {
			quantity = math.Round(positionSize / (stopLoss - buy))
		} else {
			quantity = math.Round(positionSize / (buy - stopLoss))
		}
		total = quantity * buy

	}

	Success(w, r, map[string]interface{}{
		"quantity": quantity,
		"position": fmt.Sprintf("Amount: %0.2f, loss %0.2f, 1%% loss %0.2f stopLevel: %0.2f (%0.2f%%) ", total, positionSize, total*0.01, stopLoss, 100*positionSize/total),
		"record":   fmt.Sprintf("recAll %s %0.0f %0.2f %s", symbols[0], math.Round(quantity), total/quantity, time.Now().Format("2006/01/02")),
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
	_, _, _, _, _, potentialLoss, _, err := h.getOrders(r.Context())
	if err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get working orders", err.Error()))

		return
	}

	coreEquity := totalEquity - potentialLoss

	size, err := strconv.ParseFloat(r.URL.Query().Get("position"), 64)
	if err != nil {
		size = 1.0
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
		buy := roundFloat(closes[len(closes)-1] * 0.997)
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
		if order.Position > 0 {
			position = order.Position
		}
		if position > 1 {
			Errors(w, r, http.StatusBadRequest, fmt.Sprintf("position too large: %f", position))

			return
		}

		if order.Quantity == 0 {
			summary, err := h.positionService.Summary(ctx)
			if err != nil {
				Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

				return
			}

			//sgdusd := h.getFX("SGDUSD")
			totalEquity := summary.EquityWithLoanValue.Amount
			order.Quantity = int(math.Round((totalEquity * position / 100) / math.Abs((buy - stopLoss))))
		}

		if target == 0 {
			target = buy * 1.30
		}

		if order.Marketprice {
			symbolsChecklist := make(map[string]string)
			for _, s := range h.allSymbols {
				symbolsChecklist[strings.Split(s, ".")[0]] = s
			}
			yahooSymbol, ok := symbolsChecklist[strings.Replace(order.Ticker, ".", "-", -1)]
			if !ok {
				yahooSymbol = order.Ticker
				//continue
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
				closingPrice := closes[len(closes)-1]

				buy = closingPrice
			}
		}
		if order.ConID == 0 {
			order.ConID = int(conID)
		}
		order.AcctID = viper.GetString("ACCOUNT_ID")
		order.CoID = fmt.Sprintf("Parent_%s_%s", order.Ticker, time.Now().Format("2006-01-02 15:04:05"))

		if order.TIF == "" {
			order.TIF = "GTC"
		}

		var halfQty1, halfQty2 int
		halfQty1 = int(order.Quantity / 2)
		halfQty2 = order.Quantity - halfQty1

		takeProfit := order
		takeProfit.ParentID = order.CoID
		takeProfit.Side = "SELL"
		takeProfit.OrderType = "LMT"
		takeProfit.Price = target
		takeProfit.CoID = ""

		takeProfit2 := takeProfit
		if target2 > target {
			takeProfit2.ParentID = ""
			takeProfit2.Price = target2
			takeProfit2.Quantity = halfQty2
			takeProfit.Quantity = halfQty1
		}

		stopLossOrder := order
		stopLossOrder.ParentID = order.CoID
		stopLossOrder.Side = "SELL"
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

				stopLossOrder.Side = "BUY"
				takeProfit.Price = buy * 0.8
				takeProfit.Side = "BUY"
				takeProfit2.Side = "BUY"
			}
		}
		if order.OrderType == model.Limit {
			if order.Side == "BUY" {
				order.Price = buy
			} else {
				order.Price = buy
			}
		}

		order.AuxPrice = roundFloat(order.AuxPrice)
		order.Price = roundFloat(order.Price)
		stopLossOrder.Price = roundFloat(stopLossOrder.Price)
		takeProfit.Price = roundFloat(takeProfit.Price)
		takeProfit2.Price = roundFloat(takeProfit2.Price)

		var res model.OrderResponse
		//if target2 > target {
		//	res, err = h.ibkrService.Order(ctx, []model.Order{order, takeProfit, stopLossOrder})
		//} else {
		res, err = h.ibkrService.Order(ctx, []model.Order{order, takeProfit, stopLossOrder})
		//}
		if err != nil {
			fmt.Errorf("cannot place order for %s: %w", order.Ticker, err)
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
		_, stopLoss, target, _ := getPrices(order.Prices)
		if order.ConID == 0 {
			order.ConID = int(conID)
		}
		order.AcctID = viper.GetString("ACCOUNT_ID")
		//parentID := fmt.Sprintf("Parent_%s_%s", order.Ticker, time.Now().Format("2006-01-02 15:04:05"))

		if order.TIF == "" {
			order.TIF = "GTC"
		}

		if order.Quantity <= 1 {
			summary, err := h.positionService.Summary(ctx)
			if err != nil {
				Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: cannot get summary", err.Error()))

				return
			}

			//sgdusd := h.getFX("SGDUSD")
			totalEquity := summary.EquityWithLoanValue.Amount
			order.Quantity = int(math.Round((totalEquity * 0.25 / 100) / (order.Price - stopLoss)))
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
