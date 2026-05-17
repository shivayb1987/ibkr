package model

type Alert struct {
	Ticker          string       `json:"ticker"`
	Condition       *string      `json:"condition,omitempty"`
	AlertMessage    string       `json:"alertMessage"`
	AlertName       string       `json:"alertName"`
	ExpireTime      string       `json:"expireTime"`
	AlertRepeatable int          `json:"alertRepeatable"`
	OutsideRth      int          `json:"outsideRth"`
	SendMessage     int          `json:"sendMessage"`
	Email           string       `json:"email"`
	ITWSOrdersOnly  int          `json:"iTWSOrdersOnly"`
	ShowPopup       int          `json:"showPopup"`
	TIF             string       `json:"tif"`
	Conditions      []Conditions `json:"conditions"`
	Exchange        string       `json:"exchange"`
}

type Conditions struct {
	Conidex       string `json:"conidex"`
	LogicBind     string `json:"logicBind"`
	Operator      string `json:"operator"`
	TriggerMethod string `json:"triggerMethod"`
	Type          int    `json:"type"`
	Value         string `json:"value"`
}
type IBKRAlert struct {
	OrderId         int64  `json:"order_id"`
	Account         string `json:"account"`
	AlertName       string `json:"alert_name"`
	AlertActive     int    `json:"alert_active"`
	OrderTime       string `json:"order_time"`
	AlertTriggered  bool   `json:"alert_triggered"`
	AlertRepeatable int    `json:"alert_repeatable"`
}
