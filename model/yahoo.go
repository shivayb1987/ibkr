package model

type SimilarStockResponse struct {
	Finance struct {
		Results []Result    `json:"result"`
		Error   interface{} `json:"error"`
	} `json:"finance"`
}

type Result struct {
	Symbol             string              `json:"symbol"`
	RecommendedSymbols []RecommendedSymbol `json:"recommendedSymbols"`
}

type RecommendedSymbol struct {
	Symbol string  `json:"symbol"`
	Score  float64 `json:"score"`
}

type YahooResponse struct {
	Chart Chart `json:"chart"`
}

type Chart struct {
	Result []Result2 `json:"result"`
}

type Result2 struct {
	Timestamps []int      `json:"timestamp"`
	Meta       Meta       `json:"meta"`
	Indicators Indicators `json:"indicators"`
}

type Meta struct {
	Symbol             string  `json:"symbol"`
	RegularMarketPrice float64 `json:"regularMarketPrice"`
	RegularMarketTime  int     `json:"regularMarketTime"`
	FiftyTwoWeekHigh   float64 `json:"fiftyTwoWeekHigh"`
	FiftyTwoWeekLow    float64 `json:"fiftyTwoWeekLow"`
	FullExchangeName   string  `json:"fullExchangeName"`
}

type Indicators struct {
	Quote []Quote `json:"quote"`
}

type Quote struct {
	Volume []int64   `json:"volume"`
	Close  []float64 `json:"close"`
	Open   []float64 `json:"open"`
	High   []float64 `json:"high"`
	Low    []float64 `json:"low"`
}

type Performance struct {
	Symbol              string  `json:"symbol"`
	FiftyWeekHigh       *bool   `json:"fiftyWeekHigh,omitempty"`
	Above200DMA         *int    `json:"above200DMA,omitempty"`
	Above50DMA          *int    `json:"above50DMA,omitempty"`
	Rising200DMA        *int    `json:"rising200DMA,omitempty"`
	Rising50DMA         *int    `json:"rising50DMA,omitempty"`
	RelativePerformance string  `json:"relativePerformance"`
	Volatility          float64 `json:"volatility"`
	Returns             float64 `json:"relativeReturns%"`
	fiftyTwoWeekHigh    *bool   `json:"fiftyTwoWeekHigh,omitempty"`

	Sector             string  `json:"sector"`
	Industry           string  `json:"industry"`
	EarningsPerShare   string  `json:"earningsPerShare"`
	Yield              string  `json:"yield"`
	PERatio            float64 `json:"PERatio"`
	ForwardPE1Yr       string  `json:"forwardPE1Yr"`
	MarketCapB         float64 `json:"marketCap($B)"`
	OneYrTarget        string  `json:"oneYrTarget"`
	InsiderNetActivity string  `json:"insiderNetActivity"`
}

type ActiveStocksResponse struct {
	Finance struct {
		Result []struct {
			Id            string        `json:"id"`
			Title         string        `json:"title"`
			Description   string        `json:"description"`
			CanonicalName string        `json:"canonicalName"`
			RawCriteria   string        `json:"rawCriteria"`
			Start         int           `json:"start"`
			Count         int           `json:"count"`
			Total         int           `json:"total"`
			Quotes        []ActiveQuote `json:"quotes"`
			UseRecords    bool          `json:"useRecords"`
			PredefinedScr bool          `json:"predefinedScr"`
			VersionId     int           `json:"versionId"`
			CreationDate  int64         `json:"creationDate"`
			LastUpdated   int64         `json:"lastUpdated"`
			IsPremium     bool          `json:"isPremium"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"finance"`
}

type ActiveQuote struct {
	Symbol                            string `json:"symbol"`
	TwoHundredDayAverageChangePercent struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"twoHundredDayAverageChangePercent"`
	FiftyTwoWeekLowChangePercent struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyTwoWeekLowChangePercent"`
	AverageAnalystRating  string `json:"averageAnalystRating"`
	Language              string `json:"language"`
	RegularMarketDayRange struct {
		Raw string `json:"raw"`
		Fmt string `json:"fmt"`
	} `json:"regularMarketDayRange"`
	EarningsTimestampEnd struct {
		Raw     int    `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"earningsTimestampEnd"`
	EpsForward struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"epsForward"`
	RegularMarketDayHigh struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"regularMarketDayHigh"`
	TwoHundredDayAverageChange struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"twoHundredDayAverageChange"`
	AskSize struct {
		Raw     int    `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"askSize"`
	TwoHundredDayAverage struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"twoHundredDayAverage"`
	BookValue struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"bookValue"`
	MarketCap struct {
		Raw     int64  `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"marketCap"`
	FiftyTwoWeekHighChange struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyTwoWeekHighChange"`
	IpoExpectedDate   string `json:"ipoExpectedDate,omitempty"`
	FiftyTwoWeekRange struct {
		Raw string `json:"raw"`
		Fmt string `json:"fmt"`
	} `json:"fiftyTwoWeekRange"`
	FiftyDayAverageChange struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyDayAverageChange"`
	ExchangeDataDelayedBy    int `json:"exchangeDataDelayedBy"`
	AverageDailyVolume3Month struct {
		Raw     int    `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"averageDailyVolume3Month"`
	FirstTradeDateMilliseconds int64 `json:"firstTradeDateMilliseconds"`
	TrailingAnnualDividendRate struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"trailingAnnualDividendRate"`
	FiftyTwoWeekChangePercent struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyTwoWeekChangePercent"`
	HasPrePostMarketData bool `json:"hasPrePostMarketData"`
	FiftyTwoWeekLow      struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyTwoWeekLow"`
	Market              string `json:"market"`
	RegularMarketVolume struct {
		Raw     int    `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"regularMarketVolume"`
	PostMarketPrice struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"postMarketPrice"`
	QuoteSourceName     string `json:"quoteSourceName"`
	MessageBoardId      string `json:"messageBoardId"`
	PriceHint           int    `json:"priceHint"`
	SourceInterval      int    `json:"sourceInterval"`
	Exchange            string `json:"exchange"`
	RegularMarketDayLow struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"regularMarketDayLow"`
	Region                       string `json:"region"`
	ShortName                    string `json:"shortName"`
	FiftyDayAverageChangePercent struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyDayAverageChangePercent"`
	FullExchangeName       string `json:"fullExchangeName"`
	EarningsTimestampStart struct {
		Raw     int    `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"earningsTimestampStart"`
	FinancialCurrency     string `json:"financialCurrency"`
	DisplayName           string `json:"displayName,omitempty"`
	GmtOffSetMilliseconds int    `json:"gmtOffSetMilliseconds"`
	RegularMarketOpen     struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"regularMarketOpen"`
	RegularMarketTime struct {
		Raw int    `json:"raw"`
		Fmt string `json:"fmt"`
	} `json:"regularMarketTime"`
	RegularMarketChangePercent struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"regularMarketChangePercent"`
	TrailingAnnualDividendYield struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"trailingAnnualDividendYield"`
	QuoteType               string `json:"quoteType"`
	AverageDailyVolume10Day struct {
		Raw     int    `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"averageDailyVolume10Day"`
	FiftyTwoWeekLowChange struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyTwoWeekLowChange"`
	FiftyTwoWeekHighChangePercent struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyTwoWeekHighChangePercent"`
	TypeDisp                     string `json:"typeDisp"`
	LastClosePriceToNNWCPerShare struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"lastClosePriceToNNWCPerShare"`
	Tradeable      bool `json:"tradeable"`
	PostMarketTime struct {
		Raw int    `json:"raw"`
		Fmt string `json:"fmt"`
	} `json:"postMarketTime"`
	Currency          string `json:"currency"`
	SharesOutstanding struct {
		Raw     int64  `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"sharesOutstanding"`
	RegularMarketPreviousClose struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"regularMarketPreviousClose"`
	FiftyTwoWeekHigh struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyTwoWeekHigh"`
	ExchangeTimezoneName    string `json:"exchangeTimezoneName"`
	PostMarketChangePercent struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"postMarketChangePercent"`
	RegularMarketChange struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"regularMarketChange"`
	BidSize struct {
		Raw     int    `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"bidSize"`
	PriceEpsCurrentYear struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"priceEpsCurrentYear"`
	CryptoTradeable bool `json:"cryptoTradeable"`
	FiftyDayAverage struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"fiftyDayAverage"`
	EpsCurrentYear struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"epsCurrentYear"`
	ExchangeTimezoneShortName  string `json:"exchangeTimezoneShortName"`
	MarketState                string `json:"marketState"`
	CustomPriceAlertConfidence string `json:"customPriceAlertConfidence"`
	RegularMarketPrice         struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"regularMarketPrice"`
	PostMarketChange struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"postMarketChange"`
	ForwardPE struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"forwardPE"`
	LastCloseTevEbitLtm struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"lastCloseTevEbitLtm,omitempty"`
	EarningsTimestamp struct {
		Raw     int    `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"earningsTimestamp,omitempty"`
	Ask struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"ask"`
	EpsTrailingTwelveMonths struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"epsTrailingTwelveMonths"`
	Bid struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"bid"`
	PriceToBook struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"priceToBook"`
	Triggerable  bool   `json:"triggerable"`
	LongName     string `json:"longName"`
	DividendDate struct {
		Raw     int    `json:"raw"`
		Fmt     string `json:"fmt"`
		LongFmt string `json:"longFmt"`
	} `json:"dividendDate,omitempty"`
	DividendYield struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"dividendYield,omitempty"`
	DividendRate struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"dividendRate,omitempty"`
	TrailingPE struct {
		Raw float64 `json:"raw"`
		Fmt string  `json:"fmt"`
	} `json:"trailingPE,omitempty"`
	PrevName       string `json:"prevName,omitempty"`
	NameChangeDate string `json:"nameChangeDate,omitempty"`
}
