package model

type StockSummary struct {
	Symbol      string `json:"symbol"`
	SummaryData struct {
		Exchange struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"Exchange"`
		Sector struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"Sector"`
		Industry struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"Industry"`
		OneYrTarget struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"OneYrTarget"`
		TodayHighLow struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"TodayHighLow"`
		ShareVolume struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"ShareVolume"`
		AverageVolume struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"AverageVolume"`
		PreviousClose struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"PreviousClose"`
		FiftTwoWeekHighLow struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"FiftTwoWeekHighLow"`
		MarketCap struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"MarketCap"`
		PERatio struct {
			Label string  `json:"label"`
			Value float64 `json:"value"`
		} `json:"PERatio"`
		ForwardPE1Yr struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"ForwardPE1Yr"`
		EarningsPerShare struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"EarningsPerShare"`
		AvgDailyVol20Days struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"AvgDailyVol20Days"`
		AvgDailyVol65Days struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"AvgDailyVol65Days"`
		AnnualizedDividend struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"AnnualizedDividend"`
		ExDividendDate struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"ExDividendDate"`
		DividendPaymentDate struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"DividendPaymentDate"`
		Yield struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"Yield"`
		Alpha struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"Alpha"`
		StandardDeviation struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"StandardDeviation"`
		AUM struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"AUM"`

		WeightedAlpha struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"WeightedAlpha"`
		Beta struct {
			Label string  `json:"label"`
			Value float64 `json:"value"`
		} `json:"Beta"`
	} `json:"summaryData"`
	AssetClass     string      `json:"assetClass"`
	AdditionalData interface{} `json:"additionalData"`
	BidAsk         struct {
		BidSize struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"Bid * Size"`
		AskSize struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"Ask * Size"`
	} `json:"bidAsk"`
}

type InsiderActivity struct {
	NumberOfSharesTraded struct {
		Rows []InsiderTrade `json:"rows"`
	} `json:"numberOfSharesTraded"`
}

type InsiderTrade struct {
	InsiderTrade string `json:"insiderTrade"`
	Months3      string `json:"months3"`
	Months12     string `json:"months12"`
}
type NewsRow struct {
	Title          string   `json:"title"`
	Image          string   `json:"image"`
	Created        string   `json:"created"`
	Ago            string   `json:"ago"`
	Primarysymbol  string   `json:"primarysymbol"`
	Primarytopic   string   `json:"primarytopic"`
	Publisher      string   `json:"publisher"`
	RelatedSymbols []string `json:"related_symbols"`
	Url            string   `json:"url"`
	Id             int      `json:"id"`
	Imagedomain    string   `json:"imagedomain"`
}
type Earnings struct {
	ReportText      string `json:"reportText"`
	Heading         string `json:"heading"`
	Announcement    string `json:"announcement"`
	EQRResposeModel struct {
		Text        []string    `json:"text"`
		BoldText    interface{} `json:"boldText"`
		DataLinkUrl struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"dataLinkUrl"`
	} `json:"eQRResposeModel"`
}
type EarningSuprises struct {
	FiscalQtrEnd       string  `json:"fiscalQtrEnd"`
	DateReported       string  `json:"dateReported"`
	Eps                float64 `json:"eps"`
	ConsensusForecast  string  `json:"consensusForecast"`
	PercentageSurprise string  `json:"percentageSurprise"`
}

type EarningsForecast struct {
	FiscalEnd            string  `json:"fiscalEnd"`
	ConsensusEPSForecast float64 `json:"consensusEPSForecast"`
	HighEPSForecast      float64 `json:"highEPSForecast"`
	LowEPSForecast       float64 `json:"lowEPSForecast"`
	NoOfEstimates        int     `json:"noOfEstimates"`
	Up                   int     `json:"up"`
	Down                 int     `json:"down"`
}

type Revenue struct {
	Value1 string `json:"value1"`
	Value2 string `json:"value2"`
	Value3 string `json:"value3"`
	Value4 string `json:"value4"`
}

type ActiveStock struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	Lastsale  string `json:"lastsale"`
	Netchange string `json:"netchange"`
	Pctchange string `json:"pctchange"`
	MarketCap string `json:"marketCap"`
	Url       string `json:"url"`
}

type Calendar struct {
	AsOf                string `json:"asOf"`
	LastYearRptDt       string `json:"lastYearRptDt"`
	LastYearEPS         string `json:"lastYearEPS"`
	Time                string `json:"time"`
	Symbol              string `json:"symbol"`
	Name                string `json:"name"`
	MarketCap           string `json:"marketCap"`
	FiscalQuarterEnding string `json:"fiscalQuarterEnding"`
	EpsForecast         string `json:"epsForecast"`
	NoOfEsts            string `json:"noOfEsts"`
}
