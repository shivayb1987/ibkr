package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"ibkr/model"
	"io"
	"net/http"
	"time"
)

type IBD struct {
	httpClient *http.Client
	retryCount int
}

func NewIBD(httpClient *http.Client) IBD {
	return IBD{
		httpClient: httpClient,
	}
}

func (s IBD) Checkup(ticker string) (model.Checkup, error) {
	result := model.Checkup{}
	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://myibd.investors.com/searchapi/checkup/%s", ticker), nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("User-Agent", "insomnia/9.1.0")
	//
	response, err := client.Do(req)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return result, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("symbol %s error: %w", ticker, err)
		return result, err
	}
	response.Body.Close()

	var res model.Checkup
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		fmt.Errorf("symbol %s error: %w", ticker, parseErr)
		return result, err
	}

	return res, nil
}

// GetStockQuotes fetches stock data from the API and returns a slice of Quote
func (s IBD) GetStockQuotes(ctx context.Context, symbol string, currentDate time.Time) ([]model.Quote, error) {
	// Derive startDate: 222 days before endDate
	endDate := currentDate.UTC()
	startDate := endDate.AddDate(0, 0, -730).Format("2006-01-02") // e.g., 2025-07-09 -> 2024-11-30
	endDateStr := endDate.Format("2006-01-02T15:04:05.999Z")      // e.g., 2025-07-09T15:26:39.794Z

	// Prepare the request payload
	payload := map[string]interface{}{
		"req": map[string]interface{}{
			"Symbol":     symbol,
			"Type":       1,
			"StartDate":  startDate,
			"EndDate":    endDateStr,
			"EnableBats": true,
		},
	}

	// Convert payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create retryable HTTP client
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 1 * time.Second
	client.RetryWaitMax = 5 * time.Second

	// Create POST request
	req, err := retryablehttp.NewRequestWithContext(ctx, "POST", "https://research.investors.com/services/ChartService.svc/GetData", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var stockData model.StockData
	if err := json.Unmarshal(body, &stockData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for error in response
	if stockData.GetDataResult.ErrorMessage != "" {
		return nil, fmt.Errorf("API error: %s", stockData.GetDataResult.ErrorMessage)
	}

	// Convert Series to Quote
	quote := model.Quote{
		Volume: make([]int64, len(stockData.GetDataResult.Series)),
		Close:  make([]float64, len(stockData.GetDataResult.Series)),
		Open:   make([]float64, 0), // Open not provided in Series
		High:   make([]float64, len(stockData.GetDataResult.Series)),
		Low:    make([]float64, len(stockData.GetDataResult.Series)),
	}

	n := len(stockData.GetDataResult.Series)
	for i, series := range stockData.GetDataResult.Series {
		quote.Volume[n-1-i] = int64(series.Volume)
		quote.Close[n-1-i] = series.Close
		quote.High[n-1-i] = series.High
		quote.Low[n-1-i] = series.Low
	}

	return []model.Quote{quote}, nil
}
