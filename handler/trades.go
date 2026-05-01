package handler

import (
	"fmt"
	"ibkr/model"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type TradesService interface {
	Get(days int) ([]model.Trade, error)
}

type TimeService interface {
	Parse(format, value string) (time.Time, error)
	GetLocation(timeZone string) *time.Location
	CurrentTime() time.Time
}

type IBKRTrades struct {
	tradesService TradesService
	timeService   TimeService
}

func NewIBKRTrades(tradesService TradesService, timeService TimeService) IBKRTrades {
	return IBKRTrades{
		tradesService: tradesService,
		timeService:   timeService,
	}
}

func (h IBKRTrades) Get(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.ParseInt(r.URL.Query().Get("days"), 10, 32)

	trades, err := h.tradesService.Get(int(days))
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	tradesByDate := make(map[string][]model.Trade)
	for _, trade := range trades {
		tradeDate := strings.Split(trade.TradeTime, "-")[0]
		tradeDateTime, _ := time.Parse("20060102", tradeDate)
		tradeDate = tradeDateTime.Format("2006/01/02")
		if _, ok := tradesByDate[tradeDate]; !ok {
			tradesByDate[tradeDate] = make([]model.Trade, 0)
		}

		ok, index := isExists(trade, tradesByDate[tradeDate])
		if ok {
			tradesByDate[tradeDate][index].Size += trade.Size
		} else {
			tradesByDate[tradeDate] = append(tradesByDate[tradeDate], trade)
		}
	}

	tradesByDay := make(map[string][]string)
	for tradeDate, trades := range tradesByDate {
		for _, trade := range trades {
			tradeStr := fmt.Sprintf("rec %s %d %s %s", trade.Symbol, int(trade.Size), trade.Price, tradeDate)
			if strings.ToLower(trade.Side) == "s" {
				tradeStr = fmt.Sprintf("rec %s %d %s %s", trade.Symbol, -int(trade.Size), trade.Price, tradeDate)
			}
			if _, ok := tradesByDay[tradeDate]; !ok {
				tradesByDay[tradeDate] = make([]string, 0)
			}
			tradesByDay[tradeDate] = append(tradesByDay[tradeDate], tradeStr)
		}
	}
	Success(w, r, map[string]interface{}{
		"list":   tradesByDay,
		"trades": trades,
	})
}

func isExists(trade model.Trade, trades []model.Trade) (bool, int) {
	for i, t := range trades {
		if t.Symbol == trade.Symbol && t.Side == trade.Side {
			return true, i
		}
	}

	return false, -1
}
