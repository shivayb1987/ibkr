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
	"net/http"
	"net/http/cookiejar"
)

type Alert struct {
	httpClient *http.Client
}

func NewAlert(client *http.Client) Alert {
	return Alert{
		httpClient: client,
	}
}

func (s Alert) Create(ctx context.Context, alert model.Alert) (interface{}, error) {
	alert.SendMessage = 1
	alert.Email = viper.GetString("EMAIL")
	alert.TIF = "GTC"
	alert.AlertMessage = fmt.Sprintf("%s at %.2s", alert.Ticker, alert.Conditions[0].Value)
	alert.AlertName = fmt.Sprintf(alert.AlertName)
	//alert.AlertRepeatable = 1

	requestJSON, err := Marshal(alert)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s%s", viper.GetString("BASEURL"), viper.GetString("ALERT"))

	s.httpClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	s.httpClient.Transport.(*http.Transport).DisableKeepAlives = true
	jar, _ := cookiejar.New(nil)
	s.httpClient.Jar = jar
	response, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(requestJSON))

	if err != nil {
		return nil, err
	}
	if response.StatusCode > http.StatusOK {
		return []model.IBKRAlert{}, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("cannot get orders :%w", err)
		return []model.IBKRAlert{}, err
	}
	response.Body.Close()

	return string(bodyBytes), err
}

func (s Alert) GetAll(ctx context.Context) ([]model.IBKRAlert, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	response, err := http.Get(fmt.Sprintf("%s%s", viper.GetString("BASEURL"), viper.GetString("ALERTS")))
	if err != nil {
		return nil, err
	}
	if response.StatusCode > http.StatusOK {
		return []model.IBKRAlert{}, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("cannot get orders :%w", err)
		return []model.IBKRAlert{}, err
	}
	response.Body.Close()

	var alerts []model.IBKRAlert
	parseErr := json.Unmarshal(bodyBytes, &alerts)
	if parseErr != nil {
		fmt.Errorf("symbol error: %w", parseErr)
		return []model.IBKRAlert{}, err
	}

	return alerts, nil
}

func Marshal(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}
