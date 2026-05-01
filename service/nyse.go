package service

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"ibkr/model"
	"io"
	"net/http"
)

type NYSE struct {
}

func NewNYSE() NYSE {
	return NYSE{}
}

func (s NYSE) Quotes(payload model.QuotesPayload) ([]model.NYSE, error) {
	endpoint := "https://www.nyse.com/api/quotes/filter"
	reqBytes, err := json.Marshal(payload)
	if err != nil {
		return []model.NYSE{}, err
	}
	var quotes []model.NYSE
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(reqBytes))
	if err != nil {
		return quotes, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return quotes, err
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return quotes, err
	}
	response.Body.Close()

	var res []model.NYSE
	parseErr := json.Unmarshal(bodyBytes, &res)
	if parseErr != nil {
		return quotes, err
	}

	for _, quote := range res {
		if quote.InstrumentType == "COMMON_STOCK" {
			quotes = append(quotes, quote)
		}
	}

	return quotes, nil
}
