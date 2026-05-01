package handler

import (
	"fmt"
	"ibkr/model"
	"net/http"
	"strings"
)

type NasdaqService interface {
	Summary(ticker string, assetClass string) (model.StockSummary, error)
	InsiderActivity(ticker string) (model.InsiderActivity, error)
	News(ticker string) ([]model.NewsRow, error)
	Earnings(ticker string) (model.Earnings, error)
	EarningSurprises(ticker string) ([]model.EarningSuprises, error)
	EarningsForecast(ticker string) ([]model.EarningsForecast, error)
	Active(offset, limit int) ([]model.ActiveStock, int, error)
	Revenues(ticker string) (model.Revenue, error)
	Calendar(date string) (string, []model.Calendar, error)
}

type Nasdaq struct {
	nasdaqService NasdaqService
}

func NewNasdaq(nasdaqService NasdaqService) Nasdaq {
	return Nasdaq{
		nasdaqService: nasdaqService,
	}
}

func (h Nasdaq) Summary(w http.ResponseWriter, r *http.Request) {
	ticker := r.URL.Query().Get("ticker")
	data, err := h.nasdaqService.Summary(ticker, "stocks")
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	Success(w, r, data)
}

func (h Nasdaq) InsiderActivity(w http.ResponseWriter, r *http.Request) {
	ticker := r.URL.Query().Get("ticker")
	data, err := h.nasdaqService.InsiderActivity(ticker)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	Success(w, r, data)
}

func (h Nasdaq) News(w http.ResponseWriter, r *http.Request) {
	publishers := []string{
		"RTTNews",
		"Nasdaq.Com",
	}
	ticker := r.URL.Query().Get("ticker")
	data, err := h.nasdaqService.News(ticker)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}
	articles := make([]string, 0)
	for _, row := range data {
		if isContains(row.Publisher, publishers) {
			articles = append(articles, fmt.Sprintf("%s %s", row.Title, row.Url))
		}
	}

	Success(w, r, data)
}

func (h Nasdaq) Calendar(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	tickers := r.URL.Query()["ticker"]
	asOf, data, err := h.nasdaqService.Calendar(date)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	earnings := make([]model.Calendar, 0)
	befores := make([]string, 0)
	afters := make([]string, 0)
	for _, earning := range data {
		if isContains(earning.Symbol, tickers) {
			earning.AsOf = asOf
			earnings = append(earnings, earning)
		}
		if len(tickers) == 0 {
			if strings.Contains(earning.Time, "after") {
				afters = append(afters, earning.Symbol)
			} else {
				befores = append(befores, earning.Symbol)
			}
		}
	}

	if len(tickers) == 0 {
		earnings = data
	}

	Success(w, r, map[string]interface{}{
		"befores": strings.Join(befores, ","),
		"afters":  strings.Join(afters, ","),
		"data":    earnings,
	})
}

func (h Nasdaq) Earnings(w http.ResponseWriter, r *http.Request) {
	ticker := r.URL.Query().Get("ticker")
	data, err := h.nasdaqService.Earnings(ticker)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}
	Success(w, r, data)
}

func (h Nasdaq) Active(w http.ResponseWriter, r *http.Request) {
	count := 0
	var allQuotes []model.ActiveStock
	var activeSymbols []string
	stocks, totalRecords, err := h.nasdaqService.Active(count, 25)
	allQuotes = append(allQuotes, stocks...)
	for len(stocks) <= totalRecords {
		count += 25
		stocks, _, _ := h.nasdaqService.Active(count, 25)
		allQuotes = append(allQuotes, stocks...)
	}

	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	for _, quote := range allQuotes {
		activeSymbols = append(activeSymbols, quote.Symbol)
	}
	Success(w, r, activeSymbols)
}
