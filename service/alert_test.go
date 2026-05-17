package service_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"ibkr/model"
	"ibkr/service"
)

func TestAlertCreate(t *testing.T) {
	t.Run("posts transformed non-email alert and returns response body", func(t *testing.T) {
		const (
			email      = "alerts@example.com"
			alertRoute = "/alerts"
		)

		var (
			gotMethod      string
			gotPath        string
			gotContentType string
			gotAccept      string
			gotUserAgent   string
			gotBody        []byte
			gotErr         error
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotContentType = r.Header.Get("Content-Type")
			gotAccept = r.Header.Get("Accept")
			gotUserAgent = r.Header.Get("User-Agent")
			gotBody, gotErr = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`created`))
		}))
		defer server.Close()

		restoreConfig := setAlertConfig(email, server.URL, alertRoute)
		defer restoreConfig()

		alertService := service.NewAlert(&http.Client{Transport: &http.Transport{}})
		cond := "<= 123.45"
		alert := model.Alert{
			Ticker:     "AAPL",
			Condition:  &cond,
			AlertName:  "breakout",
			Exchange:   "NASDAQ",
			Conditions: buildAlertConditions("265598@NASDAQ", "<=", "123.45"),
		}

		response, err := alertService.Create(context.Background(), alert)

		assert.NoError(t, err)
		assert.NoError(t, gotErr)
		assert.Equal(t, "created", response)
		assert.Equal(t, http.MethodPost, gotMethod)
		assert.Equal(t, alertRoute, gotPath)
		assert.Equal(t, "application/json", gotContentType)
		assert.Equal(t, "application/json", gotAccept)
		assert.Equal(t, "Console", gotUserAgent)

		var gotAlert model.Alert
		err = json.Unmarshal(gotBody, &gotAlert)
		assert.NoError(t, err)
		assert.Equal(t, "AAPL", gotAlert.Ticker)
		assert.Equal(t, "<= 123.45", gotAlert.Condition)
		assert.Equal(t, "breakout", gotAlert.AlertName)
		assert.Equal(t, "NASDAQ", gotAlert.Exchange)
		assert.Equal(t, 0, gotAlert.SendMessage)
		assert.Empty(t, gotAlert.Email)
		assert.Equal(t, "GTC", gotAlert.TIF)
		assert.Equal(t, "AAPL at 123.45", gotAlert.AlertMessage)
		assert.Len(t, gotAlert.Conditions, 1)
		assert.Equal(t, "265598@NASDAQ", gotAlert.Conditions[0].Conidex)
		assert.Equal(t, "n", gotAlert.Conditions[0].LogicBind)
		assert.Equal(t, "<=", gotAlert.Conditions[0].Operator)
		assert.Equal(t, "0", gotAlert.Conditions[0].TriggerMethod)
		assert.Equal(t, 1, gotAlert.Conditions[0].Type)
		assert.Equal(t, "123.45", gotAlert.Conditions[0].Value)
	})

	t.Run("fills email only when sendMessage is enabled", func(t *testing.T) {
		const (
			email      = "alerts@example.com"
			alertRoute = "/alerts"
		)

		var gotBody []byte

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`created`))
		}))
		defer server.Close()

		restoreConfig := setAlertConfig(email, server.URL, alertRoute)
		defer restoreConfig()

		alertService := service.NewAlert(&http.Client{Transport: &http.Transport{}})
		cond := "<= 123.45"
		alert := model.Alert{
			Ticker:      "AAPL",
			Condition:   &cond,
			AlertName:   "breakout",
			SendMessage: 1,
			Conditions:  buildAlertConditions("265598@NASDAQ", "<=", "123.45"),
		}

		_, err := alertService.Create(context.Background(), alert)
		assert.NoError(t, err)

		var gotAlert model.Alert
		err = json.Unmarshal(gotBody, &gotAlert)
		assert.NoError(t, err)
		assert.Equal(t, 1, gotAlert.SendMessage)
		assert.Equal(t, email, gotAlert.Email)
	})

	t.Run("returns error when downstream request fails", func(t *testing.T) {
		const (
			email      = "alerts@example.com"
			alertRoute = "/alerts"
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"forbidden"}`))
		}))
		defer server.Close()

		restoreConfig := setAlertConfig(email, server.URL, alertRoute)
		defer restoreConfig()

		alertService := service.NewAlert(&http.Client{Transport: &http.Transport{}})
		cond := "<= 123.45"
		alert := model.Alert{
			Ticker:     "AAPL",
			Condition:  &cond,
			AlertName:  "breakout",
			Exchange:   "NASDAQ",
			Conditions: buildAlertConditions("265598@NASDAQ", "<=", "123.45"),
		}

		response, err := alertService.Create(context.Background(), alert)

		assert.Equal(t, []model.IBKRAlert{}, response)
		assert.EqualError(t, err, `400 Bad Request: {"error":"forbidden"}`)
	})
}

func buildAlertConditions(conidex, operator, value string) []model.Conditions {
	return []model.Conditions{{
		Conidex:       conidex,
		LogicBind:     "n",
		Operator:      operator,
		TriggerMethod: "0",
		Type:          1,
		Value:         value,
	}}
}

func setAlertConfig(email, baseURL, alertPath string) func() {
	oldEmail := viper.GetString("EMAIL")
	oldBaseURL := viper.GetString("BASEURL")
	oldAlertPath := viper.GetString("ALERT")

	viper.Set("EMAIL", email)
	viper.Set("BASEURL", baseURL)
	viper.Set("ALERT", alertPath)

	return func() {
		viper.Set("EMAIL", oldEmail)
		viper.Set("BASEURL", oldBaseURL)
		viper.Set("ALERT", oldAlertPath)
	}
}
