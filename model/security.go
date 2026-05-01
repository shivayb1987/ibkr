package model

type Security struct {
	Conid           int         `json:"conid"`
	Currency        string      `json:"currency"`
	Time            int         `json:"time"`
	ChineseName     string      `json:"chineseName"`
	AllExchanges    string      `json:"allExchanges"`
	ListingExchange string      `json:"listingExchange"`
	CountryCode     string      `json:"countryCode"`
	Name            string      `json:"name"`
	AssetClass      string      `json:"assetClass"`
	Expiry          interface{} `json:"expiry"`
	LastTradingDay  interface{} `json:"lastTradingDay"`
	Group           string      `json:"group"`
	PutOrCall       interface{} `json:"putOrCall"`
	Sector          string      `json:"sector"`
	SectorGroup     string      `json:"sectorGroup"`
	Strike          string      `json:"strike"`
	Ticker          string      `json:"ticker"`
	UndConid        int         `json:"undConid"`
	Multiplier      float64     `json:"multiplier"`
	Type            string      `json:"type"`
	HasOptions      bool        `json:"hasOptions"`
	FullName        string      `json:"fullName"`
	IsUS            bool        `json:"isUS"`
}
