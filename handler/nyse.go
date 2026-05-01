package handler

import (
	"ibkr/model"
	"net/http"
	"strconv"
	"strings"
)

type NyseService interface {
	Quotes(payload model.QuotesPayload) ([]model.NYSE, error)
}

type NYSE struct {
	nyseService NyseService
}

func NewNyse(nyseService NyseService) NYSE {
	return NYSE{
		nyseService: nyseService,
	}
}

func (h NYSE) GetQuotes(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	var page int64
	if pageStr != "" {
		page, _ = strconv.ParseInt(pageStr, 10, 64)
	}
	if page <= 0 {
		page = 1
	}
	allQuotes := make([]model.NYSE, 0)
	payload := model.QuotesPayload{
		InstrumentType:    "EQUITY",
		PageNumber:        page,
		SortColumn:        "NORMALIZED_TICKER",
		SortOrder:         "ASC",
		MaxResultsPerPage: 500,
		FilterToken:       "",
	}
	quotes, _ := h.nyseService.Quotes(payload)
	allQuotes = append(allQuotes, quotes...)

	symbols := make([]string, 0)
	for _, quote := range allQuotes {
		symbols = append(symbols, quote.SymbolTicker)
	}

	Success(w, r, map[string]interface{}{
		"quotes": allQuotes,
		"list":   strings.Join(symbols, " "),
	})
}
