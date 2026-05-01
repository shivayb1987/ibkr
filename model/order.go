package model

const (
	StopLimit = "STP LMT"
	Limit     = "LMT"
	Market    = "MKT"
	Stop      = "STP"
)

type IBKROrder struct {
	Orders []Order `json:"orders"`
}

type Order struct {
	Prices      string  `json:"prices"`
	Position    float64 `json:"position,omitempty"`
	AcctID      string  `json:"acctId"`
	ConID       int     `json:"conid"`
	ConidEX     string  `json:"conidex"`
	OrderType   string  `json:"orderType"`
	Price       float64 `json:"price"`
	AuxPrice    float64 `json:"auxPrice"` // stop
	Side        string  `json:"side"`
	Ticker      string  `json:"ticker"`
	TIF         string  `json:"tif"`
	Quantity    int     `json:"quantity"`
	CoID        string  `json:"cOID,omitempty"`
	ParentID    string  `json:"parentId,omitempty"`
	OrderId     *int    `json:"orderId,omitempty"`
	Marketprice bool    `json:"marketprice"`
}

type OrderAck struct {
	Error          string   `json:"error"`
	OrderId        string   `json:"order_id"`
	OrderStatus    string   `json:"order_status"`
	EncryptMessage string   `json:"-"`
	Id             string   `json:"id"`
	Message        []string `json:"message"`
	IsSuppressed   bool     `json:"-"`
	MessageIds     []string `json:"-"`
	Text           string   `json:"text"`
	Total          float64  `json:"total"`
}

type OrderResponse []OrderAck

type Reply struct {
	Confirmed bool `json:"confirmed"`
}

type CancelOrder struct {
	Msg     string `json:"msg"`
	OrderId int    `json:"order_id"`
	Conid   int    `json:"conid"`
	Account string `json:"account"`
}

type MarketData struct {
	ServerId           string `json:"serverId"`
	Symbol             string `json:"symbol"`
	Text               string `json:"text"`
	PriceFactor        int    `json:"priceFactor"`
	StartTime          string `json:"startTime"`
	High               string `json:"high"`
	Low                string `json:"low"`
	TimePeriod         string `json:"timePeriod"`
	BarLength          int    `json:"barLength"`
	MdAvailability     string `json:"mdAvailability"`
	MktDataDelay       int    `json:"mktDataDelay"`
	OutsideRth         bool   `json:"outsideRth"`
	TradingDayDuration int    `json:"tradingDayDuration"`
	VolumeFactor       int    `json:"volumeFactor"`
	PriceDisplayRule   int    `json:"priceDisplayRule"`
	PriceDisplayValue  string `json:"priceDisplayValue"`
	ChartPanStartTime  string `json:"chartPanStartTime"`
	Direction          int    `json:"direction"`
	NegativeCapable    bool   `json:"negativeCapable"`
	MessageVersion     int    `json:"messageVersion"`
	Data               []struct {
		O float64 `json:"o"`
		C float64 `json:"c"`
		H float64 `json:"h"`
		L float64 `json:"l"`
		V float64 `json:"v"`
		T int64   `json:"t"`
	} `json:"data"`
	Points     int `json:"points"`
	TravelTime int `json:"travelTime"`
}
