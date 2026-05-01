package model

type PolygonTickerDetails struct {
	Ticker                      string  `json:"ticker"`
	Name                        string  `json:"name"`
	Market                      string  `json:"market"`
	Locale                      string  `json:"locale"`
	PrimaryExchange             string  `json:"primary_exchange"`
	Type                        string  `json:"type"`
	Active                      bool    `json:"active"`
	CurrencyName                string  `json:"currency_name"`
	Cik                         string  `json:"cik"`
	CompositeFigi               string  `json:"composite_figi"`
	ShareClassFigi              string  `json:"share_class_figi"`
	MarketCap                   float64 `json:"market_cap"`
	Description                 string  `json:"description"`
	SicCode                     string  `json:"sic_code"`
	SicDescription              string  `json:"sic_description"`
	TickerRoot                  string  `json:"ticker_root"`
	HomepageUrl                 string  `json:"homepage_url"`
	TotalEmployees              int     `json:"total_employees"`
	ListDate                    string  `json:"list_date"`
	ShareClassSharesOutstanding int64   `json:"share_class_shares_outstanding"`
	WeightedSharesOutstanding   int64   `json:"weighted_shares_outstanding"`
}
