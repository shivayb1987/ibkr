package model

type Trade struct {
	ExecutionId           string  `json:"execution_id"`
	Symbol                string  `json:"symbol"`
	SupportsTaxOpt        string  `json:"supports_tax_opt"`
	Side                  string  `json:"side"`
	OrderDescription      string  `json:"order_description"`
	TradeTime             string  `json:"trade_time"`
	TradeTimeR            int64   `json:"trade_time_r"`
	Size                  float64 `json:"size"`
	Price                 string  `json:"price"`
	OrderRef              string  `json:"order_ref"`
	Submitter             string  `json:"submitter"`
	Exchange              string  `json:"exchange"`
	Commission            string  `json:"commission"`
	NetAmount             float64 `json:"net_amount"`
	Account               string  `json:"account"`
	AccountCode           string  `json:"accountCode"`
	AccountAllocationName string  `json:"account_allocation_name"`
	CompanyName           string  `json:"company_name"`
	ContractDescription1  string  `json:"contract_description_1"`
	SecType               string  `json:"sec_type"`
	ListingExchange       string  `json:"listing_exchange"`
	Conid                 int     `json:"conid"`
	ConidEx               string  `json:"conidEx"`
	ClearingId            string  `json:"clearing_id"`
	ClearingName          string  `json:"clearing_name"`
	LiquidationTrade      string  `json:"liquidation_trade"`
	IsEventTrading        string  `json:"is_event_trading"`
}
