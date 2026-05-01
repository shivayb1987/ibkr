package model

type NYSE struct {
	Total                int    `json:"total"`
	Url                  string `json:"url"`
	ExchangeId           string `json:"exchangeId"`
	InstrumentType       string `json:"instrumentType"`
	SymbolTicker         string `json:"symbolTicker"`
	SymbolExchangeTicker string `json:"symbolExchangeTicker"`
	NormalizedTicker     string `json:"normalizedTicker"`
	InstrumentName       string `json:"instrumentName"`
	MicCode              string `json:"micCode"`
}

type QuotesPayload struct {
	InstrumentType    string `json:"instrumentType"`
	PageNumber        int64  `json:"pageNumber"`
	SortColumn        string `json:"sortColumn"`
	SortOrder         string `json:"sortOrder"`
	MaxResultsPerPage int    `json:"maxResultsPerPage"`
	FilterToken       string `json:"filterToken"`
}
