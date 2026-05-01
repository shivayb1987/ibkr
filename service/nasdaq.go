package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"ibkr/model"
	"io"
	"net/http"
)

type Nasdaq struct {
	httpClient *http.Client
	retryCount int
}

func NewNasdaq(httpClient *http.Client) Nasdaq {
	return Nasdaq{
		httpClient: httpClient,
	}
}

type StockSummaryResponse struct {
	Data   model.StockSummary `json:"data"`
	Status struct {
		RCode        int `json:"rCode"`
		BCodeMessage []struct {
			Code         int    `json:"code"`
			ErrorMessage string `json:"errorMessage"`
		} `json:"bCodeMessage"`
	} `json:"status"`
}

type InsiderActivityResponse struct {
	Data   model.InsiderActivity `json:"data"`
	Status struct {
		RCode        int `json:"rCode"`
		BCodeMessage []struct {
			Code         int    `json:"code"`
			ErrorMessage string `json:"errorMessage"`
		} `json:"bCodeMessage"`
	} `json:"status"`
}
type NewsResponse struct {
	Data struct {
		Rows []model.NewsRow `json:"rows"`
	} `json:"data"`
}

func (s Nasdaq) Summary(ticker string, assetClass string) (model.StockSummary, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.nasdaq.com/api/quote/%s/summary?assetclass=%s", ticker, assetClass), nil)
	if err != nil {
		return model.StockSummary{}, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")
	//
	response, err := client.Do(req)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return model.StockSummary{}, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return model.StockSummary{}, err
	}
	response.Body.Close()

	var res StockSummaryResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", ticker, parseErr)
		return model.StockSummary{}, err
	}

	if res.Status.RCode >= http.StatusBadRequest && s.retryCount == 0 {
		s.retryCount++
		return s.Summary(ticker, "etf")
	}

	return res.Data, nil
}

func (s Nasdaq) InsiderActivity(ticker string) (model.InsiderActivity, error) {
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/company/%s/insider-trades?limit=10&type=all&sortColumn=lastDate&sortOrder=DESC", ticker)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return model.InsiderActivity{}, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	response, err := client.Do(req)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return model.InsiderActivity{}, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return model.InsiderActivity{}, err
	}
	response.Body.Close()

	var res InsiderActivityResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", ticker, parseErr)
		return model.InsiderActivity{}, err
	}

	return res.Data, nil

}

func (s Nasdaq) News(ticker string) ([]model.NewsRow, error) {
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/news/topic/articlebysymbol?q=%s|STOCKS&offset=0&limit=10&fallback=true", ticker)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return []model.NewsRow{}, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	response, err := client.Do(req)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return []model.NewsRow{}, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return []model.NewsRow{}, err
	}
	response.Body.Close()

	var res NewsResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", ticker, parseErr)
		return []model.NewsRow{}, err
	}

	return res.Data.Rows, nil

}

type EarningsResponse struct {
	Data    model.Earnings `json:"data"`
	Message interface{}    `json:"message"`
	Status  struct {
		RCode            int         `json:"rCode"`
		BCodeMessage     interface{} `json:"bCodeMessage"`
		DeveloperMessage interface{} `json:"developerMessage"`
	} `json:"-"`
}

type EarningsSurpriseResponse struct {
	Data struct {
		Symbol                string `json:"symbol"`
		EarningsSurpriseTable struct {
			Headers struct {
				FiscalQtrEnd       string `json:"fiscalQtrEnd"`
				DateReported       string `json:"dateReported"`
				Eps                string `json:"eps"`
				ConsensusForecast  string `json:"consensusForecast"`
				PercentageSurprise string `json:"percentageSurprise"`
			} `json:"headers"`
			Rows []model.EarningSuprises `json:"rows"`
		} `json:"earningsSurpriseTable"`
	} `json:"data"`
}
type EarningsForecastResponse struct {
	Data struct {
		Symbol            string `json:"symbol"`
		QuarterlyForecast struct {
			AsOf    interface{} `json:"asOf"`
			Headers struct {
				FiscalEnd            string `json:"fiscalEnd"`
				ConsensusEPSForecast string `json:"consensusEPSForecast"`
				HighEPSForecast      string `json:"highEPSForecast"`
				LowEPSForecast       string `json:"lowEPSForecast"`
				NoOfEstimates        string `json:"noOfEstimates"`
				Up                   string `json:"up"`
				Down                 string `json:"down"`
			} `json:"headers"`
			Rows []model.EarningsForecast `json:"rows"`
		} `json:"quarterlyForecast"`
		YearlyForecast struct {
			AsOf    interface{} `json:"asOf"`
			Headers struct {
				FiscalEnd            string `json:"fiscalEnd"`
				ConsensusEPSForecast string `json:"consensusEPSForecast"`
				HighEPSForecast      string `json:"highEPSForecast"`
				LowEPSForecast       string `json:"lowEPSForecast"`
				NoOfEstimates        string `json:"noOfEstimates"`
				Up                   string `json:"up"`
				Down                 string `json:"down"`
			} `json:"headers"`
			Rows []model.EarningsForecast `json:"rows"`
		} `json:"yearlyForecast"`
	} `json:"data"`
}
type RevenuesResponse struct {
	Data struct {
		Title        string `json:"title"`
		RevenueTable struct {
			AsOf    interface{} `json:"asOf"`
			Headers struct {
				Value1 string `json:"value1"`
				Value2 string `json:"value2"`
				Value3 string `json:"value3"`
				Value4 string `json:"value4"`
			} `json:"headers"`
			Rows []model.Revenue `json:"rows"`
		} `json:"revenueTable"`
	} `json:"data"`
}

type CalendarResponse struct {
	Data struct {
		AsOf    string `json:"asOf"`
		Headers struct {
			Time                string `json:"time"`
			Symbol              string `json:"symbol"`
			Name                string `json:"name"`
			MarketCap           string `json:"marketCap"`
			FiscalQuarterEnding string `json:"fiscalQuarterEnding"`
			EpsForecast         string `json:"epsForecast"`
			NoOfEsts            string `json:"noOfEsts"`
			LastYearRptDt       string `json:"lastYearRptDt"`
			LastYearEPS         string `json:"lastYearEPS"`
		} `json:"headers"`
		Rows []model.Calendar `json:"rows"`
	} `json:"data"`
}

func (s Nasdaq) Earnings(ticker string) (model.Earnings, error) {
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/analyst/%s/earnings-date", ticker)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return model.Earnings{}, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	response, err := client.Do(req)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return model.Earnings{}, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return model.Earnings{}, err
	}
	response.Body.Close()

	var res EarningsResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", ticker, parseErr)
		return model.Earnings{}, err
	}

	return res.Data, nil

}

func (s Nasdaq) EarningSurprises(ticker string) ([]model.EarningSuprises, error) {
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/company/%s/earnings-surprise", ticker)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	var surprises []model.EarningSuprises
	if err != nil {
		return surprises, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	response, err := client.Do(req)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return surprises, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return surprises, err
	}
	response.Body.Close()

	var res EarningsSurpriseResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", ticker, parseErr)
		return surprises, err
	}

	return res.Data.EarningsSurpriseTable.Rows, nil

}
func (s Nasdaq) EarningsForecast(ticker string) ([]model.EarningsForecast, error) {
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/analyst/%s/earnings-forecast", ticker)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	var surprises []model.EarningsForecast
	if err != nil {
		return surprises, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	response, err := client.Do(req)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return surprises, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return surprises, err
	}
	response.Body.Close()

	var res EarningsForecastResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", ticker, parseErr)
		return surprises, err
	}

	return res.Data.QuarterlyForecast.Rows, nil

}
func (s Nasdaq) Revenues(ticker string) (model.Revenue, error) {
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/company/%s/revenue?limit=1", ticker)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	var revenues model.Revenue
	if err != nil {
		return revenues, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	response, err := client.Do(req)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return revenues, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return revenues, err
	}
	response.Body.Close()

	var res RevenuesResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", ticker, parseErr)
		return revenues, err
	}

	for i, row := range res.Data.RevenueTable.Rows {
		if row.Value1 == "Totals" {
			revenues = res.Data.RevenueTable.Rows[i+1]
			break
		}
	}

	return revenues, nil
}
func (s Nasdaq) Calendar(date string) (string, []model.Calendar, error) {
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/calendar/earnings?date=%s", date)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	var earnings []model.Calendar
	var asOf string
	if err != nil {
		return asOf, earnings, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	response, err := client.Do(req)
	if err != nil {
		return asOf, earnings, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return asOf, earnings, err
	}
	response.Body.Close()

	var res CalendarResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		return asOf, earnings, err
	}

	return res.Data.AsOf, res.Data.Rows, nil
}

type ActiveStocksResponse struct {
	Data struct {
		Table struct {
			Rows []model.ActiveStock `json:"rows"`
		}
		TotalRecords int `json:"totalrecords"`
	} `json:"data"`
	Message string `json:"message"`
	Status  struct {
		RCode            int         `json:"rCode"`
		BCodeMessage     interface{} `json:"bCodeMessage"`
		DeveloperMessage interface{} `json:"developerMessage"`
	} `json:"-"`
}

func (s Nasdaq) Active(offset, limit int) ([]model.ActiveStock, int, error) {
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/screener/stocks?tableonly=true&limit=%d&offset=%d", limit, offset)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	activeStocks := make([]model.ActiveStock, 0)
	if err != nil {
		return activeStocks, 0, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	response, err := client.Do(req)
	if err != nil {
		fmt.Errorf("cannot get active stocks error: %w", err)
		return activeStocks, 0, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol error: %w", err)
		return activeStocks, 0, err
	}
	response.Body.Close()

	var res ActiveStocksResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		return activeStocks, 0, err
	}

	return res.Data.Table.Rows, res.Data.TotalRecords, nil

}
