package model

type SheetData struct {
	Range   string     `json:"range"`
	Values  [][]string `json:"values"`
	Updates Update     `json:"updates"`
}

type Update struct {
	UpdatedRange   string `json:"updatedRange"`
	UpdatedRows    int    `json:"updatedRows"`
	UpdatedColumns int    `json:"updatedColumns"`
	UpdatedCells   int    `json:"updatedCells"`
}

type InsertRow struct {
	Symbol    string `json:"ticker"`
	Prices    string `json:"prices"`
	DataRange string `json:"dataRange"`
}

type RecordTrade struct {
	Symbol     string  `json:"ticker"`
	Bought     float64 `json:"bought"`
	Sold       float64 `json:"sold"`
	Quantity   int     `json:"quantity"`
	Sector     string  `json:"sector"`
	DataRange  string  `json:"dataRange"`
	OpenDate   string  `json:"open"`
	ClosedDate string  `json:"closed"`
}
type Shortlist struct {
	Symbol            string `json:"symbol"`
	CompositeRank     string `json:"compositeRank"`
	EpsGrowth         string `json:"epsGrowth"`
	SalesGrowth       string `json:"salesGrowth"`
	ROE               string `json:"roe"`
	GrossMargin       string `json:"grossMargin"`
	IndustryGroupRank string `json:"industryGroupRank"`
	PERatio           string `json:"peRatio"`
	EPSRanking        string `json:"epsRank"`
	GroupRSRating     string `json:"groupRSRating"`
	Position          string `json:"position"`
}
type ActionItem struct {
	Ticker          string  `json:"ticker"`
	Name            string  `json:"name"`
	Sector          string  `json:"sector"`
	Industry        string  `json:"industry"`
	MarketCap       string  `json:"marketCap"`
	Volume          string  `json:"volume"`
	Beta            float64 `json:"beta"`
	ATR             string  `json:"atr"`
	Revenues        string  `json:"revenues"`
	InsiderActivity float64 `json:"insiderActivity"`
	Surprises       int     `json:"surprises"`
	Upgrades        string  `json:"upgrades"`
	Downgrades      string  `json:"downgrades"`
	NetUpgrades     string  `json:"netUpgrades"`
	Yield           string  `json:"yield"`
}
