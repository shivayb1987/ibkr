package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"ibkr/model"
	"ibkr/service"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ActionItems        = "Action Items!A2:Z100"
	ActionItemsTargets = "Action Items!R3:Z20"
	Portfolio          = "portfolio!A%d:Z%d"
	Record             = "Records!I%d:AD%d"
	RecordNew          = "New!I%d:AD%d"
	RecordAll          = "All Records!I%d:AD%d"
	IndianRecords      = "India!I%d:AD%d"
	Trades             = "Records!I1:AG5000"
	Shortlist          = "Shortlist!A1:Z500"
)

var sheetMap = map[string]string{
	"actionItems": ActionItems,
	"portfolio":   Portfolio,
	"targets":     ActionItemsTargets,
	"trades":      Trades,
	"shortlist":   Shortlist,
}

type SheetsHandler struct {
	sheetsService SheetService
	fileService   FileService
	yahooService  YahooService
	orderService  IBKRService
	nasdaqService NasdaqService
	ibdService    IBDService
	spreadsheetID string
}

type SheetService interface {
	GetValues(ctx context.Context, spreadsheetID string, readRange string) (model.SheetData, error)
	CreateWatchlist(row model.InsertRow, spreadsheetID string, dataRange string) (model.SheetData, error)
	RecordTrade(req model.RecordTrade, spreadsheetID string, dataRange string) (model.SheetData, error)
	ClearSheet(spreadsheetID string, dataRange string) (string, error)
	RecordActionItems(req model.ActionItem, spreadsheetID string, dataRange string) (model.SheetData, error)
	ShortlistRecord(req model.Shortlist, spreadsheetID string, dataRange string) (model.SheetData, error)
}

type IBDService interface {
	Checkup(ticker string) (model.Checkup, error)
	GetStockQuotes(ctx context.Context, symbol string, currentDate time.Time) ([]model.Quote, error)
}

func NewSheetsHandler(sheetsService SheetService, fileService FileService, orderService IBKRService, nasdaqService NasdaqService, yahooService YahooService, ibdService IBDService, spreadsheetID string) SheetsHandler {
	return SheetsHandler{
		sheetsService: sheetsService,
		fileService:   fileService,
		orderService:  orderService,
		nasdaqService: nasdaqService,
		yahooService:  yahooService,
		ibdService:    ibdService,
		spreadsheetID: spreadsheetID,
	}
}

func (h SheetsHandler) Read(w http.ResponseWriter, r *http.Request) {
	readRange := sheetMap["actionItems"]
	result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, readRange)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	var targets model.SheetData
	var symbols []string
	var values [][]string
	for _, row := range result.Values {
		if len(row) > 0 && row[0] != "" {
			if strings.ToLower(row[0]) == "week" {
				break
			}

			symbols = append(symbols, row[0])
			if len(row) >= 15 {
				row = append(row, fmt.Sprintf("buy %s %s %s", row[0], row[15], row[3]))
			}
			values = append(values, row)
		}
	}

	for _, row := range targets.Values {
		if len(row) > 0 && row[0] != "" {
			symbols = append(symbols, row[0])
			values = append(values, row)
		}
	}

	actionSymbol := r.URL.Query().Get("symbol")
	var filteredValues [][]string
	if actionSymbol != "" {
		for _, row := range values {
			if len(row) > 0 && row[0] == actionSymbol {
				filteredValues = append(filteredValues, row)
			}
		}
	} else {
		filteredValues = values
	}

	attachedOrders, err := h.orderService.GetAll(r.Context(), nil)
	if err != nil {
		//Errors(w, r, 500, err.Error())
		//return
	}

	ordersBySymbol := make(map[string]model.OrderDetails)
	for _, attachedOrder := range attachedOrders.Orders {
		if _, ok := ordersBySymbol[attachedOrder.Ticker]; !ok {
			ordersBySymbol[attachedOrder.Ticker] = attachedOrder
		}
	}

	potentialSymbols := make([]string, 0)
	for _, symbol := range symbols {
		if _, ok := ordersBySymbol[symbol]; !ok {
			potentialSymbols = append(potentialSymbols, symbol)
		}
	}

	h.fileService.Write("ActionItems",
		replaceString(strings.Join(potentialSymbols, ","), [][]string{
			{"TXG", "TSX:TXG"},
			{"IVN", "TSX:IVN"},
			{"U.U", "TSX:U.UN"},
		}),
	)
	Success(w, r, values)
}

func (h SheetsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	readRange := sheetMap["trades"]
	result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, readRange)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	symbolsBySector := make(map[string]map[string]interface{})
	headers := result.Values[0]
	for _, row := range result.Values[1:] {
		if len(row) > 0 && row[getIndex(headers, "Status")] != "" {
			continue
		}

		if len(row) > 0 && row[getIndex(headers, "Ticker")] != "" {
			if _, ok := symbolsBySector[row[getIndex(headers, "Sector")]]; ok {
				if !isContains(row[getIndex(headers, "Ticker")], symbolsBySector[row[getIndex(headers, "Sector")]]["symbols"]) {
					symbolsBySector[row[getIndex(headers, "Sector")]]["symbols"] = append(symbolsBySector[row[getIndex(headers, "Sector")]]["symbols"].([]string), row[getIndex(headers, "Ticker")])
				}
				total, _ := strconv.ParseFloat(strings.Replace(row[getIndex(headers, "Amount")], ",", "", -1), 64)
				symbolsBySector[row[getIndex(headers, "Sector")]]["subTotal"] = roundFloat(symbolsBySector[row[getIndex(headers, "Sector")]]["subTotal"].(float64) + total)
			} else {
				symbolsBySector[row[getIndex(headers, "Sector")]] = make(map[string]interface{})
				symbolsBySector[row[getIndex(headers, "Sector")]]["symbols"] = []string{row[getIndex(headers, "Ticker")]}
				total, _ := strconv.ParseFloat(strings.Replace(row[getIndex(headers, "Amount")], ",", "", -1), 64)
				symbolsBySector[row[getIndex(headers, "Sector")]]["subTotal"] = roundFloat(total)
			}
		}
	}

	Success(w, r, symbolsBySector)
}

func (h SheetsHandler) Shortlist(w http.ResponseWriter, r *http.Request) {
	readRange := sheetMap["shortlist"]
	tickers := r.URL.Query()["ticker"]
	result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, readRange)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	allRecords := make([]interface{}, 0)
	headers := result.Values[0]
	for _, row := range result.Values[1:] {
		symbolsBySector := make(map[string]interface{})
		if len(row) > 0 && row[getIndex(headers, "Symbol")] != "" && (len(tickers) == 0 || isContains(row[getIndex(headers, "Symbol")], tickers)) {
			symbolsBySector["a-Symbol"] = formatString(row[getIndex(headers, "Symbol")], 20)
			symbolsBySector["b-Price"] = formatString(row[getIndex(headers, "Price")], 20)
			symbolsBySector["c-Volume ($m)"] = formatString(row[getIndex(headers, "Volume ($m)")], 20)
			symbolsBySector["d-Marketcap ($m)"] = formatMarketCap(row[getIndex(headers, "Marketcap ($m)")])
			symbolsBySector["e-P/E"] = formatPE(row[getIndex(headers, "P/E")])
			symbolsBySector["f-Composite Rank"] = formatCompositeRank(row[getIndex(headers, "Composite Rank")])
			symbolsBySector["g-EPS 3 x YoY"] = formatString(row[getIndex(headers, "EPS 3 x YoY")], 20)
			symbolsBySector["h-Sales x YoY"] = formatString(row[getIndex(headers, "Saels x YoY")], 20)
			symbolsBySector["i-ROE"] = formatString(formatString(row[getIndex(headers, "ROE")], 20), 20)
			symbolsBySector["j-Gross Margin %"] = formatString(row[getIndex(headers, "Gross Margin %")], 20)
			symbolsBySector["k-EPSRank"] = formatString(row[getIndex(headers, "EPSRank")], 90)
			symbolsBySector["l-GroupRSRating"] = formatString(row[getIndex(headers, "GroupRSRating")], 90)
			symbolsBySector["m-Net Score"] = formatNetScore(row[getIndex(headers, "Net Score")])
			symbolsBySector["z-Position"] = row[getIndex(headers, "Position")]
			allRecords = append(allRecords, symbolsBySector)
		}
	}

	Success(w, r, allRecords)
}

func (h SheetsHandler) GetActionItem(w http.ResponseWriter, r *http.Request) {
	//result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, "Action Items!A1:Z100")
	//if err != nil {
	//	Errors(w, r, 500, err.Error())
	//	return
	//}
	//
	ticker := r.URL.Query().Get("ticker")
	//
	//headers := result.Values[0]
	var summary string
	//for _, row := range result.Values[1:] {
	//	if len(row) > 0 && row[getIndex(headers, "Symbol")] == "" {
	//		break
	//	}
	//
	//	if row[getIndex(headers, "Symbol")] == ticker {
	//		atr := row[getIndex(headers, "ATR")]
	//		insiderActivity := row[getIndex(headers, "Insider Activity (months3+months12)")]
	//		yield := row[getIndex(headers, "Yield")]
	//		surprises := row[getIndex(headers, "Surprises")]
	//		netUpgrades := row[getIndex(headers, "NetUpgrades")]
	//
	//		summary = fmt.Sprintf("atr %s insiders %s surprises %s netUpgrades %s yield %s", atr, insiderActivity, surprises, netUpgrades, yield)
	//		break
	//	}
	//}

	if summary == "" {
		tickerInfo, err := h.nasdaqService.Summary(ticker, "stocks")
		if err != nil {
			Success(w, r, summary)

			return
		}
		var surprisesCount int
		var closingPrice float64
		var atr float64
		var months3 float64
		var months12 float64
		var earningsForecasts []model.EarningsForecast
		wg := sync.WaitGroup{}
		wg.Add(4)
		go func() {
			defer func() {
				wg.Done()
			}()
			_, surprisesCount = h.getSurprises(err, ticker)
		}()
		go func() {
			defer func() {
				wg.Done()
			}()
			closingPrice, atr = h.getAtr(ticker)
		}()
		go func() {
			defer func() {
				wg.Done()
			}()
			months3, months12 = h.getInsiderActivity(ticker)
		}()
		go func() {
			defer func() {
				wg.Done()
			}()
			earningsForecasts, _ = h.nasdaqService.EarningsForecast(ticker)
		}()
		wg.Wait()
		var upForecasts int
		var downForecasts int
		for _, forecast := range earningsForecasts {
			if forecast.Up > 0 {
				upForecasts += forecast.Up
			}

			if forecast.Down > 0 {
				downForecasts -= forecast.Down
			}
		}
		summary = fmt.Sprintf("atr %s ", fmt.Sprintf("%0.0f%%", 100*atr/closingPrice))
		if months12+months3 > 0 {
			summary += fmt.Sprintf("insiders %0.2f ", months3+months12)
		}
		if surprisesCount > 0 {
			summary += fmt.Sprintf("surprises %d ", surprisesCount)
		}
		if upForecasts-downForecasts > 0 {
			summary += fmt.Sprintf("netUpgrades %d ", upForecasts-downForecasts)
		}
		if tickerInfo.SummaryData.Yield.Value != "" && tickerInfo.SummaryData.Yield.Value != "N/A" {
			summary += fmt.Sprintf("yield %s", tickerInfo.SummaryData.Yield.Value)
		}
	}

	Success(w, r, summary)
}

func (h SheetsHandler) Performance(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	perfStartTime, _ := time.Parse(service.DateTimeFormat, since)
	till := r.URL.Query().Get("till")
	perfEndTime, _ := time.Parse(service.DateTimeFormat, till)
	result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, sheetMap["trades"])
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	tickers := r.URL.Query()["ticker"]
	tickersFilter := make(map[string]bool)
	for _, ticker := range tickers {
		tickersFilter[ticker] = true
	}

	headers := result.Values[0]
	performanceByTicker := make(map[string]string)
	totalPnL := 0.0
	totalCost := 0.0
	for rowIdx, row := range result.Values[1:] {
		if len(row) <= 15 || row[getIndex(headers, "Ticker")] == "" {
			break
		}
		fx := extractNum(row[getIndex(headers, "Currency")])
		ticker := row[getIndex(headers, "Ticker")]
		tickersYahoo := map[string]string{
			"U.U": "U-UN",
		}
		if yahooTicker, ok := tickersYahoo[ticker]; ok {
			ticker = yahooTicker
		}
		if fx != 0 && fx != 1.00 {
			ticker = fmt.Sprintf("%s.TO", ticker)
		}

		open := row[getIndex(headers, "Open")]
		entryPriceStr := row[getIndex(headers, "Bought")]
		entryPrice := extractNum(entryPriceStr)
		exitPriceStr := row[getIndex(headers, "Sold")]
		exitPrice := extractNum(exitPriceStr)
		entryTime, _ := time.Parse(service.DateTimeFormat2, open)
		close := row[getIndex(headers, "Close")]
		closeTime, _ := time.Parse(service.DateTimeFormat2, close)
		shares := extractNum(row[getIndex(headers, "Shares")])
		if open == "" {
			continue
		}

		if _, ok := tickersFilter[ticker]; !ok && len(tickers) > 0 {
			continue
		}

		thenTime := perfStartTime
		if perfStartTime.Before(entryTime) || perfStartTime.Equal(entryTime) {
			thenTime = entryTime
		}
		currentTime := time.Now()

		if close != "" {
			if closeTime.Before(perfStartTime) {
				continue
			}
			if closeTime.Before(time.Now()) {
				currentTime = closeTime
			}
		}
		if till != "" && perfStartTime.Before(entryTime) && perfEndTime.Before(entryTime) {
			continue
		}
		if till != "" {
			if perfEndTime.Before(currentTime) {
				currentTime = perfEndTime
			}
		}

		displayTicker := ticker
		if _, ok := performanceByTicker[ticker]; ok {
			displayTicker = fmt.Sprintf("%s_%d", ticker, rowIdx+1)
		}
		//if entryTime.Before(perfStartTime) || open == since {

		result, _ := h.yahooService.GetStock(ticker, "1d", "1y")
		if len(result.Indicators.Quote) > 0 && len(result.Indicators.Quote[0].Close) > 0 {
			quote := result.Indicators.Quote[0]
			closes := quote.Close
			currentPrice := closes[len(closes)-1]

			thenTime = h.getSinceTime(result, thenTime)
			currentTime = h.getSinceTime(result, currentTime)
			for i, timestamp := range result.Timestamps {
				val, err := strconv.ParseInt(fmt.Sprintf("%d", timestamp), 10, 64)
				if err != nil {
					panic(err)
				}
				tm := time.Unix(val, 0)
				if tm.Format(service.DateTimeFormat) == currentTime.Format(service.DateTimeFormat) {
					currentPrice = closes[i]
					if exitPriceStr != "" && (till == "" || perfEndTime.After(currentTime) || closeTime.Format(service.DateTimeFormat) == perfEndTime.Format(service.DateTimeFormat)) {
						currentPrice = exitPrice
					}
				}
			}

			for i, timestamp := range result.Timestamps {
				val, err := strconv.ParseInt(fmt.Sprintf("%d", timestamp), 10, 64)
				if err != nil {
					panic(err)
				}
				tm := time.Unix(val, 0)
				if tm.Format(service.DateTimeFormat) == thenTime.Format(service.DateTimeFormat) {
					thenPrice := closes[i-1]
					if perfStartTime.Before(entryTime) || perfStartTime.Equal(entryTime) {
						thenPrice = entryPrice
					}
					totalPnL += shares * (currentPrice - thenPrice) * fx
					totalCost += shares * thenPrice * fx

					if shares*(currentPrice-thenPrice) >= 0 {
						performanceByTicker[displayTicker] = fmt.Sprintf("Green %0.2f :%0.2f NC", 100*((currentPrice/thenPrice)-1), shares*(currentPrice-thenPrice))
					} else {
						performanceByTicker[displayTicker] = fmt.Sprintf("Red %0.2f :%0.2f NC", 100*((currentPrice/thenPrice)-1), shares*(currentPrice-thenPrice))
					}
				}
			}
		}
	}
	//}

	Success(w, r, map[string]interface{}{
		"totalPnL": fmt.Sprintf("%0.2f (%0.2f) :%0.2f%%", roundFloat(totalPnL), totalCost, 100*totalPnL/totalCost),
		"data":     performanceByTicker,
	})
}

func (h SheetsHandler) Analyse(w http.ResponseWriter, r *http.Request) {
	percentStr := r.URL.Query().Get("percent")
	side := r.URL.Query().Get("side")
	if percentStr == "" {
		//Success(w, r, map[string]interface{}{})
		//return
	}
	percent := extractNum(r.URL.Query().Get("percent"))
	multiple := extractNum(r.URL.Query().Get("multiple"))
	if multiple == 0 {
		multiple = 2
	}

	atrMultiple := extractNum(r.URL.Query().Get("atr"))
	if atrMultiple == 0 {
		atrMultiple = 2
	}
	result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, sheetMap["trades"])
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	tickers := r.URL.Query()["ticker"]
	tickersFilter := make(map[string]bool)
	for _, ticker := range tickers {
		tickersFilter[ticker] = true
	}

	headers := result.Values[0]
	performanceByTicker := make(map[string]string)
	countTracker := make(map[string]int)
	totalPnL := 0.0
	for rowIdx, row := range result.Values[1:] {
		if len(row) <= 15 || row[getIndex(headers, "Ticker")] == "" {
			break
		}
		fx := extractNum(row[getIndex(headers, "Currency")])
		ticker := row[getIndex(headers, "Ticker")]
		tickersYahoo := map[string]string{
			"U.U": "U-UN",
		}
		if yahooTicker, ok := tickersYahoo[ticker]; ok {
			ticker = yahooTicker
		}
		if fx != 0 && fx != 1.00 {
			ticker = fmt.Sprintf("%s.TO", ticker)
		}

		//status := row[getIndex(headers, "Status")]
		open := row[getIndex(headers, "Open")]
		entryPrice := row[getIndex(headers, "Bought")]
		entryTime, _ := time.Parse(service.DateTimeFormat2, open)
		//close := row[getIndex(headers, "Close")]
		//closeTime, _ := time.Parse(service.DateTimeFormat2, close)
		shares := extractNum(row[getIndex(headers, "Shares")])
		if open == "" {
			continue
		}

		loc, _ := time.LoadLocation("Asia/Singapore")

		if entryTime.Before(time.Date(2025, 1, 1, 0, 0, 0, 0, loc)) {
			continue
		}
		if _, ok := tickersFilter[ticker]; !ok && len(tickers) > 0 {
			continue
		}

		if strings.ToLower(side) == "short" && shares > 0 {
			continue
		}
		if strings.ToLower(side) == "long" && shares < 0 {
			continue
		}

		thenTime := entryTime
		//currentTime := time.Now()

		//if close != "" {
		//	if closeTime.Before(sinceTime) {
		//		continue
		//	}
		//	if closeTime.Before(time.Now()) {
		//		currentTime = closeTime
		//	}
		//}

		displayTicker := ticker
		if _, ok := performanceByTicker[ticker]; ok {
			displayTicker = fmt.Sprintf("%s_%d", ticker, rowIdx+1)
		}
		//if entryTime.Before(sinceTime) || open == since {

		result, _ := h.yahooService.GetStock(ticker, "1d", "2y")
		if len(result.Indicators.Quote) > 0 && len(result.Indicators.Quote[0].Close) > 0 {
			quote := result.Indicators.Quote[0]
			closes := quote.Close
			highs := quote.High
			lows := quote.Low

			tickerPnL := 0.0
			for i, timestamp := range result.Timestamps {
				val, err := strconv.ParseInt(fmt.Sprintf("%d", timestamp), 10, 64)
				if err != nil {
					panic(err)
				}
				tm := time.Unix(val, 0)
				if tm.Format(service.DateTimeFormat) == thenTime.Format(service.DateTimeFormat) || tm.After(thenTime) {
					thenPrice := extractNum(entryPrice)
					currentPrice := closes[i]
					dayHighLow := closes[i]
					if shares > 0 {
						dayHighLow = lows[i]
					} else {
						dayHighLow = highs[i]
					}
					if percentStr == "" {
						atr := h.yahooService.ATR(quote, 14)
						percent = atrMultiple * atr / currentPrice
					}
					runningPercent := (dayHighLow / thenPrice) - 1
					if (shares > 0 && runningPercent <= -percent) || (shares < 0 && runningPercent >= percent) {
						performanceByTicker[displayTicker] = fmt.Sprintf("Red %0.2f :%0.2f NC", 100*-percent, shares*(percent*currentPrice))
						tickerPnL = shares * (currentPrice - thenPrice) * fx
						countTracker["stopLoss"] = countTracker["stopLoss"] + 1
						break
					}

					// for target, refer to highs for longs, lows for shorts
					if shares > 0 {
						dayHighLow = highs[i]
					} else {
						dayHighLow = lows[i]
					}
					runningPercent = (dayHighLow / thenPrice) - 1
					profitPercent := multiple * percent
					if (shares > 0 && runningPercent >= profitPercent) || (shares < 0 && -runningPercent >= profitPercent) {
						performanceByTicker[displayTicker] = fmt.Sprintf("Green %0.2f :%0.2f NC", 100*profitPercent, shares*(profitPercent*currentPrice))
						tickerPnL = shares * (currentPrice - thenPrice) * fx
						countTracker["target"] = countTracker["target"] + 1

						break
					}

					if shares*(currentPrice-thenPrice) >= 0 {
						performanceByTicker[displayTicker] = fmt.Sprintf("Green %0.2f :%0.2f NC", 100*((currentPrice/thenPrice)-1), shares*(currentPrice-thenPrice))
					} else {
						performanceByTicker[displayTicker] = fmt.Sprintf("Red %0.2f :%0.2f NC", 100*((currentPrice/thenPrice)-1), shares*(currentPrice-thenPrice))
					}
					tickerPnL = shares * (currentPrice - thenPrice) * fx

				}
			}
			totalPnL += tickerPnL
		}
	}

	Success(w, r, map[string]interface{}{
		"totalPnL": roundFloat(totalPnL),
		"data":     performanceByTicker,
		"count":    countTracker,
	})
}
func (h SheetsHandler) AnalyseEntry(w http.ResponseWriter, r *http.Request) {
	percentStr := r.URL.Query().Get("percent")
	side := r.URL.Query().Get("side")
	if percentStr == "" {
		//Success(w, r, map[string]interface{}{})
		//return
	}
	percent := extractNum(r.URL.Query().Get("percent"))
	multiple := extractNum(r.URL.Query().Get("multiple"))
	if multiple == 0 {
		multiple = 2
	}

	atrMultiple := extractNum(r.URL.Query().Get("atr"))
	if atrMultiple == 0 {
		atrMultiple = 2
	}
	result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, sheetMap["trades"])
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	tickers := r.URL.Query()["ticker"]
	tickersFilter := make(map[string]bool)
	for _, ticker := range tickers {
		tickersFilter[ticker] = true
	}

	headers := result.Values[0]
	performanceByTicker := make(map[string]string)
	countTracker := make(map[string]int)
	totalPnL := 0.0
	for rowIdx, row := range result.Values[1:] {
		if len(row) <= 15 || row[getIndex(headers, "Ticker")] == "" {
			break
		}
		fx := extractNum(row[getIndex(headers, "Currency")])
		ticker := row[getIndex(headers, "Ticker")]
		tickersYahoo := map[string]string{
			"U.U": "U-UN",
		}
		if yahooTicker, ok := tickersYahoo[ticker]; ok {
			ticker = yahooTicker
		}
		if fx != 0 && fx != 1.00 {
			ticker = fmt.Sprintf("%s.TO", ticker)
		}

		//status := row[getIndex(headers, "Status")]
		open := row[getIndex(headers, "Open")]
		entryPrice := row[getIndex(headers, "Bought")]
		target := row[getIndex(headers, "Sold")]
		entryTime, _ := time.Parse(service.DateTimeFormat2, open)
		//close := row[getIndex(headers, "Close")]
		//closeTime, _ := time.Parse(service.DateTimeFormat2, close)

		loc, _ := time.LoadLocation("Asia/Singapore")

		if entryTime.Before(time.Date(2025, 1, 1, 0, 0, 0, 0, loc)) {
			continue
		}
		status := row[getIndex(headers, "Status")]
		if status != "Target" {
			continue
		}
		shares := extractNum(row[getIndex(headers, "Shares")])
		if open == "" {
			continue
		}

		if _, ok := tickersFilter[ticker]; !ok && len(tickers) > 0 {
			continue
		}

		if strings.ToLower(side) == "short" && shares > 0 {
			continue
		}
		if strings.ToLower(side) == "long" && shares < 0 {
			continue
		}

		thenTime := entryTime
		//currentTime := time.Now()

		//if close != "" {
		//	if closeTime.Before(sinceTime) {
		//		continue
		//	}
		//	if closeTime.Before(time.Now()) {
		//		currentTime = closeTime
		//	}
		//}

		displayTicker := ticker
		if _, ok := performanceByTicker[ticker]; ok {
			displayTicker = fmt.Sprintf("%s_%d", ticker, rowIdx+1)
		}
		//if entryTime.Before(sinceTime) || open == since {

		result, _ := h.yahooService.GetStock(ticker, "1d", "2y")
		thenPrice := extractNum(entryPrice)
		targetPrice := extractNum(target)
		targetPriceReached := false
		if len(result.Indicators.Quote) > 0 && len(result.Indicators.Quote[0].Close) > 0 {
			quote := result.Indicators.Quote[0]
			closes := quote.Close
			highs := quote.High
			lows := quote.Low

			tickerPnL := 0.0
			for i, timestamp := range result.Timestamps {
				val, err := strconv.ParseInt(fmt.Sprintf("%d", timestamp), 10, 64)
				if err != nil {
					panic(err)
				}
				tm := time.Unix(val, 0)
				if tm.Format(service.DateTimeFormat) == thenTime.Format(service.DateTimeFormat) || tm.After(thenTime) {
					currentPrice := closes[i]
					currentHigh := highs[i]
					currentLow := lows[i]
					dayHighLow := closes[i]
					if shares > 0 {
						dayHighLow = lows[i]
						if currentHigh >= targetPrice {
							targetPriceReached = true
						}
					} else {
						dayHighLow = highs[i]
						if currentLow <= targetPrice {
							targetPriceReached = true
						}
					}
					if percentStr == "" {
						percent = currentPrice/thenPrice - 1
					}
					runningPercent := (dayHighLow / thenPrice) - 1
					if targetPriceReached && ((shares > 0 && runningPercent <= 0) || (shares < 0 && runningPercent >= 0)) {
						performanceByTicker[displayTicker] = fmt.Sprintf("Red %0.2f :%0.2f NC", 100*-percent, shares*(percent*currentPrice))
						tickerPnL = shares * (dayHighLow - thenPrice) * fx
						countTracker["stopLoss"] = countTracker["stopLoss"] + 1
						break
					}

					// for target, refer to highs for longs, lows for shorts
					if shares > 0 {
						dayHighLow = highs[i]
					} else {
						dayHighLow = lows[i]
					}
					runningPercent = (dayHighLow / thenPrice) - 1
					profitPercent := multiple * (targetPrice/thenPrice - 1)
					if (shares > 0 && runningPercent >= profitPercent) || (shares < 0 && -runningPercent >= profitPercent) {
						tickerPnL = shares * thenPrice * profitPercent * fx
						performanceByTicker[displayTicker] = fmt.Sprintf("Green %0.2f%% :%0.2f NC", 100*profitPercent, tickerPnL)
						countTracker["target"] = countTracker["target"] + 1

						break
					}

					if shares*(currentPrice-thenPrice) >= 0 {
						performanceByTicker[displayTicker] = fmt.Sprintf("Green %0.2f :%0.2f NC", 100*((currentPrice/thenPrice)-1), shares*(currentPrice-thenPrice))
					} else {
						performanceByTicker[displayTicker] = fmt.Sprintf("Red %0.2f :%0.2f NC", 100*((currentPrice/thenPrice)-1), shares*(currentPrice-thenPrice))
					}
					tickerPnL = shares * (currentPrice - thenPrice) * fx

				}
			}
			totalPnL += tickerPnL
		}
	}

	Success(w, r, map[string]interface{}{
		"totalPnL": roundFloat(totalPnL),
		"data":     performanceByTicker,
		"count":    countTracker,
	})
}
func (h SheetsHandler) Analyse2(w http.ResponseWriter, r *http.Request) {
	percentStr := r.URL.Query().Get("percent")
	side := r.URL.Query().Get("side")
	if percentStr == "" {
		//Success(w, r, map[string]interface{}{})
		//return
	}
	percent := extractNum(r.URL.Query().Get("percent"))
	multiple := extractNum(r.URL.Query().Get("multiple"))
	if multiple == 0 {
		multiple = 2
	}

	atrMultiple := extractNum(r.URL.Query().Get("atr"))
	if atrMultiple == 0 {
		atrMultiple = 2
	}
	result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, sheetMap["trades"])
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	tickers := r.URL.Query()["ticker"]
	tickersFilter := make(map[string]bool)
	for _, ticker := range tickers {
		tickersFilter[ticker] = true
	}

	headers := result.Values[0]
	performanceByTicker := make(map[string]string)
	countTracker := make(map[string]int)
	totalPnL := 0.0
	for rowIdx, row := range result.Values[1:] {
		if len(row) <= 15 || row[getIndex(headers, "Ticker")] == "" {
			break
		}
		fx := extractNum(row[getIndex(headers, "Currency")])
		ticker := row[getIndex(headers, "Ticker")]
		tickersYahoo := map[string]string{
			"U.U": "U-UN",
		}
		if yahooTicker, ok := tickersYahoo[ticker]; ok {
			ticker = yahooTicker
		}
		if fx != 0 && fx != 1.00 {
			ticker = fmt.Sprintf("%s.TO", ticker)
		}

		//status := row[getIndex(headers, "Status")]
		open := row[getIndex(headers, "Open")]
		entryPrice := row[getIndex(headers, "Bought")]
		entryTime, _ := time.Parse(service.DateTimeFormat2, open)
		//close := row[getIndex(headers, "Close")]
		//closeTime, _ := time.Parse(service.DateTimeFormat2, close)
		shares := extractNum(row[getIndex(headers, "Shares")])
		if open == "" {
			continue
		}

		if _, ok := tickersFilter[ticker]; !ok && len(tickers) > 0 {
			continue
		}

		if strings.ToLower(side) == "short" && shares > 0 {
			continue
		}
		if strings.ToLower(side) == "long" && shares < 0 {
			continue
		}

		thenTime := entryTime
		//currentTime := time.Now()

		//if close != "" {
		//	if closeTime.Before(sinceTime) {
		//		continue
		//	}
		//	if closeTime.Before(time.Now()) {
		//		currentTime = closeTime
		//	}
		//}

		displayTicker := ticker
		if _, ok := performanceByTicker[ticker]; ok {
			displayTicker = fmt.Sprintf("%s_%d", ticker, rowIdx+1)
		}
		//if entryTime.Before(sinceTime) || open == since {

		result, _ := h.yahooService.GetStock(ticker, "1d", "1y")
		if len(result.Indicators.Quote) > 0 && len(result.Indicators.Quote[0].Close) > 0 {
			quote := result.Indicators.Quote[0]
			closes := quote.Close
			highs := quote.High
			lows := quote.Low
			opens := quote.Open

			tickerPnL := 0.0
			newPercentSet := false
			for i, timestamp := range result.Timestamps {
				val, err := strconv.ParseInt(fmt.Sprintf("%d", timestamp), 10, 64)
				if err != nil {
					panic(err)
				}
				tm := time.Unix(val, 0)
				if tm.Format(service.DateTimeFormat) == thenTime.Format(service.DateTimeFormat) || tm.After(thenTime) {
					thenPrice := extractNum(entryPrice)
					currentPrice := closes[i]
					dayHighLow := closes[i]
					open := opens[i]
					if shares > 0 {
						dayHighLow = lows[i]
					} else {
						dayHighLow = highs[i]
					}
					if tm.Format(service.DateTimeFormat) == thenTime.Format(service.DateTimeFormat) {
						if shares > 0 {
							if open < currentPrice {
								dayHighLow = currentPrice
							}
						} else {
							if open > currentPrice {
								dayHighLow = currentPrice
							}
						}
					}
					if percentStr == "" {
						atr := h.yahooService.ATR(quote, 14)
						percent = atrMultiple * atr / currentPrice
					}
					runningPercent := (dayHighLow / thenPrice) - 1
					newPercent := 0.0
					newTargetPercent := multiple*percent + percent
					if !newPercentSet {
						newPercent = percent
						newTargetPercent = multiple * percent
					}
					if (shares > 0 && runningPercent >= percent) || (shares < 0 && -runningPercent >= percent) {
						newPercent = 0.00
						newTargetPercent = multiple*percent + percent
						newPercentSet = true
					}
					if (shares > 0 && runningPercent <= -newPercent) || (shares < 0 && runningPercent >= newPercent) {
						performanceByTicker[displayTicker] = fmt.Sprintf("Red %0.2f :%0.2f NC", 100*-newPercent, shares*(newPercent*currentPrice))
						tickerPnL = shares * (currentPrice - thenPrice) * fx
						countTracker["stopLoss"] = countTracker["stopLoss"] + 1
						break
					}

					// for target, refer to highs for longs, lows for shorts
					if shares > 0 {
						dayHighLow = highs[i]
					} else {
						dayHighLow = lows[i]
					}
					runningPercent = (dayHighLow / thenPrice) - 1
					if (shares > 0 && runningPercent >= newTargetPercent) || (shares < 0 && -runningPercent >= newTargetPercent) {
						performanceByTicker[displayTicker] = fmt.Sprintf("Green %0.2f :%0.2f NC", 100*newTargetPercent, shares*(newTargetPercent*currentPrice))
						tickerPnL = shares * (currentPrice - thenPrice) * fx
						countTracker["target"] = countTracker["target"] + 1

						break
					}

					if shares*(currentPrice-thenPrice) >= 0 {
						performanceByTicker[displayTicker] = fmt.Sprintf("Green %0.2f :%0.2f NC", 100*((currentPrice/thenPrice)-1), shares*(currentPrice-thenPrice))
					} else {
						performanceByTicker[displayTicker] = fmt.Sprintf("Red %0.2f :%0.2f NC", 100*((currentPrice/thenPrice)-1), shares*(currentPrice-thenPrice))
					}
					tickerPnL = shares * (currentPrice - thenPrice) * fx

				}
			}
			totalPnL += tickerPnL
		}
	}

	Success(w, r, map[string]interface{}{
		"totalPnL": roundFloat(totalPnL),
		"data":     performanceByTicker,
		"count":    countTracker,
	})
}

func (h SheetsHandler) getSinceTime(result model.Result2, sinceTime time.Time) time.Time {
	for _, timestamp := range result.Timestamps {
		val, err := strconv.ParseInt(fmt.Sprintf("%d", timestamp), 10, 64)
		if err != nil {
			panic(err)
		}
		tm := time.Unix(val, 0)
		if tm.Format(service.DateTimeFormat) == sinceTime.Format(service.DateTimeFormat) {
			return sinceTime
		}
		if sinceTime.After(time.Now()) {
			return sinceTime
		}
	}

	latestTime := result.Timestamps[len(result.Timestamps)-1]
	val, err := strconv.ParseInt(fmt.Sprintf("%d", latestTime), 10, 64)
	if err != nil {
		panic(err)
	}
	tm := time.Unix(val, 0)
	if tm.Before(sinceTime) {
		return tm
	}

	return h.getSinceTime(result, sinceTime.AddDate(0, 0, 1))
}

func (h SheetsHandler) Trades(w http.ResponseWriter, r *http.Request) {
	readRange := sheetMap["trades"]
	side := r.URL.Query().Get("side")
	sector := r.URL.Query().Get("sector")
	status := r.URL.Query().Get("status")
	working := r.URL.Query().Get("working")
	result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, readRange)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	headers := result.Values[0]
	total := 0.0
	count := 0.0
	amount := 0.0
	for _, row := range result.Values[1:] {
		if len(row) <= 15 || row[getIndex(headers, "Ticker")] == "" {
			break
		}

		if side == "long" && parseFloat(row[getIndex(headers, "Shares")]) > 0 {
			fmt.Printf("%s\n", row[getIndex(headers, "Ticker")])
			total, amount, count = getTradeStats(sector, row, headers, total, amount, count, status, working, "")
			continue
		}
		if side == "short" && parseFloat(row[getIndex(headers, "Shares")]) < 0 {
			fmt.Printf("%s\n", row[getIndex(headers, "Ticker")])
			total, amount, count = getTradeStats(sector, row, headers, total, amount, count, status, working, "")
			continue
		}

		if sector != "" && strings.TrimSpace(row[getIndex(headers, "Sector")]) == sector {
			fmt.Printf("%s\n", row[getIndex(headers, "Ticker")])
			total, amount, count = getTradeStats("", row, headers, total, amount, count, status, working, side)
			continue
		}
		if status != "" && strings.TrimSpace(row[getIndex(headers, "Status")]) == status {
			fmt.Printf("%s\n", row[getIndex(headers, "Ticker")])
			total, amount, count = getTradeStats(sector, row, headers, total, amount, count, "", working, side)
			continue
		}

		if working == "open" && row[getIndex(headers, "Status")] == "" {
			fmt.Printf("%s\n", row[getIndex(headers, "Ticker")])
			total, amount, count = getTradeStats(sector, row, headers, total, amount, count, status, "", side)
			continue
		}
		if working == "all" && row[getIndex(headers, "Status")] == "" {
			fmt.Printf("%s\n", row[getIndex(headers, "Ticker")])
			total, amount, count = getTradeStats(sector, row, headers, total, amount, count, status, "", side)
			continue
		}
	}

	Success(w, r, map[string]interface{}{
		"total":    roundFloat(total),
		"count":    count,
		"amount":   roundFloat(amount),
		"p/l":      fmt.Sprintf("%0.2f %%, avg trade %0.3f %%", 100*total/amount, 100*total/amount/count),
		"avgTrade": fmt.Sprintf("%0.2f", total/count),
	})
}

func getTradeStats(sector string, row []string, headers []string, total float64, amount float64, count float64, status string, working string, side string) (float64, float64, float64) {
	if side == "long" && parseFloat(row[getIndex(headers, "Shares")]) > 0 {
		if sector == "" && status == "" && working == "" {
			pnl := extractNum(row[getIndex(headers, "P/L")])
			amt := extractNum(row[getIndex(headers, "Amount")])
			amount += amt
			total += pnl
			count++
		} else {
			total, amount, count = getTradeStats(sector, row, headers, total, amount, count, status, working, "")
		}
		return total, amount, count
	}
	if side == "short" && parseFloat(row[getIndex(headers, "Shares")]) < 0 {
		if sector == "" && status == "" && working == "" {
			pnl := extractNum(row[getIndex(headers, "P/L")])
			amt := extractNum(row[getIndex(headers, "Amount")])
			amount += amt
			total += pnl
			count++
		} else {
			total, amount, count = getTradeStats(sector, row, headers, total, amount, count, status, working, "")
		}
		return total, amount, count
	}
	if sector != "" {
		if strings.TrimSpace(row[getIndex(headers, "Sector")]) == sector {
			if side == "" && status == "" && working == "" {
				pnl := extractNum(row[getIndex(headers, "P/L")])
				total += pnl
				amt := extractNum(row[getIndex(headers, "Amount")])
				amount += amt
				count++
			} else {
				total, amount, count = getTradeStats("", row, headers, total, amount, count, status, working, side)
			}
		}
		return total, amount, count
	}
	if status != "" && row[getIndex(headers, "Status")] == status {
		if side == "" && sector == "" && working == "" {
			pnl := extractNum(row[getIndex(headers, "P/L")])
			total += pnl
			amt := extractNum(row[getIndex(headers, "Amount")])
			amount += amt
			count++
		} else {
			total, amount, count = getTradeStats(sector, row, headers, total, amount, count, "", working, side)
		}
		return total, amount, count
	}
	if working == "open" {
		if row[getIndex(headers, "Status")] == "" {
			if side == "" && sector == "" && status == "" {
				pnl := extractNum(row[getIndex(headers, "P/L")])
				total += pnl
				amt := extractNum(row[getIndex(headers, "Amount")])
				amount += amt
				count++
			} else {
				total, amount, count = getTradeStats(sector, row, headers, total, amount, count, status, "", side)
			}
		}
		return total, amount, count
	}
	if working == "all" {
		if side == "" && sector == "" && status == "" {
			pnl := extractNum(row[getIndex(headers, "P/L")])
			total += pnl
			amt := extractNum(row[getIndex(headers, "Amount")])
			amount += amt
			count++
		} else {
			total, amount, count = getTradeStats(sector, row, headers, total, amount, count, status, "", side)
		}
		return total, amount, count
	}
	if working == "" && side == "" && sector == "" && status == "" {
		pnl := extractNum(row[getIndex(headers, "P/L")])
		total += pnl
		amt := extractNum(row[getIndex(headers, "Amount")])
		amount += amt
		count++
	}

	return total, amount, count
}

func parseFloat(str string) float64 {
	val, _ := strconv.ParseFloat(str, 64)
	return val
}
func extractNum(str string) float64 {
	if strings.HasPrefix(str, "(") && strings.HasSuffix(str, ")") {
		str = str[1 : len(str)-1]
		return -extractNum(str)
	}
	str = strings.Replace(str, ",", "", -1)
	str = strings.Replace(str, "$", "", -1)
	return parseFloat(str)
}

func getIndex(headers []string, column string) int {
	for i, header := range headers {
		if header == column {
			return i
		}
	}

	return -1
}

func (h SheetsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request model.InsertRow

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: bad request", err))

		return
	}

	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(request.DataRange, -1)
	if len(matches) == 0 {
		Errors(w, r, http.StatusBadRequest, "cannot find portfolio data range")
		return
	}
	rowNumber, _ := strconv.ParseInt(matches[0], 10, 32)
	rowNumber++
	dataRange := fmt.Sprintf(Portfolio, rowNumber, rowNumber)

	result, err := h.sheetsService.CreateWatchlist(request, h.spreadsheetID, dataRange)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	Success(w, r, result.Updates)
}

func (h SheetsHandler) Record(w http.ResponseWriter, r *http.Request) {
	var request model.RecordTrade
	region := r.URL.Query().Get("region")

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: bad request", err))

		return
	}

	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(request.DataRange, -1)
	if len(matches) == 0 {
		Errors(w, r, http.StatusBadRequest, "cannot find portfolio data range")
		return
	}
	rowNumber, _ := strconv.ParseInt(matches[0], 10, 32)
	rowNumber++
	dataRange := fmt.Sprintf(Record, rowNumber, rowNumber)
	if strings.ToLower(region) == "ind" {
		dataRange = fmt.Sprintf(IndianRecords, rowNumber, rowNumber)
	}

	//tickerDetails, err := h.nasdaqService.Summary(request.Symbol, "stocks")
	//if err != nil {
	//	fmt.Errorf("cannot get details: %w", err)
	//}
	//request.Sector = tickerDetails.SummaryData.Sector.Value

	result, err := h.sheetsService.RecordTrade(request, h.spreadsheetID, dataRange)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	Success(w, r, result.Updates)
}

func (h SheetsHandler) RecordAll(w http.ResponseWriter, r *http.Request) {
	var request model.RecordTrade
	region := r.URL.Query().Get("region")

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: bad request", err))

		return
	}

	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(request.DataRange, -1)
	if len(matches) == 0 {
		Errors(w, r, http.StatusBadRequest, "cannot find portfolio data range")
		return
	}
	rowNumber, _ := strconv.ParseInt(matches[0], 10, 32)
	rowNumber++
	dataRange := fmt.Sprintf(RecordAll, rowNumber, rowNumber)
	if strings.ToLower(region) == "ind" {
		dataRange = fmt.Sprintf(IndianRecords, rowNumber, rowNumber)
	}

	tickerDetails, err := h.nasdaqService.Summary(request.Symbol, "stocks")
	if err != nil {
		fmt.Errorf("cannot get details: %w", err)
	}
	request.Sector = tickerDetails.SummaryData.Sector.Value

	result, err := h.sheetsService.RecordTrade(request, h.spreadsheetID, dataRange)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	Success(w, r, result.Updates)
}

func (h SheetsHandler) ShortlistInsert(w http.ResponseWriter, r *http.Request) {
	readRange := sheetMap["shortlist"]

	var request model.Shortlist
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		Errors(w, r, http.StatusBadRequest, fmt.Sprintf("%v: bad request", err))

		return
	}

	result, err := h.sheetsService.GetValues(r.Context(), h.spreadsheetID, readRange)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	dataRange := fmt.Sprintf("%s!A%d:N%d", "Shortlist", len(result.Values)+1, len(result.Values)+1)

	checkupDetails, err := h.ibdService.Checkup(request.Symbol)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}
	for _, detail := range checkupDetails.QuotesData.CompanyFundamentals {
		if detail.Element == "3 Year EPS Growth Rate" {
			request.EpsGrowth = detail.Value
		}
		if detail.Element == "3-Year Sales Growth Rate" {
			request.SalesGrowth = detail.Value
		}
		if detail.Element == "Annual ROE" {
			request.ROE = detail.Value
		}
		if detail.Element == "Profit Margin" {
			request.GrossMargin = detail.Value
		}
		if detail.Element == "Industry Group Rank" {
			request.IndustryGroupRank = detail.Value
		}
		if detail.Element == "PE Ratio" {
			request.PERatio = detail.Value
		}
	}
	for _, detail := range checkupDetails.QuotesData.StockData {
		if detail.Element == "PE Ratio" {
			request.PERatio = detail.Value
		}
	}

	result, err = h.sheetsService.ShortlistRecord(request, h.spreadsheetID, dataRange)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	Success(w, r, result.Updates)
}

func replaceString(str string, list [][]string) string {
	var finalStr string = str
	for _, strSlice := range list {
		finalStr = strings.Replace(str, strSlice[0], strSlice[1], 1)
	}

	return finalStr
}

func (h SheetsHandler) ActiveItems(w http.ResponseWriter, r *http.Request) {
	tickers := r.URL.Query()["ticker"]
	actionItems := make([]model.ActionItem, 0)
	_, err := h.sheetsService.ClearSheet(h.spreadsheetID, ActionItems)
	if err != nil {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	for _, ticker := range tickers {
		tickerInfo, err := h.nasdaqService.Summary(ticker, "stocks")
		if err != nil {
			continue
		}
		_, surprisesCount := h.getSurprises(err, ticker)
		closingPrice, atr := h.getAtr(ticker)
		revenues, _ := h.nasdaqService.Revenues(ticker)
		months3, months12 := h.getInsiderActivity(ticker)
		earningsForecasts, _ := h.nasdaqService.EarningsForecast(ticker)
		var upForecasts int
		var downForecasts int
		for _, forecast := range earningsForecasts {
			if forecast.Up > 0 {
				upForecasts += forecast.Up
			}

			if forecast.Down > 0 {
				downForecasts -= forecast.Down
			}
		}
		revenueStr := fmt.Sprintf("%s | %s | %s", revenues.Value2, revenues.Value3, revenues.Value4)
		if revenues.Value2 == "" {
			revenueStr = ""
		}
		actionItem := model.ActionItem{
			Ticker:          ticker,
			Industry:        tickerInfo.SummaryData.Industry.Value,
			Sector:          tickerInfo.SummaryData.Sector.Value,
			MarketCap:       tickerInfo.SummaryData.MarketCap.Value,
			Volume:          tickerInfo.SummaryData.AverageVolume.Value,
			Beta:            0,
			ATR:             fmt.Sprintf("%0.2f%%", 100*atr/closingPrice),
			Revenues:        revenueStr,
			InsiderActivity: months3 + months12,
			Surprises:       surprisesCount,
			Upgrades:        fmt.Sprintf("%d", upForecasts),
			Downgrades:      fmt.Sprintf("%d", downForecasts),
			NetUpgrades:     fmt.Sprintf("%d", upForecasts-downForecasts),
			Yield:           tickerInfo.SummaryData.Yield.Value,
		}

		actionItems = append(actionItems, actionItem)
	}

	response := make([]model.Update, 0)
	for _, actionItem := range actionItems {
		res, _ := h.sheetsService.RecordActionItems(actionItem, h.spreadsheetID, ActionItems)
		response = append(response, res.Updates)
	}

	Success(w, r, response)
}

func (h SheetsHandler) getInsiderActivity(ticker string) (float64, float64) {
	insiderActivity, _ := h.nasdaqService.InsiderActivity(ticker)
	var months3 string
	var months12 string
	for _, insiderActivity := range insiderActivity.NumberOfSharesTraded.Rows {
		if insiderActivity.InsiderTrade == "Net Activity" {
			months3 = insiderActivity.Months3
			months12 = insiderActivity.Months12
		}
	}

	return extractNum(months3), extractNum(months12)
}

func (h SheetsHandler) getAtr(ticker string) (float64, float64) {
	result, _ := h.yahooService.GetStock(ticker, "1d", "2y")
	if len(result.Indicators.Quote) == 0 {
		return 1, 0
	}
	quote := result.Indicators.Quote[0]
	closes := quote.Close
	closingPrice := closes[len(closes)-1]
	atr := h.yahooService.ATR(quote, 14)
	return closingPrice, atr
}

func (h SheetsHandler) getSurprises(err error, ticker string) ([]float64, int) {
	surprises, err := h.nasdaqService.EarningSurprises(ticker)
	if err != nil {
	}
	surprisesList := make([]float64, 0)
	count := 0
	for _, surprise := range surprises {
		surpriseValue, _ := strconv.ParseFloat(surprise.PercentageSurprise, 64)
		if surpriseValue >= 0 {
			count++
		} else {
			count--
		}
		surprisesList = append(surprisesList, surpriseValue)
	}
	return surprisesList, count
}

func formatString(val string, value float64) string {
	parsedFloat, err := ExtractNumber(val)
	if err != nil {
		return val
	}

	if parsedFloat >= value {
		return fmt.Sprintf("Green%0.2fNC", parsedFloat)
	}

	if parsedFloat >= 0.8*value {
		return fmt.Sprintf("Blue%0.2fNC", parsedFloat)
	}

	return fmt.Sprintf("Red%0.2fNC", parsedFloat)
}

func formatNetScore(val string) string {
	parsedFloat, err := ExtractNumber(val)
	if err != nil {
		return val
	}

	if parsedFloat >= 10.0 {
		return fmt.Sprintf("Green%0.2fNC (/14)", parsedFloat)
	}

	return fmt.Sprintf("%0.2fNC (/14)", parsedFloat)
}

func formatMarketCap(val string) string {
	parsedFloat, err := ExtractNumber(val)
	if err != nil {
		return val
	}

	if parsedFloat >= 100 {
		return fmt.Sprintf("Red%0.2fNC", parsedFloat)
	}

	if parsedFloat >= 50 {
		return fmt.Sprintf("Blue%0.2fNC", parsedFloat)
	}

	return fmt.Sprintf("Green%0.2fNC", parsedFloat)
}
func formatCompositeRank(val string) string {
	parsedFloat, err := ExtractNumber(val)
	if err != nil {
		return val
	}

	if parsedFloat <= 50 {
		return fmt.Sprintf("Red%0.0fNC", parsedFloat)
	}

	if parsedFloat <= 90 {
		return fmt.Sprintf("Blue%0.0fNC", parsedFloat)
	}

	return fmt.Sprintf("Green%0.0fNC", parsedFloat)
}

func formatPE(val string) string {
	parsedFloat, err := ExtractNumber(val)
	if err != nil {
		return val
	}

	if parsedFloat >= 100 {
		return fmt.Sprintf("Red%0.2fNC", parsedFloat)
	}

	if parsedFloat > 20 {
		return fmt.Sprintf("Blue%0.2fNC", parsedFloat)
	}

	return fmt.Sprintf("Green%0.2fNC", parsedFloat)
}

func ExtractNumber(input string) (float64, error) {
	// Use regex to match a number (optional negative sign, digits)
	input = strings.Replace(input, ",", "", -1)
	re := regexp.MustCompile(`^-?\d+(\.\d+)?`)
	match := re.FindString(input)
	if match == "" {
		return 0, errors.New("no valid integer found")
	}

	// Convert the matched string to an integer
	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, errors.New("invalid integer format")
	}

	return value, nil
}
