package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"ibkr/model"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mocks ---

type mockIBKRService struct {
	searchFn      func(ctx context.Context, query string) ([]model.Symbol, error)
	orderFn       func(ctx context.Context, orders []model.Order) (model.OrderResponse, error)
	updateFn      func(ctx context.Context, order model.Order) (model.OrderResponse, error)
	getAllFn      func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error)
	listOrdersFn  func(ctx context.Context) ([]model.ListOrder, []string, error)
	cancelFn      func(ctx context.Context, symbol string, orderID string) ([]model.CancelOrder, error)
	lastOrders    []model.Order
	updatedOrders []model.Order
}

func (m *mockIBKRService) GetAll(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx, tickers)
	}
	return model.AllOrders{}, nil
}
func (m *mockIBKRService) Order(ctx context.Context, orders []model.Order) (model.OrderResponse, error) {
	m.lastOrders = orders
	if m.orderFn != nil {
		return m.orderFn(ctx, orders)
	}
	return model.OrderResponse{{OrderId: "1", OrderStatus: "Submitted"}}, nil
}
func (m *mockIBKRService) Update(ctx context.Context, order model.Order) (model.OrderResponse, error) {
	m.updatedOrders = append(m.updatedOrders, order)
	if m.updateFn != nil {
		return m.updateFn(ctx, order)
	}
	return model.OrderResponse{}, nil
}
func (m *mockIBKRService) Search(ctx context.Context, query string) ([]model.Symbol, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query)
	}
	return []model.Symbol{{ConID: "123", Symbol: query, Description: "NASDAQ"}}, nil
}
func (m *mockIBKRService) Security(ctx context.Context, contractIDs []string) ([]model.Security, error) {
	return nil, nil
}
func (m *mockIBKRService) Cancel(ctx context.Context, symbol string, orderID string) ([]model.CancelOrder, error) {
	if m.cancelFn != nil {
		return m.cancelFn(ctx, symbol, orderID)
	}
	return nil, nil
}
func (m *mockIBKRService) Reply(replyID string) (model.OrderResponse, error) {
	return nil, nil
}
func (m *mockIBKRService) ListOrders(ctx context.Context) ([]model.ListOrder, []string, error) {
	if m.listOrdersFn != nil {
		return m.listOrdersFn(ctx)
	}
	return []model.ListOrder{{Ticker: "AAPL"}}, []string{"AAPL"}, nil
}
func (m *mockIBKRService) Init() {}

type mockPositionService struct {
	summaryFn func(ctx context.Context) (model.Summary, error)
	getAllFn  func(ctx context.Context) ([]model.Position, []string, float64, error)
}

func (m *mockPositionService) GetAll(ctx context.Context) ([]model.Position, []string, float64, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return nil, nil, 0, nil
}
func (m *mockPositionService) Summary(ctx context.Context) (model.Summary, error) {
	if m.summaryFn != nil {
		return m.summaryFn(ctx)
	}
	return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
}

type mockYahooService struct {
	stockFn func(symbol, period, duration string) (model.Result2, model.Meta)
}

func (m *mockYahooService) Similar(ctx context.Context, symbol string) (model.SimilarStockResponse, error) {
	return model.SimilarStockResponse{}, nil
}
func (m *mockYahooService) GetStock(symbol string, period string, duration string) (model.Result2, model.Meta) {
	if m.stockFn != nil {
		return m.stockFn(symbol, period, duration)
	}
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = 100
	}
	return model.Result2{
		Meta: model.Meta{RegularMarketPrice: 100},
		Indicators: model.Indicators{Quote: []model.Quote{{
			Close: closes, High: closes, Low: closes, Open: closes, Volume: make([]int64, 30),
		}}},
	}, model.Meta{RegularMarketPrice: 100}
}
func (m *mockYahooService) GetActiveStocks(start, count int, scrID string, region string) ([]model.ActiveQuote, error) {
	return nil, nil
}
func (m *mockYahooService) GetMovingAverage(closes []float64, days int) float64 { return 0 }
func (m *mockYahooService) IsRisingMA(closes []float64, days int) bool          { return false }
func (m *mockYahooService) ATR(quote model.Quote, days int) float64             { return 2 }
func (m *mockYahooService) Volatility(symbol string) float64                    { return 0 }
func (m *mockYahooService) CrossIndex(symbol string) (string, float64, int)     { return "", 0, 0 }

type mockIBDService struct {
	quotesFn func(ctx context.Context, symbol string, currentDate time.Time) ([]model.Quote, error)
}

func (m *mockIBDService) Checkup(ticker string) (model.Checkup, error) {
	return model.Checkup{}, nil
}
func (m *mockIBDService) GetStockQuotes(ctx context.Context, symbol string, currentDate time.Time) ([]model.Quote, error) {
	if m.quotesFn != nil {
		return m.quotesFn(ctx, symbol, currentDate)
	}
	return nil, errors.New("no quotes")
}

type mockFileService struct{}

func (m *mockFileService) Write(fileName, content string)             {}
func (m *mockFileService) WriteBytes(fileName string, content []byte) {}

type mockAlertService struct{}

func (m *mockAlertService) Create(ctx context.Context, alert model.Alert) (interface{}, error) {
	return nil, nil
}
func (m *mockAlertService) GetAll(ctx context.Context) ([]model.IBKRAlert, error) { return nil, nil }

type mockPolygonService struct{}

func (m *mockPolygonService) Summary(ticker string) (model.PolygonTickerDetails, error) {
	return model.PolygonTickerDetails{}, nil
}

type mockNasdaqService struct{}

func (m *mockNasdaqService) Summary(ticker string, assetClass string) (model.StockSummary, error) {
	return model.StockSummary{}, nil
}
func (m *mockNasdaqService) InsiderActivity(ticker string) (model.InsiderActivity, error) {
	return model.InsiderActivity{}, nil
}
func (m *mockNasdaqService) News(ticker string) ([]model.NewsRow, error) { return nil, nil }
func (m *mockNasdaqService) Earnings(ticker string) (model.Earnings, error) {
	return model.Earnings{}, nil
}
func (m *mockNasdaqService) EarningSurprises(ticker string) ([]model.EarningSuprises, error) {
	return nil, nil
}
func (m *mockNasdaqService) EarningsForecast(ticker string) ([]model.EarningsForecast, error) {
	return nil, nil
}
func (m *mockNasdaqService) Active(offset, limit int) ([]model.ActiveStock, int, error) {
	return nil, 0, nil
}
func (m *mockNasdaqService) Revenues(ticker string) (model.Revenue, error) {
	return model.Revenue{}, nil
}
func (m *mockNasdaqService) Calendar(date string) (string, []model.Calendar, error) {
	return "", nil, nil
}

func newTestIbkr(ibkr *mockIBKRService, pos *mockPositionService, yahoo *mockYahooService, ibd *mockIBDService) Ibkr {
	if ibkr == nil {
		ibkr = &mockIBKRService{}
	}
	if pos == nil {
		pos = &mockPositionService{}
	}
	if yahoo == nil {
		yahoo = &mockYahooService{}
	}
	if ibd == nil {
		ibd = &mockIBDService{}
	}
	return NewIbkr(ibkr, &mockAlertService{}, pos, &mockFileService{}, yahoo, &mockPolygonService{}, &mockNasdaqService{}, ibd, nil, 0)
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) Response {
	t.Helper()
	var resp Response
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp
}

// --- unit: validateTradeRisk / getPrices ---

func TestValidateTradeRisk(t *testing.T) {
	t.Parallel()

	assert.NoError(t, validateTradeRisk(100000, 10, 100, 90))   // risk 100 <= 1000; stop 10%
	assert.Error(t, validateTradeRisk(100000, 200, 100, 90))    // risk 2000 > 1000
	assert.Error(t, validateTradeRisk(0, 10, 100, 90))          // bad equity
	assert.Error(t, validateTradeRisk(100000, 10, 100, 100))    // zero stop distance
	assert.NoError(t, validateTradeRisk(100000, -10, 100, 110)) // short qty uses abs

	// stop tighter than 0.5%: 100 vs 99.6 = 0.4%
	assert.Error(t, validateTradeRisk(100000, 10, 100, 99.6))
	assert.Contains(t, validateTradeRisk(100000, 10, 100, 99.6).Error(), "stop too tight")

	// notional > 25% equity even when stop risk is OK (wide stop)
	// qty 300 * 100 = 30000 > 25000; risk = 300*20 = 6000 > 1000 anyway — use smaller risk:
	// qty 260 * entry 100 = 26000 > 25000; stop 90 → risk 2600 > 1000. Need risk OK:
	// max risk 1000 → qty*stopDist <= 1000; stopDist=10 → qty<=100; notional=100*100=10000 OK.
	// For notional fail with risk OK: entry 200, stop 190 (5%), qty 130 → risk 1300>1000.
	// entry 200, stop 190, qty 120 → risk 1200>1000.
	// entry 250, stop 237.5 (5%), qty 80 → risk 1000, notional 20000 OK.
	// entry 250, stop 237.5, qty 101 → risk 1262.5>1000.
	// entry 400, stop 380 (5%), qty 50 → risk 1000, notional 20000 OK.
	// entry 400, stop 380, qty 63 → risk 1260>1000.
	// entry 500, stop 475 (5%), qty 40 → risk 1000, notional 20000.
	// entry 500, stop 475, qty 51 → risk 1275; notional 25500 > 25000 — both fail; check notional msg with risk under:
	// equity 100000, max risk 1000, max notional 25000.
	// qty 50, entry 501, stop 481 (dist 20) → risk 1000, notional 25050 > 25000.
	assert.Error(t, validateTradeRisk(100000, 50, 501, 481))
	assert.Contains(t, validateTradeRisk(100000, 50, 501, 481).Error(), "notional too large")

	assert.Error(t, validateNotional(100000, 300, 100))
	assert.NoError(t, validateNotional(100000, 200, 100))
}

func TestResolveQuantity(t *testing.T) {
	t.Parallel()

	// equity 100k, pos 0.2 → risk budget 200; stop dist 10 → derived 20
	qty, err := resolveQuantity(0, 100000, 0.2, 100, 90)
	require.NoError(t, err)
	assert.Equal(t, 20, qty)

	qty, err = resolveQuantity(20, 100000, 0.2, 100, 90)
	require.NoError(t, err)
	assert.Equal(t, 20, qty)

	qty, err = resolveQuantity(22, 100000, 0.2, 100, 90) // 10% exactly
	require.NoError(t, err)
	assert.Equal(t, 22, qty)

	_, err = resolveQuantity(23, 100000, 0.2, 100, 90) // 15%
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quantity mismatch")

	// TWLO-style fat finger: given 193 vs derived ~15
	_, err = resolveQuantity(193, 100000, 0.2, 187.91, 187.91*(1-0.077))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quantity mismatch")
}

func TestGetPrices(t *testing.T) {
	t.Parallel()

	buy, stop, target, target2 := getPrices("100/90/130/150")
	assert.Equal(t, 100.0, buy)
	assert.Equal(t, 90.0, stop)
	assert.Equal(t, 130.0, target)
	assert.Equal(t, 150.0, target2)

	buy, stop, target, target2 = getPrices("100/90")
	assert.Equal(t, 100.0, buy)
	assert.Equal(t, 90.0, stop)
	assert.Equal(t, 0.0, target)
	assert.Equal(t, 0.0, target2)
}

func TestLastValidClose(t *testing.T) {
	t.Parallel()

	result := model.Result2{
		Timestamps: []int{100, 200, 300},
		Indicators: model.Indicators{Quote: []model.Quote{{Close: []float64{50, 51, 52}}}},
	}
	assert.Equal(t, 52.0, lastValidClose(result, model.Meta{}))

	// Latest bar is 0; use close at regularMarketTime.
	result = model.Result2{
		Timestamps: []int{100, 200, 300},
		Indicators: model.Indicators{Quote: []model.Quote{{Close: []float64{50, 51, 0}}}},
	}
	assert.Equal(t, 51.0, lastValidClose(result, model.Meta{RegularMarketTime: 200}))

	// No matching time bar; walk back to last non-zero close.
	assert.Equal(t, 51.0, lastValidClose(result, model.Meta{}))

	// All closes zero — fall back to regularMarketPrice.
	result = model.Result2{
		Timestamps: []int{100},
		Indicators: model.Indicators{Quote: []model.Quote{{Close: []float64{0}}}},
	}
	assert.Equal(t, 96.4, lastValidClose(result, model.Meta{RegularMarketPrice: 96.4}))
}

func TestGetOrders_OpenRiskFromStop(t *testing.T) {
	ibkr := &mockIBKRService{
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return []model.ListOrder{{
				Ticker: "MSFT", OrderType: "Stop", Status: "Submitted",
				OrderDesc: "Sell 1000 Share Stop 90.00",
			}}, []string{"MSFT"}, nil
		},
	}
	pos := &mockPositionService{
		getAllFn: func(ctx context.Context) ([]model.Position, []string, float64, error) {
			return nil, []string{"MSFT"}, 0, nil
		},
	}
	yahoo := &mockYahooService{
		stockFn: func(symbol, period, duration string) (model.Result2, model.Meta) {
			return model.Result2{Indicators: model.Indicators{Quote: []model.Quote{{
				Close: []float64{0},
			}}}}, model.Meta{RegularMarketPrice: 100, RegularMarketTime: 1}
		},
	}
	h := newTestIbkr(ibkr, pos, yahoo, nil)
	_, _, _, _, _, _, openRisk, _, err := h.getOrders(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, 10000.0, openRisk, 0.01)
}

func TestGetOrders_NewOrdersFromStopLimitBracket(t *testing.T) {
	ibkr := &mockIBKRService{
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return []model.ListOrder{
				{
					Ticker: "TEVA", OrderType: "Stop Limit", Status: "PreSubmitted",
					OrderDesc: "Buy 246 TEVA Stop 37.45 LMT 37.56, GTC",
					Price:     0, Quantity: 246,
				},
				{
					Ticker: "TEVA", OrderType: "Limit", Status: "PreSubmitted",
					OrderDesc: "Sell 246 TEVA Limit 39.50, GTC",
					Price:     39.50, Quantity: 246,
				},
				{
					Ticker: "TEVA", OrderType: "Stop", Status: "PreSubmitted",
					OrderDesc: "Sell 246 TEVA Stop 36.95, GTC",
					Price:     0, Quantity: 246,
				},
				{
					Ticker: "TEVA", OrderType: "Limit", Status: "Submitted",
					OrderDesc: "Sell 102 TEVA Limit 39.50, GTC",
					Price:     39.50, Quantity: 102,
				},
				{
					Ticker: "TEVA", OrderType: "Stop", Status: "Submitted",
					OrderDesc: "Sell 102 TEVA Stop 36.95, GTC",
					Price:     0, Quantity: 102,
				},
			}, []string{"TEVA"}, nil
		},
	}
	pos := &mockPositionService{
		getAllFn: func(ctx context.Context) ([]model.Position, []string, float64, error) {
			return nil, []string{"TEVA"}, 0, nil
		},
	}
	yahoo := &mockYahooService{
		stockFn: func(symbol, period, duration string) (model.Result2, model.Meta) {
			return model.Result2{Indicators: model.Indicators{Quote: []model.Quote{{
				Close: []float64{0},
			}}}}, model.Meta{RegularMarketPrice: 37.54, RegularMarketTime: 1}
		},
	}
	h := newTestIbkr(ibkr, pos, yahoo, nil)
	_, _, _, newOrders, newOrdersList, _, _, _, err := h.getOrders(context.Background())
	require.NoError(t, err)
	require.Len(t, newOrdersList, 1)
	assert.Equal(t, "TEVA", newOrdersList[0])
	require.Len(t, newOrders, 1)
	// Entry LMT 37.56; SL Stop 36.95; Target Limit 39.50
	assert.Contains(t, newOrders[0], "TEVA SL 36.95")
	assert.Contains(t, newOrders[0], "Target 39.50")
	assert.NotContains(t, newOrders[0], "SL 37.45")
	assert.NotContains(t, newOrders[0], "Target 37.56")
}

// --- Preview ---

func TestPreview_RequiresExactlyOneSymbol(t *testing.T) {
	h := newTestIbkr(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/preview?position=0.2", nil)
	rr := httptest.NewRecorder()
	h.Preview(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "exactly one symbol")

	req = httptest.NewRequest(http.MethodPost, "/preview?symbol=AAPL&symbol=MSFT&position=0.2", nil)
	rr = httptest.NewRecorder()
	h.Preview(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "exactly one symbol")
}

func TestPreview_RejectsPositionOverOne(t *testing.T) {
	h := newTestIbkr(nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/preview?symbol=AAPL&position=1.5&entry=100&stopLoss=5&side=buy", nil)
	rr := httptest.NewRecorder()
	h.Preview(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "position too large")
}

func TestPreview_SizesFromEquity(t *testing.T) {
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(nil, pos, nil, nil)

	// risk = 100000 * 0.01 * 0.2 = 200; stop dist = 5% of 100 = 5; qty = 40
	req := httptest.NewRequest(http.MethodPost, "/preview?symbol=AAPL&position=0.2&entry=100&stopLoss=5&side=buy&coID=123", nil)
	rr := httptest.NewRecorder()
	h.Preview(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	resp := decodeBody(t, rr)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 40.0, data["quantity"])
	assert.Equal(t, 100.0, data["entry"])
	assert.Equal(t, 95.0, data["stopLoss"])
	assert.Equal(t, 5.0, data["stopLossPercent"])
}

func TestPreview_UsesStopLossPercentFromURL(t *testing.T) {
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	// Yahoo would return a different ATR-based stop if URL stopLoss were ignored.
	yahoo := &mockYahooService{
		stockFn: func(symbol, period, duration string) (model.Result2, model.Meta) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 385.74
			}
			return model.Result2{Indicators: model.Indicators{Quote: []model.Quote{{
				Close: closes, High: closes, Low: closes, Open: closes,
			}}}}, model.Meta{RegularMarketPrice: 385.74}
		},
	}
	h := newTestIbkr(nil, pos, yahoo, nil)

	// entry 385.74, stop 9.5% → stopLevel 349.09; risk 250; qty = round(250/36.65) = 7
	req := httptest.NewRequest(http.MethodPost, "/preview?symbol=HUM&entry=385.74&position=0.25&coID=8251&stopLoss=9.5&side=buy", nil)
	rr := httptest.NewRecorder()
	h.Preview(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	data := decodeBody(t, rr).Data.(map[string]interface{})
	assert.InDelta(t, 349.09, data["stopLoss"].(float64), 0.01)
	assert.Equal(t, 9.5, data["stopLossPercent"])
	assert.Equal(t, 7.0, data["quantity"])
}

// --- MarketBuy ---

func TestMarketBuy_RequiresExactlyOneSymbol(t *testing.T) {
	h := newTestIbkr(nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/buy?symbol=AAPL&symbol=MSFT&position=0.2", nil)
	rr := httptest.NewRecorder()
	h.MarketBuy(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "exactly one symbol")
}

func TestMarketBuy_RejectsPositionOverOne(t *testing.T) {
	h := newTestIbkr(nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/buy?symbol=AAPL&position=1.5&coID=123", nil)
	rr := httptest.NewRecorder()
	h.MarketBuy(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "position too large")
}

// --- BracketOrder ---

func TestBracketOrder_RejectsRiskOverOnePercent(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{}, nil
		},
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	// qty 200 vs derived 20 → mismatch (fat-finger guard; same class as TWLO 193)
	body := `{"orders":[{"ticker":"TWLO","conid":1,"prices":"100/110","side":"SELL","orderType":"STP LMT","quantity":200,"position":0.2}]}`
	req := httptest.NewRequest(http.MethodPost, "/bracketOrders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.BracketOrder(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "quantity mismatch")
}

func TestBracketOrder_RejectsTightStop(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	// stop 0.4% of entry — below 0.5% min; qty 0 so sizing runs then stop check fails
	body := `{"orders":[{"ticker":"AAPL","conid":1,"prices":"100/99.6","side":"BUY","orderType":"LMT","quantity":0,"position":0.2}]}`
	req := httptest.NewRequest(http.MethodPost, "/bracketOrders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.BracketOrder(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "stop too tight")
}

func TestBracketOrder_RejectsPortfolioRisk(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return []model.ListOrder{{
				Ticker: "MSFT", OrderType: "Stop", Status: "Submitted",
				OrderDesc: "Sell 1600 Share Stop 90.00",
			}}, []string{"MSFT"}, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
		getAllFn: func(ctx context.Context) ([]model.Position, []string, float64, error) {
			return nil, []string{"MSFT"}, 0, nil
		},
	}
	yahoo := &mockYahooService{
		stockFn: func(symbol, period, duration string) (model.Result2, model.Meta) {
			return model.Result2{Indicators: model.Indicators{Quote: []model.Quote{{
				Close: []float64{100},
			}}}}, model.Meta{RegularMarketPrice: 100}
		},
	}
	h := newTestIbkr(ibkr, pos, yahoo, nil)

	// open risk: |100-90|*1600 = 16000 > 15% of 100k; qty 20 matches derived
	body := `{"orders":[{"ticker":"AAPL","conid":1,"prices":"100/90","side":"BUY","orderType":"LMT","quantity":20,"position":0.2}]}`
	req := httptest.NewRequest(http.MethodPost, "/bracketOrders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.BracketOrder(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "portfolio risk too large")
}

func TestBracketOrder_SubmitsSecondTarget(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{}, nil
		},
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	// qty 20 matches derived (100k * 0.2 / 100 / 10); two targets
	body := `{"orders":[{"ticker":"AAPL","conid":1,"prices":"100/90/120/140","side":"BUY","orderType":"LMT","quantity":20,"position":0.2}]}`
	req := httptest.NewRequest(http.MethodPost, "/bracketOrders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.BracketOrder(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Len(t, ibkr.lastOrders, 4) // parent, tp1, tp2, stop
	assert.Equal(t, 120.0, ibkr.lastOrders[1].Price)
	assert.Equal(t, 140.0, ibkr.lastOrders[2].Price)
	assert.Equal(t, 10, ibkr.lastOrders[1].Quantity)
	assert.Equal(t, 10, ibkr.lastOrders[2].Quantity)
}

func TestBracketOrder_ShortSubmitsSecondTarget(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{}, nil
		},
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	// short: entry 100, stop 110, tp1 80, tp2 70 (target2 < target)
	body := `{"orders":[{"ticker":"TWLO","conid":1,"prices":"100/110/80/70","side":"SELL","orderType":"STP LMT","quantity":20,"position":0.2}]}`
	req := httptest.NewRequest(http.MethodPost, "/bracketOrders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.BracketOrder(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Len(t, ibkr.lastOrders, 4) // parent, tp1, tp2, stop
	assert.Equal(t, 80.0, ibkr.lastOrders[1].Price)
	assert.Equal(t, 70.0, ibkr.lastOrders[2].Price)
	assert.Equal(t, "BUY", ibkr.lastOrders[1].Side)
	assert.Equal(t, "BUY", ibkr.lastOrders[2].Side)
	assert.Equal(t, 10, ibkr.lastOrders[1].Quantity)
	assert.Equal(t, 10, ibkr.lastOrders[2].Quantity)
}

func TestBracketOrder_ShortKeepsCallerTarget(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{}, nil
		},
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	// target 75 (not buy*0.8=80) so we prove caller target is kept; qty 20 matches derived
	body := `{"orders":[{"ticker":"TWLO","conid":1,"prices":"100/110/75","side":"SELL","orderType":"STP LMT","quantity":20,"position":0.2}]}`
	req := httptest.NewRequest(http.MethodPost, "/bracketOrders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.BracketOrder(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.GreaterOrEqual(t, len(ibkr.lastOrders), 3)
	var takeProfit model.Order
	for _, o := range ibkr.lastOrders {
		if o.OrderType == "LMT" && o.ParentID != "" {
			takeProfit = o
			break
		}
	}
	assert.Equal(t, 75.0, takeProfit.Price)
	assert.Equal(t, "BUY", takeProfit.Side)
}

func TestBracketOrder_ShortLimitFlipsExitSides(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{}, nil
		},
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	body := `{"orders":[{"ticker":"TWLO","conid":1,"prices":"100/110/75","side":"SELL","orderType":"LMT","quantity":20,"position":0.2}]}`
	req := httptest.NewRequest(http.MethodPost, "/bracketOrders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.BracketOrder(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Len(t, ibkr.lastOrders, 3) // parent, tp, stop
	assert.Equal(t, "SELL", ibkr.lastOrders[0].Side)
	assert.Equal(t, "BUY", ibkr.lastOrders[1].Side) // take profit
	assert.Equal(t, "LMT", ibkr.lastOrders[1].OrderType)
	assert.Equal(t, "BUY", ibkr.lastOrders[2].Side) // stop
	assert.Equal(t, "STP", ibkr.lastOrders[2].OrderType)
}

func TestBracketOrder_SizesAfterMarketPrice(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{}, nil
		},
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	yahoo := &mockYahooService{
		stockFn: func(symbol, period, duration string) (model.Result2, model.Meta) {
			// live price 95; prices entry was 100/90 → after rewrite buy=95, stop=90, dist=5
			// risk budget at pos 0.2 = 200 → qty = 40
			return model.Result2{Indicators: model.Indicators{Quote: []model.Quote{{
				Close: []float64{95},
			}}}}, model.Meta{RegularMarketPrice: 95}
		},
	}
	h := newTestIbkr(ibkr, pos, yahoo, nil)

	body := `{"orders":[{"ticker":"AAPL","conid":1,"prices":"100/90","side":"BUY","orderType":"LMT","quantity":0,"position":0.2,"marketprice":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/bracketOrders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.BracketOrder(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotEmpty(t, ibkr.lastOrders)
	assert.Equal(t, 40, ibkr.lastOrders[0].Quantity)
	assert.InDelta(t, 95.0, ibkr.lastOrders[0].Price, 0.01)
}

// --- OCA ---

func TestOCA_FailsWhenQuantityTooSmall(t *testing.T) {
	h := newTestIbkr(nil, nil, nil, nil)

	for _, qty := range []int{0, 1} {
		body, _ := json.Marshal(model.IBKROrder{Orders: []model.Order{{
			Ticker: "AAPL", Prices: "100/90/120", Quantity: qty, Price: 100,
		}}})
		req := httptest.NewRequest(http.MethodPost, "/ocaOrders?side=BUY", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.OCA(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "qty=%d", qty)
		assert.Contains(t, decodeBody(t, rr).Message, "quantity must be > 1")
	}
}

func TestOCA_RejectsRiskOverOnePercent(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	ibkr := &mockIBKRService{
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	body, _ := json.Marshal(model.IBKROrder{Orders: []model.Order{{
		Ticker: "AAPL", Prices: "100/90/120", Quantity: 200, Price: 100,
	}}})
	req := httptest.NewRequest(http.MethodPost, "/ocaOrders?side=BUY", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.OCA(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "risk too large")
}

func TestContracts_RejectsLargeNotional(t *testing.T) {
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{Orders: []model.OrderDetails{{
				Ticker: "AAPL", OrderId: 42, TotalSize: 10, Price: "100",
			}}}, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	body := `{"orderID":42,"price":100,"quantity":300}`
	req := httptest.NewRequest(http.MethodPut, "/contracts", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Contracts(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "notional too large")
}

func TestUpdate_RejectsRiskOverOnePercent(t *testing.T) {
	ibkr := &mockIBKRService{
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	body := `{"orders":[{"ticker":"AAPL","conid":1,"orderId":9,"account":"TEST","aaPrices":"100/90","orderType":"Stop","side":"SELL","totalSize":200}]}`
	req := httptest.NewRequest(http.MethodPut, "/update", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "risk too large")
}

// --- Merge ---

func TestMerge_RequiresTwoOrders(t *testing.T) {
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{Orders: []model.OrderDetails{{
				Ticker: "AAPL", OrigOrderType: "LIMIT", RemainingQuantity: 10, OrderId: 1,
			}}}, nil
		},
	}
	h := newTestIbkr(ibkr, nil, nil, nil)

	body := `{"ticker":"AAPL","stopLoss":90,"target":120}`
	req := httptest.NewRequest(http.MethodPut, "/merge", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Merge(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "need at least 2 open orders")
}

// --- MarketSell ---

func TestMarketSell_RejectsPositionOverOne(t *testing.T) {
	h := newTestIbkr(nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/close?symbol=AAPL&position=5", nil)
	rr := httptest.NewRecorder()
	h.MarketSell(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "position invalid")
}

func TestMarketSell_NothingToCloseWhenNoPosition(t *testing.T) {
	pos := &mockPositionService{
		getAllFn: func(ctx context.Context) ([]model.Position, []string, float64, error) {
			return []model.Position{{
				ContractDesc: "MSFT", Position: 10, Conid: 1, AcctId: "TEST",
			}}, []string{"MSFT"}, 1000, nil
		},
	}
	ibkr := &mockIBKRService{}
	h := newTestIbkr(ibkr, pos, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/close?symbol=AAPL&position=1", nil)
	rr := httptest.NewRecorder()
	h.MarketSell(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "nothing to close")
	assert.Nil(t, ibkr.lastOrders)
}

func TestMarketSell_ExitsLongPosition(t *testing.T) {
	pos := &mockPositionService{
		getAllFn: func(ctx context.Context) ([]model.Position, []string, float64, error) {
			return []model.Position{{
				ContractDesc: "AAPL", Position: 10, Conid: 1, AcctId: "TEST",
			}}, []string{"AAPL"}, 1000, nil
		},
	}
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{Orders: []model.OrderDetails{
				{Conid: 1, Ticker: "AAPL", OrderType: "Limit", OrderDesc: "Sell 10 AAPL Limit 120, GTC", RemainingQuantity: 10, Status: "Submitted"},
				{Conid: 1, Ticker: "AAPL", OrderType: "Stop", OrderDesc: "Sell 10 AAPL Stop 90, GTC", RemainingQuantity: 10, Status: "PreSubmitted"},
			}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/close?symbol=AAPL&position=1", nil)
	rr := httptest.NewRecorder()
	h.MarketSell(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMarketSell_ExitsShortPosition(t *testing.T) {
	pos := &mockPositionService{
		getAllFn: func(ctx context.Context) ([]model.Position, []string, float64, error) {
			return []model.Position{{
				ContractDesc: "AAPL", Position: -10, Conid: 1, AcctId: "TEST",
			}}, []string{"AAPL"}, 1000, nil
		},
	}
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{Orders: []model.OrderDetails{
				{Conid: 1, Ticker: "AAPL", OrderType: "Limit", OrderDesc: "Buy 10 AAPL Limit 120, GTC", RemainingQuantity: 10, Status: "Submitted"},
				{Conid: 1, Ticker: "AAPL", OrderType: "Stop", OrderDesc: "Buy 10 AAPL Stop 90, GTC", RemainingQuantity: 10, Status: "PreSubmitted"},
			}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/close?symbol=AAPL&position=1", nil)
	rr := httptest.NewRecorder()
	h.MarketSell(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMarketSell_ReturnsOnQuoteFailure(t *testing.T) {
	pos := &mockPositionService{
		getAllFn: func(ctx context.Context) ([]model.Position, []string, float64, error) {
			return []model.Position{{
				ContractDesc: "AAPL", Position: 10, Conid: 1, AcctId: "TEST",
			}}, []string{"AAPL"}, 1000, nil
		},
	}
	yahoo := &mockYahooService{
		stockFn: func(symbol, period, duration string) (model.Result2, model.Meta) {
			return model.Result2{}, model.Meta{}
		},
	}
	ibd := &mockIBDService{
		quotesFn: func(ctx context.Context, symbol string, currentDate time.Time) ([]model.Quote, error) {
			return nil, errors.New("ibd down")
		},
	}
	ibkr := &mockIBKRService{}
	h := newTestIbkr(ibkr, pos, yahoo, ibd)

	req := httptest.NewRequest(http.MethodPost, "/close?symbol=AAPL&position=1", nil)
	rr := httptest.NewRecorder()
	h.MarketSell(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "ibd down")
	assert.Nil(t, ibkr.lastOrders)
}

func TestMarketSell_UpdatePositionOnlyExitLegs(t *testing.T) {
	pos := &mockPositionService{
		getAllFn: func(ctx context.Context) ([]model.Position, []string, float64, error) {
			return []model.Position{{
				ContractDesc: "AAPL", Position: 10, Conid: 1, AcctId: "TEST",
			}}, []string{"AAPL"}, 1000, nil
		},
	}
	ibkr := &mockIBKRService{
		getAllFn: func(ctx context.Context, tickers map[string]interface{}) (model.AllOrders, error) {
			return model.AllOrders{Orders: []model.OrderDetails{
				{Conid: 1, Ticker: "AAPL", OrderType: "Limit", OrderDesc: "Buy 10 AAPL Limit 95, GTC",
					RemainingQuantity: 10, Price: "95", Account: "TEST", TimeInForce: "GTC", Side: "BUY", OrderId: 1, Status: "Submitted"},
				{Conid: 1, Ticker: "AAPL", OrderType: "Stop", OrderDesc: "Sell 4 AAPL Stop 90, GTC",
					RemainingQuantity: 4, Price: "90", Account: "TEST", TimeInForce: "GTC", Side: "SELL", OrderId: 2, Status: "PreSubmitted"},
			}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	// close half → qty 5; available = 10-4 = 6 → close 5; remaining shares 5 → only sell stop resized
	req := httptest.NewRequest(http.MethodPost, "/close?symbol=AAPL&position=0.5", nil)
	rr := httptest.NewRecorder()
	h.MarketSell(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Len(t, ibkr.updatedOrders, 1)
	assert.Equal(t, 2, *ibkr.updatedOrders[0].OrderId)
	assert.Equal(t, 5, ibkr.updatedOrders[0].Quantity)
	assert.Equal(t, "SELL", ibkr.updatedOrders[0].Side)
}

// --- Order ---

func TestOrder_ValidateMaxRisk(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	body := `{"orders":[{"ticker":"AAPL","conid":1,"prices":"100/90","side":"BUY","orderType":"LMT","quantity":500,"position":0.25}]}`
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Order(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "quantity mismatch")
	assert.Nil(t, ibkr.lastOrders)
}

func TestOrder_SellRejectsQuantityMismatch(t *testing.T) {
	viper.Set("ACCOUNT_ID", "TEST")
	viper.Set("MARGIN", 0.003)
	ibkr := &mockIBKRService{
		listOrdersFn: func(ctx context.Context) ([]model.ListOrder, []string, error) {
			return nil, nil, nil
		},
	}
	pos := &mockPositionService{
		summaryFn: func(ctx context.Context) (model.Summary, error) {
			return model.Summary{EquityWithLoanValue: model.Equitywithloanvalue{Amount: 100000}}, nil
		},
	}
	h := newTestIbkr(ibkr, pos, nil, nil)

	// short: entry 100 / stop 110; derived qty 25 at pos 0.25 — 500 is a fat finger
	body := `{"orders":[{"ticker":"AAPL","conid":1,"prices":"100/110/80","side":"SELL","orderType":"LMT","quantity":500,"position":0.25}]}`
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Order(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeBody(t, rr).Message, "quantity mismatch")
	assert.Nil(t, ibkr.lastOrders)
}
