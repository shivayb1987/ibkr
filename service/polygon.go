package service

import (
	"encoding/json"
	"fmt"
	"ibkr/model"
	"io"
	"net/http"
)

type Polygon struct {
	httpClient *http.Client
	apiKey     string
}

func NewPolygon(httpClient *http.Client, apiKey string) Polygon {
	return Polygon{
		httpClient: httpClient,
		apiKey:     apiKey,
	}
}

type TickerDetailsResponse struct {
	Data model.PolygonTickerDetails `json:"results"`
}

func (s Polygon) Summary(ticker string) (model.PolygonTickerDetails, error) {
	//http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	response, err := http.Get(fmt.Sprintf("https://api.polygon.io/v3/reference/tickers/%s?apiKey=%s", ticker, s.apiKey))
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return model.PolygonTickerDetails{}, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return model.PolygonTickerDetails{}, err
	}
	response.Body.Close()

	var res TickerDetailsResponse
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", ticker, parseErr)
		return model.PolygonTickerDetails{}, err
	}

	return res.Data, nil
}
