package model

type AllOrders struct {
	Orders []OrderDetails `json:"orders"`
}

type OrderDetails struct {
	Prices             string  `json:"aaPrices"`
	Account            string  `json:"account"`
	Acct               string  `json:"acct"`
	BgColor            string  `json:"bgColor"`
	CashCcy            string  `json:"cashCcy"`
	CompanyName        string  `json:"companyName"`
	Conid              int     `json:"conid"`
	Conidex            string  `json:"conidex"`
	Description1       string  `json:"description1"`
	FgColor            string  `json:"fgColor"`
	FilledQuantity     float64 `json:"filledQuantity"`
	LastExecutionTime  string  `json:"lastExecutionTime"`
	LastExecutionTimeR int64   `json:"lastExecutionTime_r"`
	ListingExchange    string  `json:"listingExchange"`
	OrderDesc          string  `json:"orderDesc"`
	OrderId            int     `json:"orderId"`
	OrderType          string  `json:"orderType"`
	OrderCcpStatus     string  `json:"order_ccp_status"`
	OrderRef           string  `json:"order_ref"`
	OrigOrderType      string  `json:"origOrderType"`
	Price              string  `json:"price"`
	StopPrice          string  `json:"stop_price"`
	RemainingQuantity  float64 `json:"remainingQuantity"`
	SecType            string  `json:"secType"`
	Side               string  `json:"side"`
	SizeAndFills       string  `json:"sizeAndFills"`
	Status             string  `json:"status"`
	SupportsTaxOpt     string  `json:"supportsTaxOpt"`
	Ticker             string  `json:"ticker"`
	TimeInForce        string  `json:"timeInForce"`
	TotalSize          float64 `json:"totalSize"`
	TotalAmount        float64 `json:"totalAmount"`
	StopLossPercent    string  `json:"zStopLossPercent"`
	TargetPercent      string  `json:"zTargetPercent"`
	PotentialLoss      string  `json:"zPotentialLoss"`
	PotentialProfit    string  `json:"zPotentialProfit"`
}

type ListOrder struct {
	Ticker          string  `json:"ticker"`
	OrderDesc       string  `json:"orderDesc"`
	Quantity        float64 `json:"remainingQuantity"`
	Status          string  `json:"status"`
	Price           float64 `json:"price"`
	StopLossPercent string  `json:"stopLossPercent"`
	TargetPercent   string  `json:"targetPercent"`
	OrderType       string  `json:"orderType"`
	OrderId         int     `json:"orderId"`
	CashCcy         string  `json:"cashCcy"`
}

type UpdateOrder struct {
	Prices string `json:"aaPrices"`
	OrderDetails
}

type UpdateRequest struct {
	Orders []UpdateOrder `json:"orders"`
}

type UpdateContract struct {
	Ticker   string  `json:"ticker"`
	OrderID  int     `json:"orderID"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Target   float64 `json:"target"`
	StopLoss float64 `json:"stopLoss"`
}
