package model

type Checkup struct {
	Symbol          string      `json:"Symbol"`
	UpdatedDateTime interface{} `json:"UpdatedDateTime"`
	Price           string      `json:"Price"`
	PriceClose      string      `json:"PriceClose"`
	PriceChange     string      `json:"PriceChange"`
	PricePctChange  string      `json:"PricePctChange"`
	VolumePctChange string      `json:"VolumePctChange"`
	Type            int         `json:"Type"`
	High            string      `json:"High"`
	Low             string      `json:"Low"`
	Volume          string      `json:"Volume"`
	Descripion      string      `json:"Descripion"`
	Coname          string      `json:"Coname"`
	IndustryGroup   string      `json:"IndustryGroup"`
	ExchangeName    string      `json:"ExchangeName"`
	CheckListData   interface{} `json:"CheckListData"`
	Performance     interface{} `json:"Performance"`
	QuotesData      struct {
		StockData           []Field `json:"StockData"`
		CompanyFundamentals []Field `json:"CompanyFundamentals"`
	} `json:"QuotesData"`
	InLists       []interface{} `json:"InLists"`
	ChartAnalysis string        `json:"ChartAnalysis"`
	ExtendedHours struct {
		Price              string `json:"Price"`
		PriceChange        string `json:"PriceChange"`
		PricePctChange     string `json:"PricePctChange"`
		MarketStatusTypeId int    `json:"MarketStatusTypeId"`
	} `json:"ExtendedHours"`
}

type Field struct {
	Element         string      `json:"Element"`
	Value           string      `json:"Value"`
	Pass            interface{} `json:"Pass"`
	Criteria        interface{} `json:"Criteria"`
	PassingCriteria interface{} `json:"PassingCriteria"`
	CriteriaValue   interface{} `json:"CriteriaValue"`
}

type StockData struct {
	GetDataResult struct {
		AsOf        string `json:"asOf"`
		CompanyInfo struct {
			AvgVolume                          int     `json:"avgVolume"`
			AvgVolumeAsLabel                   string  `json:"avgVolumeAsLabel"`
			CoName                             string  `json:"coName"`
			DayRate                            float64 `json:"dayRate"`
			HasConvertableIssue                bool    `json:"hasConvertableIssue"`
			HasWarrant                         bool    `json:"hasWarrant"`
			IndustryGroup                      string  `json:"industryGroup"`
			IsValidETF                         bool    `json:"isValidETF"`
			IsValidIndex                       bool    `json:"isValidIndex"`
			MarketCapitalizationPrimary        float64 `json:"marketCapitalizationPrimary"`
			MarketCapitalizationPrimaryAsLabel string  `json:"marketCapitalizationPrimaryAsLabel"`
			MarketCapitalizationSecondary      int     `json:"-"`
			OffWeekHigh                        float64 `json:"offWeekHigh"`
			Price                              float64 `json:"price"`
			PriceChange                        float64 `json:"priceChange"`
			PrimaryExchange                    string  `json:"primaryExchange"`
			SharesInFloat                      int     `json:"sharesInFloat"`
			SharesOutstandingPrimary           int     `json:"sharesOutstandingPrimary"`
			SharesOutstandingPrimaryAsLabel    string  `json:"sharesOutstandingPrimaryAsLabel"`
			SharesOutstandingSecondary         int     `json:"sharesOutstandingSecondary"`
			ShortInterestPosition              float64 `json:"shortInterestPosition"`
			ShortInterestRatio                 int     `json:"shortInterestRatio"`
			Symbol                             string  `json:"symbol"`
			Type                               string  `json:"type"`
			Volume                             int     `json:"volume"`
			VolumeBase                         int     `json:"volumeBase"`
			WeeksHigh                          float64 `json:"weeksHigh"`
			WeeksLow                           float64 `json:"weeksLow"`
		} `json:"companyInfo"`
		CurrentMarketStatus       string `json:"currentMarketStatus"`
		CurrentMarketStatusTypeId int    `json:"currentMarketStatusTypeId"`
		EPSRating                 int    `json:"ePSRating"`
		EndDate                   string `json:"endDate"`
		ErrorMessage              string `json:"errorMessage"`
		FiscalMonthEnd            int    `json:"fiscalMonthEnd"`
		Holidays                  []struct {
			Date    string `json:"date"`
			Holiday string `json:"holiday"`
		} `json:"holidays"`
		MarketDate string `json:"marketDate"`
		Osid       int    `json:"osid"`
		RSRating   int    `json:"rSRating"`
		Series     []struct {
			Close         float64 `json:"close"`
			Date          string  `json:"date"`
			DateIndicator int     `json:"dateIndicator"`
			High          float64 `json:"high"`
			IndexClose    float64 `json:"indexClose"`
			Low           float64 `json:"low"`
			Volume        int     `json:"volume"`
		} `json:"series"`
		Source    string `json:"source"`
		StartDate string `json:"startDate"`
		Summary   struct {
			URL          string `json:"URL"`
			Headquarters string `json:"headquarters"`
			NewIssue     string `json:"newIssue"`
			Phone        string `json:"phone"`
			ShortStory   string `json:"shortStory"`
		} `json:"summary"`
		Symbol string `json:"symbol"`
		Type   int    `json:"type"`
	} `json:"GetDataResult"`
}
