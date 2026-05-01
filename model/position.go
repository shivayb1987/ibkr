package model

type Position struct {
	AcctId        string        `json:"acctId"`
	Conid         int           `json:"conid"`
	ContractDesc  string        `json:"contractDesc"`
	Position      float64       `json:"position"`
	MktPrice      float64       `json:"mktPrice"`
	MktValue      float64       `json:"mktValue"`
	Currency      string        `json:"currency"`
	AvgCost       float64       `json:"avgCost"`
	AvgPrice      float64       `json:"avgPrice"`
	RealizedPnl   float64       `json:"realizedPnl"`
	UnrealizedPnl float64       `json:"unrealizedPnl"`
	Exchs         interface{}   `json:"exchs"`
	Expiry        interface{}   `json:"expiry"`
	PutOrCall     interface{}   `json:"putOrCall"`
	Multiplier    interface{}   `json:"multiplier"`
	ExerciseStyle interface{}   `json:"exerciseStyle"`
	ConExchMap    []interface{} `json:"conExchMap"`
	AssetClass    string        `json:"assetClass"`
	UndConid      int           `json:"undConid"`
	Model         string        `json:"model"`
	ProfitPercent float64       `json:"zzProfitPercent"`
}
