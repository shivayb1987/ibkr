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
	"net/http/cookiejar"
	"time"
)

const alertRequestTimeout = 60 * time.Second

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
	alert.AlertMessage = fmt.Sprintf("%s-%.2s", alert.Ticker, alert.Conditions[0].Value)
	alert.AlertName = fmt.Sprintf(alert.AlertName)
	//alert.AlertRepeatable = 1

	requestJSON, err := Marshal(alert)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s%s", viper.GetString("BASEURL"), viper.GetString("ALERT"))
	if deadline, ok := ctx.Deadline(); ok {
		log.Printf("alert create parent deadline in %s", time.Until(deadline).Round(time.Millisecond))
	}

	transport, ok := s.httpClient.Transport.(*http.Transport)
	if !ok || transport == nil {
		transport = &http.Transport{}
		s.httpClient.Transport = transport
	}
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	transport.DisableKeepAlives = true
	jar, _ := cookiejar.New(nil)
	s.httpClient.Jar = jar

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Console")

	log.Printf("alert create payload: %s", string(requestJSON))
	start := time.Now()
	requestCtx, cancel := context.WithTimeout(context.Background(), alertRequestTimeout)
	defer cancel()
	request = request.WithContext(requestCtx)

	response, err := s.httpClient.Do(request)

	if err != nil {
		log.Printf("alert create request failed after %s: %v", time.Since(start).Round(time.Millisecond), err)
		return nil, err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return []model.IBKRAlert{}, fmt.Errorf("cannot get orders: %w", err)
	}
	if response.StatusCode > http.StatusOK {
		log.Printf("alert create completed in %s with status %s", time.Since(start).Round(time.Millisecond), response.Status)
		if len(bytes.TrimSpace(bodyBytes)) > 0 {
			log.Printf("alert create failed: %s body=%s", response.Status, string(bytes.TrimSpace(bodyBytes)))
			return []model.IBKRAlert{}, fmt.Errorf("%s: %s", response.Status, string(bytes.TrimSpace(bodyBytes)))
		}
		return []model.IBKRAlert{}, errors.New(response.Status)
	}

	log.Printf("alert create completed in %s with status %s", time.Since(start).Round(time.Millisecond), response.Status)
	return string(bodyBytes), nil
}

func (s Alert) GetAll(ctx context.Context) ([]model.IBKRAlert, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	response, err := http.Get(fmt.Sprintf("%s%s", viper.GetString("BASEURL"), viper.GetString("ALERTS")))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode > http.StatusOK {
		return []model.IBKRAlert{}, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return []model.IBKRAlert{}, fmt.Errorf("cannot get orders: %w", err)
	}

	var alerts []model.IBKRAlert
	parseErr := json.Unmarshal(bodyBytes, &alerts)
	if parseErr != nil {
		return []model.IBKRAlert{}, fmt.Errorf("symbol error: %w", parseErr)
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
