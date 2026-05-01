package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"ibkr/model"
	"io"
	"math"
	"net/http"
	"strings"
)

type Yahoo struct {
}

func NewYahoo() Yahoo {
	return Yahoo{}
}

func (s Yahoo) Similar(ctx context.Context, symbol string) (model.SimilarStockResponse, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	response, err := http.Get(fmt.Sprintf("%s%s", "https://query1.finance.yahoo.com/v6/finance/recommendationsbysymbol/", symbol))
	if err != nil {
		return model.SimilarStockResponse{}, err
	}
	if response.StatusCode > http.StatusOK {
		fmt.Errorf("cannot get stocks: %s", response.Status)

		return model.SimilarStockResponse{}, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("cannot stocks orders :%w", err)
		return model.SimilarStockResponse{}, err
	}
	response.Body.Close()

	var similarStock model.SimilarStockResponse
	parseErr := json.Unmarshal(bodyBytes, &similarStock)
	if parseErr != nil {
		fmt.Errorf("symbol error: %w", parseErr)
		return model.SimilarStockResponse{}, err
	}

	return similarStock, nil
}

func (s Yahoo) GetStock(symbol string, period string, duration string) (model.Result2, model.Meta) {
	defaultRes := model.Result2{}
	endpoint := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=%s&range=%s", symbol, period, duration)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", symbol, err)
		return model.Result2{}, model.Meta{}
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	response, err := client.Do(req)

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", symbol, err)
		return defaultRes, model.Meta{}
	}
	response.Body.Close()

	var data model.YahooResponse
	parseErr := json.Unmarshal(bodyBytes, &data)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", symbol, parseErr)
		return defaultRes, model.Meta{}
	}

	if len(data.Chart.Result) == 0 {
		fmt.Printf("error: cannot get stockinfo for %s \n", symbol)

		return defaultRes, model.Meta{}
	}

	return data.Chart.Result[0], data.Chart.Result[0].Meta
}

func (s Yahoo) GetActiveStocks(start, count int, scrID string, region string) ([]model.ActiveQuote, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v1/finance/screener/predefined/saved?formatted=true&lang=en-US&region=US&scrIds=%s&start=%d&count=%d&enableSectorIndustryLabelFix=true&corsDomain=finance.yahoo.com", scrID, start, count)
	if region == "ind" {
		url = fmt.Sprintf("https://query1.finance.yahoo.com/v1/finance/screener/public/saved?formatted=true&lang=en-US&region=US&scrId=%s&start=%d&count=%d&enableSectorIndustryLabelFix=true&corsDomain=finance.yahoo.com", scrID, start, count)
	}
	response, err := http.Get(url)
	if err != nil {
		fmt.Errorf("cannot get active stocks error: %w", err)
		return []model.ActiveQuote{}, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("cannot read response error: %w", err)
		return []model.ActiveQuote{}, err
	}
	response.Body.Close()

	var data model.ActiveStocksResponse
	parseErr := json.Unmarshal(bodyBytes, &data)
	if parseErr != nil {
		fmt.Errorf("cannot unmarshall error: %w", parseErr)
		return []model.ActiveQuote{}, parseErr
	}

	if len(data.Finance.Result) == 0 {
		return []model.ActiveQuote{}, nil
	}

	return data.Finance.Result[0].Quotes, nil
}

func (s Yahoo) GetMovingAverage(closes []float64, days int) float64 {
	i := 0
	var sum float64 = 0

	if len(closes) < days {
		return math.Inf(1)
	}

	previousTimeSeries := closes[len(closes)-days:]
	for _, price := range previousTimeSeries {
		if i == days {
			break
		}

		sum += price
		i++
	}

	return sum / float64(days)
}

func (s Yahoo) IsRisingMA(closes []float64, days int) bool {
	ma5 := s.GetMovingAverage(closes, days)
	ma4 := s.GetMovingAverage(closes[0:len(closes)-1], days)
	ma3 := s.GetMovingAverage(closes[0:len(closes)-2], days)
	ma2 := s.GetMovingAverage(closes[0:len(closes)-3], days)
	ma1 := s.GetMovingAverage(closes[0:len(closes)-4], days)

	if ma5 >= ma4 && ma4 >= ma3 && ma3 >= ma2 && ma2 >= ma1 {
		return true
	}

	return false
}

func (s Yahoo) trueRange(quote model.Quote) float64 {
	high := quote.High[len(quote.High)-1]
	low := quote.Low[len(quote.Low)-1]
	close := quote.Close[len(quote.Low)-1]

	/*TR = Max [(H−L),∣H−Cp∣,∣L−C p∣]*/
	return math.Max(high-low, math.Max(math.Abs(high-close), math.Abs(low-close)))
}

func (s Yahoo) ATR2(quote model.Quote, days int) float64 {
	total14 := 0.0
	for i := 0; i < int(days); i++ {
		quote := model.Quote{
			High:  quote.High[0 : days-i],
			Low:   quote.Low[0 : days-i],
			Close: quote.Close[0 : days-i],
		}
		total14 += s.trueRange(quote)
	}

	daysFloat := float64(days)
	prevATR := total14 / daysFloat

	for i := int(days) + 1; i < len(quote.Close)-1; i++ {
		quote := model.Quote{
			High:  quote.High[0:i],
			Low:   quote.Low[0:i],
			Close: quote.Close[0:i],
		}
		prevATR = (prevATR*(daysFloat-1) + s.trueRange(quote)) / daysFloat
	}

	prevATR = (prevATR*(daysFloat-1) + s.trueRange(quote)) / daysFloat

	return prevATR
}
func (s Yahoo) ATR(quote model.Quote, days int) float64 {
	if len(quote.Close) < days {
		return 0.0
	}

	lows := quote.Low[len(quote.Low)-days:]

	differences := make([]float64, 0, days)
	for i, high := range quote.High[len(quote.High)-days:] {
		differences = append(differences, high-lows[i])
	}
	sum := 0.0
	for _, diff := range differences {
		sum += diff
	}

	return sum / float64(days)
}

func (s Yahoo) Volatility(symbol string) float64 {
	result, _ := s.GetStock(symbol, "1d", "2y")
	quote := result.Indicators.Quote[0]
	closes := quote.Close
	closingPrice := closes[len(closes)-1]
	atr := s.ATR(quote, 14)

	index := s.getExchange(symbol, result)
	indexResult, _ := s.GetStock(index, "1d", "2y")
	spyATR := s.ATR(indexResult.Indicators.Quote[0], 14)
	spyClosingPrice := indexResult.Indicators.Quote[0].Close[len(indexResult.Indicators.Quote[0].Close)-1]

	return (atr * spyClosingPrice) / (spyATR * closingPrice)
}

func (s Yahoo) CrossIndex(symbol string) (string, float64, int) {
	result, _ := s.GetStock(symbol, "1d", "2y")
	quote := result.Indicators.Quote[0]
	closes := quote.Close

	index := s.getExchange(symbol, result)
	indexResult, _ := s.GetStock(index, "1d", "2y")
	indexCloses := indexResult.Indicators.Quote[0]

	crossIndex := make([]float64, len(quote.Close), len(quote.Close))
	for i, close := range closes {
		if indexCloses.Close[i] != 0 {
			crossIndex[i] = close / indexCloses.Close[i]
		}
	}

	ma50 := s.GetMovingAverage(crossIndex, 50)
	ma150 := s.GetMovingAverage(crossIndex, 150)
	ma200 := s.GetMovingAverage(crossIndex, 200)

	aboveStr := fmt.Sprintf("Index: %s;", index)
	crossClosingPrice := crossIndex[len(crossIndex)-1]
	count := 0
	if crossClosingPrice >= ma50 {
		aboveStr += " Above 50MA;"
		count++
	}
	if crossClosingPrice >= ma150 {
		aboveStr += " Above 150MA;"
		count++
	}
	if crossClosingPrice >= ma200 {
		aboveStr += " Above 200MA;"
		count++
	}

	closingPrice52WkHigh := s.getHighest(crossIndex[int(len(crossIndex)/2):])
	closingPrice26WkHigh := s.getHighest(crossIndex[int(3*len(crossIndex)/4):])

	closeWeekAgo := crossIndex[len(crossIndex)-6]
	fiveDayRelativeReturn := 0.0
	fiveDayRelativeReturn = (crossClosingPrice / closeWeekAgo) - 1.00

	fiveDayRelativeReturn = fiveDayRelativeReturn * 100
	aboveStr += fmt.Sprintf(" 5D return %f%%;", fiveDayRelativeReturn)

	if crossClosingPrice >= closingPrice52WkHigh {
		aboveStr += " 52w High"
	} else {
		aboveStr += fmt.Sprintf(" Below %.2f%% 52w High;", 100*(1-(crossClosingPrice/closingPrice52WkHigh)))
	}

	if crossClosingPrice >= closingPrice26WkHigh {
		aboveStr += " 26w High"
	}

	dip, turning, top := s.findCluster(crossIndex, crossClosingPrice)
	if dip {
		aboveStr += " Clustering;"
	}
	if turning {
		aboveStr += " Turning;"
	}
	if top {
		aboveStr += " Topping;"
	}

	return aboveStr, fiveDayRelativeReturn, count
}

func (s Yahoo) findCluster(closes []float64, closingPrice float64) (bool, bool, bool) {
	ma50 := s.GetMovingAverage(closes, 50)
	ma150 := s.GetMovingAverage(closes, 150)
	ma200 := s.GetMovingAverage(closes, 200)

	previousClosingPrice := closes[len(closes)-2]
	if previousClosingPrice == 0.0 {
		return false, false, false
	}
	change := s.getChange(closes, previousClosingPrice)

	maBand := 1.10
	priceBand := 1.05

	maClusters := change >= 0 &&
		closingPrice >= ma50 &&
		closingPrice >= ma150 &&
		closingPrice >= ma200 &&
		!s.isFallingMA3(closes, 200, 50) && !s.isFallingMA3(closes, 150, 50) &&
		s.getMABand(ma50, ma150, ma200) <= maBand &&
		s.isPriceBand(ma50, ma150, ma200, closingPrice) <= priceBand

	isTurning := change >= 0 &&
		closingPrice >= ma50 &&
		closingPrice >= ma150 &&
		closingPrice >= ma200 &&
		//s.isRisingMA3(closes, 50, 20) &&
		s.getRisingCount(closes) >= 1 &&
		s.getFallingCount(closes) >= 1 &&
		s.getMABand(ma50, ma150, ma200) <= maBand &&
		s.isPriceBand(ma50, ma150, ma200, closingPrice) <= priceBand

	maResistance := change < 0.0 &&
		closingPrice < ma50 &&
		closingPrice < ma150 &&
		closingPrice < ma200 &&
		s.getFallingCount(closes) >= 1 &&
		s.getMABand(ma50, ma150, ma200) <= 1.10 &&
		//s.getRisingCount(closes) <= 1 &&
		s.isPriceBand(ma50, ma150, ma200, -closingPrice) < 1.05

	return maClusters, isTurning, maResistance
}

func (s Yahoo) getRisingCount(closes []float64) int {
	count := 0
	if s.isRisingMA3(closes, 50, 10) {
		count++
	}
	if s.isRisingMA3(closes, 150, 10) {
		count++
	}
	if s.isRisingMA3(closes, 200, 10) {
		count++
	}

	return count
}

func (s Yahoo) getFallingCount(closes []float64) int {
	count := 0
	if s.isFallingMA3(closes, 50, 10) {
		count++
	}
	if s.isFallingMA3(closes, 150, 10) {
		count++
	}
	if s.isFallingMA3(closes, 200, 10) {
		count++
	}

	return count
}
func (s Yahoo) isFallingMA3(closes []float64, days int, window int) bool {
	if len(closes) < window {
		return true
	}

	ma := s.GetMovingAverage(closes, days)
	count := 0.0
	for i := 1; i < window; i++ {
		if ma < s.GetMovingAverage(closes[0:len(closes)-i], days) {
			count++
		}
		ma = s.GetMovingAverage(closes[0:len(closes)-i], days)
	}

	return count/float64(window) >= 0.6
}

func (s Yahoo) getMABand(x, y, z float64) float64 {
	greatest := x
	least := x
	if y > greatest {
		greatest = y
	}
	if z > greatest {
		greatest = z
	}

	if y < least {
		least = y
	}

	if z < least {
		least = z
	}

	return greatest / least
}

func (s Yahoo) isPriceBand(x, y, z, price float64) float64 {
	highest := x
	least := x
	if y > highest {
		highest = y
	}
	if z > highest {
		highest = z
	}

	if y < least {
		least = y
	}

	if z < least {
		least = z
	}

	if price > 0 {
		if price <= highest {
			return highest / price
		}

		return price / highest
	}

	price = math.Abs(price)
	return least / price
}
func (s Yahoo) isRisingMA3(closes []float64, days int, window int) bool {
	if len(closes) < window {
		return false
	}

	ma := s.GetMovingAverage(closes, days)
	count := 0.0
	for i := 1; i <= window; i++ {
		if ma >= s.GetMovingAverage(closes[0:len(closes)-(i+1)], days) {
			count++
		}
		ma = s.GetMovingAverage(closes[0:len(closes)-(i+1)], days)
	}

	return count/float64(window) >= 0.8
}

func (s Yahoo) getChange(closes []float64, previousClose float64) float64 {
	closePrice := closes[len(closes)-1]

	if closePrice >= previousClose {
		return (closePrice/previousClose - 1) * 100
	}

	return -((1 - closePrice/previousClose) * 100)
}

func (s Yahoo) getHighest(closes []float64) float64 {
	high := closes[0]
	for i := 1; i < len(closes)-1; i++ {
		if closes[i] > high {
			high = closes[i]
		}
	}

	return high
}

func (s Yahoo) getExchange(symbol string, result model.Result2) string {
	index := "SPY"
	if len(strings.Split(symbol, ".")) > 1 && strings.Split(symbol, ".")[1] == "NS" {
		index = "^NSEI"
	}
	if strings.Contains(strings.ToLower(result.Meta.FullExchangeName), "nasdaq") {
		index = "QQQ"
	}
	return index
}
