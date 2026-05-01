package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"ibkr/model"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
)

type Position struct {
}

func NewPosition() Position {
	return Position{}
}

func (s Position) GetAll(ctx context.Context) ([]model.Position, []string, float64, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	url := fmt.Sprintf("%s%s", viper.GetString("BASEURL"), viper.GetString("POSITION"))
	response, err := http.Get(url)
	if err != nil {
		return nil, nil, 0, err
	}
	if response.StatusCode > http.StatusOK {
		fmt.Errorf("cannot get order: %s", response.Status)

		return []model.Position{}, nil, 0, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("cannot get orders :%w", err)
		return []model.Position{}, nil, 0, err
	}
	response.Body.Close()

	var positions []model.Position
	parseErr := json.Unmarshal(bodyBytes, &positions)
	if parseErr != nil {
		fmt.Errorf("symbol error: %w", parseErr)
		return []model.Position{}, nil, 0, parseErr
	}

	symbols := make([]string, 0, len(positions))
	totalCost := 0.0
	extendedPos := make([]string, 0)
	for index, pos := range positions {
		if pos.AvgCost > 0 {
			positions[index].ProfitPercent = roundFloat((pos.MktPrice / pos.AvgCost) - 1)
		}
		if math.Abs(pos.Position) > 0 {
			if pos.ProfitPercent >= 0.1 {
				extendedPos = append(extendedPos, pos.ContractDesc)
			} else {
				symbols = append(symbols, pos.ContractDesc)
			}
		}

		totalCost += pos.AvgCost * pos.Position
	}

	extendedPos = append(extendedPos, symbols...)

	sort.Slice(positions, func(i, j int) bool {
		return positions[i].ContractDesc < positions[j].ContractDesc
	})

	return positions, extendedPos, totalCost, nil
}

func (s Position) Summary(ctx context.Context) (model.Summary, error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	endpoint := fmt.Sprintf("%s%s", viper.GetString("BASEURL"), viper.GetString("SUMMARY"))
	response, err := http.Get(endpoint)
	if err != nil {
		return model.Summary{}, err
	}
	if response.StatusCode > http.StatusOK {
		fmt.Errorf("cannot get summary: %s", response.Status)

		return model.Summary{}, errors.New(response.Status)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("cannot get summary :%w", err)
		return model.Summary{}, err
	}
	response.Body.Close()

	var data model.Summary
	parseErr := json.Unmarshal(bodyBytes, &data)
	if parseErr != nil {
		fmt.Errorf("cannot unmarshall: %w", parseErr)
		return model.Summary{}, err
	}

	return data, nil
}

func roundFloat(value float64) float64 {
	strValue := fmt.Sprintf("%.2f", value)
	floatValue, _ := strconv.ParseFloat(strValue, 64)

	return math.Round(floatValue*100) / 100
}
