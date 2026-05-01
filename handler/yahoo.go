package handler

import (
	"ibkr/model"
	"math"
	"net/http"
	"strings"
)

var scrIDs = map[string]string{
	//"undervaluedLargeCaps":      "undervalued_large_caps",
	"mostActive": "most_actives",
	//"undervalued_growth_stocks": "undervalued_growth_stocks",
	"growth_technology_stocks": "growth_technology_stocks",
	"day_gainers":              "day_gainers",
	"small_cap_gainers":        "small_cap_gainers",
}
var scrIDs_ind = map[string]string{
	"mostActive": "d39e1287-bb5e-4e8b-9154-94fd3b162618",
	//"growth":                   "1a5303d9-5703-45cc-bcd7-6c96883869d2",
	//"undervalued_growth_stock": "84f6ae2a-54b7-4634-9cc3-026389643bca",
}

type Yahoo struct {
	yahooService YahooService
	symbols      []string
}

func NewYahoo(yahooService YahooService, symbols []string) Yahoo {
	return Yahoo{
		yahooService: yahooService,
		symbols:      symbols,
	}
}

func (h Yahoo) GetActiveStocks(w http.ResponseWriter, r *http.Request) {
	region := r.URL.Query().Get("region")
	count := 100
	var allQuotes []model.ActiveQuote
	var activeSymbols []string
	symbolsChecklist := make(map[string]string)
	//for _, s := range h.symbols {
	//	symbolsChecklist[strings.Split(s, ".")[0]] = s
	//}

	regionScreenIDs := scrIDs
	if region == "ind" {
		regionScreenIDs = scrIDs_ind
	}

	for _, scrID := range regionScreenIDs {
		for i := 0; ; i += count {
			quotes, err := h.yahooService.GetActiveStocks(i, i+count, scrID, region)
			if err != nil {
				Errors(w, r, http.StatusInternalServerError, err.Error())
				return
			}

			if len(quotes) == 0 {
				break
			}
			allQuotes = append(allQuotes, quotes...)
		}
	}

	for _, quote := range allQuotes {
		if _, ok := symbolsChecklist[strings.Split(quote.Symbol, ".")[0]]; !ok {
			if _, ok2 := symbolsChecklist[quote.Symbol]; !ok2 {
				if quote.MarketCap.Raw >= int64(2*math.Pow10(9)) && quote.RegularMarketVolume.Raw >= int(math.Pow10(6)) &&
					quote.RegularMarketPrice.Raw >= quote.TwoHundredDayAverage.Raw {
					activeSymbols = append(activeSymbols, quote.Symbol)
				}
			}
		}
	}
	finalList := make([]string, 0, len(activeSymbols))
	for _, ticker := range activeSymbols {
		if !strings.Contains(ticker, ".BO") {
			finalList = append(finalList, strings.Replace(ticker, "&", "_", -1))
		}
	}
	Success(w, r, map[string]interface{}{
		"symbols": strings.Replace(strings.Join(finalList, " "), "-", ".", -1),
	})
}
