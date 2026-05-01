package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"ibkr/model"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	errCancelOrder = errors.New("cannot cancel order")
	errNotExist    = errors.New("doesn't exist")
)

type IBKRAPI struct {
	httpClient *http.Client
}

func NewIBKRAPI(client *http.Client) IBKRAPI {
	return IBKRAPI{
		httpClient: client,
	}
}

func (s IBKRAPI) Search(ctx context.Context, symbol string) ([]model.Symbol, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	endpoint := fmt.Sprintf("%s%s?symbol=%s&secType=STK", viper.Get("BASEURL"), viper.GetString("SEARCH"), symbol)
	response, err := http.Get(endpoint)

	if err != nil {
		fmt.Errorf("symbol %s error: %w", symbol, err)
		return []model.Symbol{}, err
	}

	if response.StatusCode > http.StatusOK {
		fmt.Errorf("cannot get symbol %s", response.Status)

		return []model.Symbol{}, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", symbol, err)
		return []model.Symbol{}, err
	}
	response.Body.Close()

	if response.StatusCode > http.StatusOK {
		fmt.Errorf("cannot get symbol %s", string(bodyBytes))

		return []model.Symbol{}, errors.New(response.Status)
	}

	var data []model.Symbol
	parseErr := json.Unmarshal(bodyBytes, &data)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", symbol, parseErr)
		return []model.Symbol{}, parseErr
	}

	if len(data) == 0 {
		fmt.Printf("error: cannot get stockinfo for %s \n", symbol)

		return []model.Symbol{}, errors.New("no data found")
	}

	return sortBySections(data), nil
}

func (s IBKRAPI) Security(ctx context.Context, contractIDs []string) ([]model.Security, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	endpoint := fmt.Sprintf("%s%s?conids=%s&secType=STK", viper.Get("BASEURL"), viper.GetString("SECURITY"), strings.Join(contractIDs, ","))
	response, err := http.Get(endpoint)

	if err != nil {
		return []model.Security{}, err
	}

	if response.StatusCode > http.StatusOK {
		return []model.Security{}, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return []model.Security{}, err
	}
	response.Body.Close()

	if response.StatusCode > http.StatusOK {
		return []model.Security{}, errors.New(response.Status)
	}

	var data struct {
		Securities []model.Security `json:"secdef"`
	}
	parseErr := json.Unmarshal(bodyBytes, &data)
	if parseErr != nil {
		return []model.Security{}, parseErr
	}

	return data.Securities, nil
}

func sortBySections(symbols []model.Symbol) []model.Symbol {
	stkSymbols := make([]model.Symbol, 0, len(symbols))
	for _, symbol := range symbols {
		if len(symbol.Sections) > 0 && symbol.Sections[0].SecType == "STK" {
			stkSymbols = append(stkSymbols, symbol)
		}
	}
	sort.Slice(stkSymbols, func(i, j int) bool {
		return len(symbols[i].Sections) > len(symbols[j].Sections)
	})

	return stkSymbols
}

func (s IBKRAPI) Cancel(ctx context.Context, symbol, orderID string) ([]model.CancelOrder, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	orderRes, err := s.GetAll(ctx, nil)
	if err != nil {
		return []model.CancelOrder{}, fmt.Errorf("cannot get orders: %w", err)
	}

	targetOrders := make([]model.OrderDetails, 0)
	for _, order := range orderRes.Orders {
		if order.Ticker == symbol {
			targetOrders = append(targetOrders, order)
			if orderID != "" {
				orderIDInt, _ := strconv.ParseInt(orderID, 10, 64)
				targetOrders = make([]model.OrderDetails, 0)
				targetOrders = append(targetOrders, order)
				targetOrders[0].OrderId = int(orderIDInt)
			}
		}

	}

	if len(targetOrders) == 0 {
		return []model.CancelOrder{}, errNotExist
	}

	responses := make([]model.CancelOrder, 0)
	for _, targetOrder := range targetOrders {
		endpoint := fmt.Sprintf("%s%s/%d", viper.GetString("BASEURL"), viper.GetString("CANCEL"), targetOrder.OrderId)
		req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
		if err != nil {
			fmt.Errorf("failed to form cancel request: %w", err)
			continue
		}
		client := &http.Client{}
		ctx1, cancel := context.WithTimeout(req.Context(), 20*time.Second)
		defer cancel()
		req = req.WithContext(ctx1)

		log.Printf("cancel: %d", targetOrder.OrderId)
		res, err := client.Do(req)
		if err != nil {
			fmt.Errorf("failed to form cancel request: %w", err)
			continue
		}

		if res.StatusCode > http.StatusOK {
			fmt.Errorf("cannot cancel %s, %s", targetOrder.Ticker, res.Status)
			continue
		}

		parsedRes, err := s.parseCancelResponse(res)

		responses = append(responses, parsedRes)
	}

	return responses, nil
}

func (s IBKRAPI) ListOrders(ctx context.Context) ([]model.ListOrder, []string, error) {
	allOrders, err := s.GetAll(ctx, nil)
	if err != nil {
		fmt.Errorf("cannot get orders: %w", err)
		return nil, nil, err
	}
	listOrders := make([]model.ListOrder, 0, len(allOrders.Orders))
	for _, order := range allOrders.Orders {
		if order.Status == "Cancelled" || order.Status == "Filled" {
			continue
		}

		//stopLossPercent, targetPercent := s.getStopLossTarget(ctx, order)
		price, _ := strconv.ParseFloat(order.Price, 64)
		listOrders = append(listOrders, model.ListOrder{
			OrderId:   order.OrderId,
			Ticker:    order.Ticker,
			OrderDesc: order.OrderDesc,
			Quantity:  order.RemainingQuantity,
			Status:    order.Status,
			Price:     price,
			OrderType: order.OrderType,
			//StopLossPercent: fmt.Sprintf("%0.2f", stopLossPercent),
			//TargetPercent:   fmt.Sprintf("%0.2f", targetPercent),
		})
	}

	listOrders = groupByTicker(listOrders)
	symbols := make([]string, 0, len(listOrders))
	symbolsCheck := make(map[string]bool)
	for _, order := range listOrders {
		if _, ok := symbolsCheck[order.Ticker]; !ok {
			symbols = append(symbols, order.Ticker)
			symbolsCheck[order.Ticker] = true
		}
	}

	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i] < symbols[j]
	})

	return listOrders, symbols, nil
}

func (s IBKRAPI) getStopLossTarget(ctx context.Context, order model.OrderDetails) (float64, float64, float64) {
	close := 0.0
	if order.Side == "BUY" {
		return 0.0, 0.0, 0.0
	}

	marketData, err := s.MarketData(ctx, fmt.Sprintf("%d", order.Conid))
	if err == nil {
		close = marketData.Data[len(marketData.Data)-1].C
	}

	stopLoss := 0.0
	if order.OrderType == "Stop" {
		strValue := strings.Split(order.OrderDesc, " ")[3]
		stopLoss, err = strconv.ParseFloat(strings.Replace(strValue, ",", "", 1), 64)
		if err != nil {
			fmt.Errorf("error: %w", err)
		}
	}

	target := 0.0
	if order.OrderType == "Limit" {
		target, _ = strconv.ParseFloat(order.Price, 64)
	}

	stopLossPercent := 0.0
	if stopLoss > 0 {
		stopLossPercent = (close / stopLoss) - 1
	}

	targetPercent := 0.0
	if target > 0 {
		targetPercent = (target / close) - 1
	}
	return stopLossPercent * 100, targetPercent * 100, close
}

func (s IBKRAPI) MarketData(ctx context.Context, conId string) (model.MarketData, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	endpoint := fmt.Sprintf("%s%s?conid=%s&period=30d&bar=1d&outsideRth=false", viper.Get("BASEURL"), viper.GetString("MARKET_DATA"), conId)
	response, err := http.Get(endpoint)

	if err != nil {
		return model.MarketData{}, fmt.Errorf("cannot get market data: %w", err)
	}

	if response.StatusCode > http.StatusOK {
		return model.MarketData{}, fmt.Errorf("cannot get market data: %s", response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return model.MarketData{}, fmt.Errorf("cannot parse response :%w", err)
	}
	response.Body.Close()

	var data model.MarketData
	parseErr := json.Unmarshal(bodyBytes, &data)
	if parseErr != nil {
		fmt.Errorf("symbol error: %w", parseErr)
		return model.MarketData{}, parseErr
	}

	return data, nil
}

func groupByTicker(orders []model.ListOrder) []model.ListOrder {

	// Sorting orders by ticker
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].Ticker < orders[j].Ticker
	})

	//groupedOrders := make(map[string][]model.ListOrder)
	//for _, order := range orders {
	//	groupedOrders[order.Ticker] = append(groupedOrders[order.Ticker], order)
	//}
	//
	//listOrders := make([]model.ListOrder, 0)
	//for _, orders := range groupedOrders {
	//	listOrders = append(listOrders, orders...)
	//
	//}

	return orders
}

func (s IBKRAPI) GetAll(ctx context.Context, filters map[string]interface{}) (model.AllOrders, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	endpoint := fmt.Sprintf("%s%s", viper.Get("BASEURL"), viper.GetString("ORDERS"))
	response, err := http.Get(endpoint)

	if err != nil {
		fmt.Errorf("cannot get orders: %w", err)
		return model.AllOrders{}, err
	}

	if response.StatusCode > http.StatusOK {
		fmt.Errorf("cannot get order: %s", response.Status)

		return model.AllOrders{}, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("cannot get orders :%w", err)
		return model.AllOrders{}, err
	}
	response.Body.Close()

	var data model.AllOrders
	parseErr := json.Unmarshal(bodyBytes, &data)
	if parseErr != nil {
		fmt.Errorf("symbol error: %w", parseErr)
		return model.AllOrders{}, parseErr
	}

	tickers, _ := filters["tickers"].([]string)

	if len(tickers) == 0 {
		return data, nil
	}

	return s.filterOrdersByTickers(data.Orders, tickers), nil
}

func (s IBKRAPI) filterOrdersByTickers(orders []model.OrderDetails, tickers []string) model.AllOrders {
	filteredOrders := make([]model.OrderDetails, 0, len(orders))
	tickersMap := make(map[string]bool)
	for _, ticker := range tickers {
		tickersMap[ticker] = true
	}

	for _, order := range orders {
		if order.Status != "Cancelled" && order.Status != "Inactive" && order.Status != "Filled" {
			if tickersMap[order.Ticker] {
				filteredOrders = append(filteredOrders, order)
			}
		}
	}

	var prices string = "xxx/stopLoss/target"
	for index, order := range filteredOrders {
		ss, tt, close := s.getStopLossTarget(context.TODO(), order)
		filteredOrders[index].StopLossPercent = fmt.Sprintf("%0.2f", ss)
		filteredOrders[index].TargetPercent = fmt.Sprintf("%0.2f", tt)
		if ss > 0 {
			filteredOrders[index].PotentialLoss = fmt.Sprintf("%0.2f", -close*order.RemainingQuantity*ss/100)
		}
		if order.OrderType == "Limit" {
			strValue := strings.Split(order.OrderDesc, " ")[3]
			target, _ := strconv.ParseFloat(strings.Replace(strValue, ",", "", 1), 64)
			prices = strings.Replace(prices, "target", fmt.Sprintf("%f", target), 1)
		}
		if order.OrderType == "Stop" {
			strValue := strings.Split(order.OrderDesc, " ")[3]
			stopLoss, _ := strconv.ParseFloat(strings.Replace(strValue, ",", "", 1), 64)
			prices = strings.Replace(prices, "stopLoss", fmt.Sprintf("%f", stopLoss), 1)
		}
	}

	for index, _ := range filteredOrders {
		filteredOrders[index].Prices = prices
	}

	return model.AllOrders{Orders: filteredOrders}
}

func (s IBKRAPI) Update(ctx context.Context, order model.Order) (model.OrderResponse, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	orderRes, err := s.GetAll(ctx, nil)
	if err != nil {
		return model.OrderResponse{}, fmt.Errorf("cannot get orders: %w", err)
	}
	var targetOrder model.OrderDetails

	for _, o := range orderRes.Orders {
		if order.OrderId != nil && o.OrderId == *order.OrderId {
			targetOrder = o
			break
		}
		if o.Ticker == order.Ticker {
			targetOrder = o
		}
	}

	if targetOrder.Ticker == "" {
		return model.OrderResponse{}, errNotExist
	}

	order.ConID = targetOrder.Conid
	order.AcctID = targetOrder.Acct
	order.OrderId = nil

	var orderResponse model.OrderResponse
	requestJSON, err := json.Marshal(order)
	if err != nil {
		return orderResponse, err
	}

	log.Printf("update %s: %s", order.Ticker, string(requestJSON))
	res, err := http.Post(fmt.Sprintf("%s%s/%d", viper.GetString("BASEURL"), viper.GetString("MODIFY"), targetOrder.OrderId), "application/json", bytes.NewBuffer(requestJSON))
	if err != nil {
		return orderResponse, err
	}

	if res.StatusCode > http.StatusOK {
		return model.OrderResponse{}, errors.New(res.Status)
	}

	parsedRes, err := s.parseCreateResponse(res)
	if err != nil {
		return model.OrderResponse{}, err
	}

	for _, parsed := range parsedRes {
		if parsed.Id != "" {
			confirmed, err := json.Marshal(model.Reply{
				Confirmed: true,
			})
			replyRes, err := http.Post(
				fmt.Sprintf("%s%s/%s", viper.GetString("BASEURL"), viper.GetString("REPLY"), parsed.Id),
				"application/json",
				bytes.NewBuffer(confirmed))
			if err != nil {
				return orderResponse, err
			}

			return s.parseCreateResponse(replyRes)
		}
	}

	return parsedRes, nil
}

func (s IBKRAPI) Order(ctx context.Context, orders []model.Order) (model.OrderResponse, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	ibkrOrder := model.IBKROrder{
		Orders: orders,
	}
	var orderResponse model.OrderResponse
	requestJSON, err := json.Marshal(ibkrOrder)
	if err != nil {
		return orderResponse, err
	}

	log.Printf("order %s: %s", orders[0].Ticker, string(requestJSON))
	res, err := http.Post(fmt.Sprintf("%s%s", viper.GetString("BASEURL"), viper.GetString("ORDER")), "application/json", bytes.NewBuffer(requestJSON))
	if err != nil {
		return orderResponse, err
	}

	if res.StatusCode > http.StatusOK {
		return model.OrderResponse{}, errors.New(res.Status)
	}

	parsedRes, err := s.parseCreateResponse(res)
	if err != nil {
		return nil, err
	}
	return s.retry(parsedRes, 0)
}

func (s IBKRAPI) retry(res model.OrderResponse, count int) (model.OrderResponse, error) {
	var response model.OrderResponse = res
	for _, parsed := range response {
		if parsed.Id != "" {
			replyRes, _ := s.Reply(parsed.Id)
			response = replyRes
			if count == 10 {
				return response, nil
			}
			count++
			response, _ = s.retry(response, count)
		}
	}

	return response, nil
}

func (s IBKRAPI) Init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	url := fmt.Sprintf("%s%s", viper.GetString("BASEURL"), viper.GetString("INIT"))

	type Session struct {
		Passed        bool   `json:"passed"`
		Authenticated bool   `json:"authenticated"`
		Connected     bool   `json:"connected"`
		Competing     bool   `json:"competing"`
		HardwareInfo  string `json:"hardware_info"`
	}
	requestJSON, _ := json.Marshal(`{}`)
	for i := 0; i < 5; i++ {
		res, err := http.Post(
			url,
			"application/json",
			bytes.NewBuffer(requestJSON),
		)
		if err != nil {
			continue
		}
		bodyBytes, err := io.ReadAll(res.Body)
		bodyString := string(bodyBytes)
		if err != nil {
		}

		var data Session
		parseErr := json.Unmarshal([]byte(bodyString), &data)
		if parseErr != nil {
			continue
		}

		if data.Authenticated {
			break
		}
	}
}

func (s IBKRAPI) Reply(replyID string) (model.OrderResponse, error) {
	confirmed, err := json.Marshal(model.Reply{
		Confirmed: true,
	})
	var orderResponse model.OrderResponse
	replyRes, err := http.Post(
		fmt.Sprintf("%s%s/%s", viper.GetString("BASEURL"), viper.GetString("REPLY"), replyID),
		"application/json",
		bytes.NewBuffer(confirmed))
	if err != nil {
		return orderResponse, err
	}

	return s.parseCreateResponse(replyRes)
}

func (s IBKRAPI) parseCreateResponse(res *http.Response) (model.OrderResponse, error) {
	bodyBytes, err := io.ReadAll(res.Body)
	bodyString := string(bodyBytes)
	if err != nil {
		fmt.Printf("cannot place order: %s\n", bodyString)
		return model.OrderResponse{}, fmt.Errorf("%w", err)
	}

	var data model.OrderResponse
	parseErr := json.Unmarshal([]byte(bodyString), &data)
	if parseErr != nil {
		return model.OrderResponse{}, fmt.Errorf("%s:%w", bodyString, parseErr)
	}

	return data, nil
}

func (s IBKRAPI) parseCancelResponse(res *http.Response) (model.CancelOrder, error) {
	bodyBytes, err := io.ReadAll(res.Body)
	bodyString := string(bodyBytes)
	if err != nil {
		fmt.Printf("cannot cancel order: %s\n", bodyString)
		return model.CancelOrder{}, fmt.Errorf("%w", err)
	}

	var data model.CancelOrder
	parseErr := json.Unmarshal([]byte(bodyString), &data)
	if parseErr != nil {
		return model.CancelOrder{}, fmt.Errorf("%w", err)
	}

	return data, nil
}

func logging(value interface{}) {
	valBytes, err := json.Marshal(value)
	if err != nil {
		log.Printf("cannot marshal value: %v", err)
	}

	log.Println(string(valBytes))
}
