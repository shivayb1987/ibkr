package service

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"ibkr/model"
	"io"
	"net/http"
)

type Trades struct {
}

func NewTrades() Trades {
	return Trades{}
}

func (s Trades) Get(days int) ([]model.Trade, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	endpoint := fmt.Sprintf("%s%s", viper.GetString("BASEURL"), viper.GetString("TRADES"))
	if days > 0 {
		endpoint = fmt.Sprintf("%s%s?days=%d", viper.GetString("BASEURL"), viper.GetString("TRADES"), days)
	}
	response, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	if response.StatusCode > http.StatusOK {
		return []model.Trade{}, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("cannot get orders :%w", err)
		return []model.Trade{}, err
	}
	response.Body.Close()

	var trades []model.Trade
	parseErr := json.Unmarshal(bodyBytes, &trades)
	if parseErr != nil {
		fmt.Errorf("symbol error: %w", parseErr)
		return []model.Trade{}, err
	}

	return trades, nil
}
