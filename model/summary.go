package model

type Summary struct {
	GrossPositionValue  GrossPositionValue  `json:"grosspositionvalue"`
	EquityWithLoanValue Equitywithloanvalue `json:"equitywithloanvalue"`
	TotalCashValue      TotalCashValue      `json:"totalcashvalue"`
	SMA                 SMA                 `json:"sma"`
	InitialMargin       InitialMargin       `json:"initmarginreq"`
	MaintenanceMargin   MaintenanceMargin   `json:"maintmarginreq"`
	AccruedDividend     AccruedDividend     `json:"accrueddividend"`
	AccruedCash         AccruedCash         `json:"accruedcash"`
	AvailableFunds      Availablefunds      `json:"availablefunds"`
}

type AccruedDividend struct {
	Amount    float64     `json:"amount"`
	Currency  string      `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Severity  int         `json:"severity"`
}

type AccruedCash struct {
	Amount    float64     `json:"amount"`
	Currency  string      `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Severity  int         `json:"severity"`
}

type InitialMargin struct {
	Amount    float64     `json:"amount"`
	Currency  string      `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Severity  int         `json:"severity"`
}

type Account struct {
	Amount    float64     `json:"amount"`
	Currency  interface{} `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     string      `json:"value"`
	Severity  int         `json:"severity"`
}

type Availablefunds struct {
	Amount    float64     `json:"amount"`
	Currency  string      `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Severity  int         `json:"severity"`
}

type Equitywithloanvalue struct {
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	IsNull    bool    `json:"isNull"`
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
	Severity  int     `json:"severity"`
}

type ExcessLiquidity struct {
	Amount    float64     `json:"amount"`
	Currency  string      `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Severity  int         `json:"severity"`
}

type GrossPositionValue struct {
	Amount    float64     `json:"amount"`
	Currency  string      `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Severity  int         `json:"severity"`
}

type TotalCashValue struct {
	Amount    float64     `json:"amount"`
	Currency  string      `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Severity  int         `json:"severity"`
}

type SMA struct {
	Amount    float64     `json:"amount"`
	Currency  string      `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Severity  int         `json:"severity"`
}

type MaintenanceMargin struct {
	Amount    float64     `json:"amount"`
	Currency  string      `json:"currency"`
	IsNull    bool        `json:"isNull"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Severity  int         `json:"severity"`
}
