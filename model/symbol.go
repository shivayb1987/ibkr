package model

type Symbol struct {
	ConID         string         `json:"conid"`
	CompanyHeader string         `json:"companyHeader"`
	CompanyName   string         `json:"companyName"`
	Symbol        string         `json:"symbol"`
	Description   string         `json:"description"`
	Restricted    interface{}    `json:"restricted"`
	Fop           interface{}    `json:"fop"`
	Opt           interface{}    `json:"opt"`
	War           interface{}    `json:"war"`
	Sections      []SecurityType `json:"sections"`
	SecType       string         `json:"secType"`
}

type SecurityType struct {
	SecType string `json:"secType"`
}
