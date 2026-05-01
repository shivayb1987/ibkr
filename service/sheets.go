package service

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/api/sheets/v4"
	"ibkr/model"
	"regexp"
	"strings"
)

type SheetsService struct {
	sheetsClient *sheets.Service
}

func NewSheetsService(sheetsService *sheets.Service) SheetsService {
	return SheetsService{sheetsClient: sheetsService}
}

func (s SheetsService) GetValues(ctx context.Context, spreadsheetID string, dataRange string) (model.SheetData, error) {
	resp, err := s.sheetsClient.Spreadsheets.Values.Get(spreadsheetID, dataRange).Context(context.Background()).Do()
	if err != nil {
		return model.SheetData{}, err
	}

	jsonResult, err := resp.MarshalJSON()
	if err != nil {
		return model.SheetData{}, err
	}

	var data model.SheetData

	err = json.Unmarshal(jsonResult, &data)
	if err != nil {
		return model.SheetData{}, err
	}

	return data, nil
}

func (s SheetsService) CreateWatchlist(req model.InsertRow, spreadsheetID string, dataRange string) (model.SheetData, error) {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(dataRange, -1)

	valueRange := sheets.ValueRange{}
	valueRange.Values = make([][]interface{}, 0, 50)
	valueRange.MajorDimension = "ROWS"
	valueRange.Values = append(valueRange.Values, []interface{}{
		req.Symbol,
		"",
		strings.ReplaceAll("=IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),A14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),A14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),A14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),A14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),A14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),A14), \"price\"),\n\"Unknown exchange\"))))))", "A14", fmt.Sprintf("\"%s\"", req.Symbol)),
		req.Prices,
		strings.ReplaceAll("=1-C126/INDEX(SPLIT(D126, \"/\"), 1, 1)", "126", fmt.Sprintf("%s", matches[0])),
		strings.ReplaceAll("=IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),126), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),126), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),126), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),126), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),126), \"marketcap\"),\n\"\")))))/POW(10, 9)", "126", fmt.Sprintf("A%s", matches[0])),
		"",
		strings.ReplaceAll("=(INDEX(SPLIT(D126, \"/\"), 1, 3)-INDEX(SPLIT(D126, \"/\"), 1, 1))/(INDEX(SPLIT(D126, \"/\"), 1, 1)-INDEX(SPLIT(D126, \"/\"), 1, 2))", "126", fmt.Sprintf("%s", matches[0])), // R/R
		strings.ReplaceAll("=INDEX(SPLIT($D126, \"/\"), 1, 3)/C126-1", "126", fmt.Sprintf("%s", matches[0])),                                                                                             // target1
		strings.ReplaceAll("=(INDEX(SPLIT($D126, \"/\"), 1, 4)/C126)-1", "126", fmt.Sprintf("%s", matches[0])),                                                                                           // target2
		strings.ReplaceAll("=INDEX(SPLIT($D126, \"/\"), 1, 1)-INDEX(SPLIT($D126, \"/\"), 1, 2)", "126", fmt.Sprintf("%s", matches[0])),                                                                   // loss/share
		strings.ReplaceAll("=$K$1*$L$1/$K126", "126", fmt.Sprintf("%s", matches[0])),                                                                                                                     // loss/share
		strings.ReplaceAll("=$K$1*$M$1/$K126", "126", fmt.Sprintf("%s", matches[0])),                                                                                                                     // 100bps risk
		strings.ReplaceAll("=$K$1*$N$1/$K126", "126", fmt.Sprintf("%s", matches[0])),                                                                                                                     // 50bps risk
		strings.ReplaceAll("=$K$1*O$1/$K126", "126", fmt.Sprintf("%s", matches[0])),                                                                                                                      // 25bps risk
		strings.ReplaceAll("=$K$1*P$1/$K126", "126", fmt.Sprintf("%s", matches[0])),                                                                                                                      // 15bps risk
	})
	result, err := s.sheetsClient.Spreadsheets.Values.Append(spreadsheetID, dataRange, &valueRange).
		Context(context.Background()).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").Do()

	if err != nil {
		return model.SheetData{}, err
	}

	jsonResult, err := result.MarshalJSON()
	if err != nil {
		return model.SheetData{}, err
	}

	var data model.SheetData
	err = json.Unmarshal(jsonResult, &data)
	if err != nil {
		return model.SheetData{}, err
	}

	return data, nil
}

func (s SheetsService) RecordTrade(req model.RecordTrade, spreadsheetID string, dataRange string) (model.SheetData, error) {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(dataRange, -1)

	valueRange := sheets.ValueRange{}
	valueRange.Values = make([][]interface{}, 0, 50)
	valueRange.MajorDimension = "ROWS"

	sold := strings.ReplaceAll("=IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),I14), \"price\"),\nGOOGLEFINANCE(I14)))))))", "14", fmt.Sprintf("%s", matches[0]))

	if req.Sold > 0.0 {
		sold = fmt.Sprintf("%.2f", req.Sold)
	}
	valueRange.Values = append(valueRange.Values, []interface{}{
		req.Symbol,
		req.Bought,
		sold,
		1, // currency
		req.Quantity,
		req.Sector,
		strings.ReplaceAll("=IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),I14), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),I14), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),I14), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),I14), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),I14), \"marketcap\"),\n\"\")))))/POW(10, 9)", "14", fmt.Sprintf("%s", matches[0])),
		strings.ReplaceAll("=J182 * M182*L182", "182", fmt.Sprintf("%s", matches[0])), //Amount
		strings.ReplaceAll("=K182/J182-1", "182", fmt.Sprintf("%s", matches[0])),      // p/l amount
		strings.ReplaceAll("=P182*Q182", "182", fmt.Sprintf("%s", matches[0])),        // p/l amount
		strings.ReplaceAll("=2000 * Q182", "182", fmt.Sprintf("%s", matches[0])),      // equal allocation
		strings.ReplaceAll("=IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),I14), \"change\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),I14), \"change\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),I14), \"change\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),I14), \"change\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),I14), \"change\"),\n\"\")))))/K14", "14", fmt.Sprintf("%s", matches[0])),
		"",
		req.OpenDate,
		req.ClosedDate,
		"",
		"",
		"",
		strings.ReplaceAll("=IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),I14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),I14), \"price\"),\n\"Unknown exchange\"))))))", "14", fmt.Sprintf("%s", matches[0])),
		strings.ReplaceAll("=(X182-K182)/K182", "182", fmt.Sprintf("%s", matches[0])), // if held
		strings.ReplaceAll("=-M182*K182*Y182", "182", fmt.Sprintf("%s", matches[0])),  // target2
		strings.ReplaceAll("=X181/ INDEX(IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),$I181), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),$I181), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),$I181), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),$I181), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),$I181), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),$I181), \"price\", TODAY()-8),\n\"Unknown exchange\")))))), 2, 2)-1", "181", fmt.Sprintf("%s", matches[0])),
		strings.ReplaceAll("=IF(T:T=\"\",(X206-INDEX(IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),$I206), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),$I206), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),$I206), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),$I206), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),$I206), \"price\", TODAY()-8),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),$I206), \"price\", TODAY()-8),\n\"Unknown exchange\")))))), 2, 2)) * M206,0)", "206", fmt.Sprintf("%s", matches[0])), // target2
		strings.ReplaceAll("=IF(T:T=\"\",R181, \"\")", "181", fmt.Sprintf("%s", matches[0])),                // target2
		strings.ReplaceAll("=IFERROR(DATEDIF(V182,W182,\"D\"),\"\")", "182", fmt.Sprintf("%s", matches[0])), // target2
	})
	result, err := s.sheetsClient.Spreadsheets.Values.Append(spreadsheetID, dataRange, &valueRange).
		Context(context.Background()).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").Do()

	if err != nil {
		return model.SheetData{}, err
	}

	jsonResult, err := result.MarshalJSON()
	if err != nil {
		return model.SheetData{}, err
	}

	var data model.SheetData
	err = json.Unmarshal(jsonResult, &data)
	if err != nil {
		return model.SheetData{}, err
	}

	return data, nil
}
func (s SheetsService) ShortlistRecord(req model.Shortlist, spreadsheetID string, dataRange string) (model.SheetData, error) {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(dataRange, -1)

	valueRange := sheets.ValueRange{}
	valueRange.Values = make([][]interface{}, 0, 50)
	valueRange.MajorDimension = "ROWS"

	price := strings.ReplaceAll("=IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),A14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),A14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),A14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),A14), \"price\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),A14), \"price\"),\n\"\")))))", "14", fmt.Sprintf("%s", matches[0]))
	avgVol := strings.ReplaceAll("=GOOGLEFINANCE($A14, \"volumeavg\")/POW(10,6)*B14", "14", fmt.Sprintf("%s", matches[0]))
	marketcap := strings.ReplaceAll("=IFERROR(GOOGLEFINANCE(concat(concat(\"NYSE\",\":\"),A14), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NASDAQ\",\":\"),A14), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEAMERICAN\",\":\"),A14), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"NYSEARCA\",\":\"),A14), \"marketcap\"),\nIFERROR(GOOGLEFINANCE(concat(concat(\"TSE\",\":\"),A14), \"marketcap\"),\n\"\")))))/POW(10, 9)", "14", fmt.Sprintf("%s", matches[0]))
	netScore := strings.ReplaceAll("=IF(B146>=20,1,0)+IF(C146>=500,2,IF(C146>=100,1,IF(C146>=20,1,0)))+IF(D146<=100,1,0)+IF(E146>0 & E146<25,1,0)+IF(F146>=97,3,IF(F146>=95,2,IF(F146>=90,1,0)))+IF(G146>=20%,2,0)+IF(H146>=20%,2,0)+IF(I146>=20%,1,0)+IF(J146>=20%,1,0)+IF(K146>90,1,0)+IF(L146>90,1,0)", "146", fmt.Sprintf("%s", matches[0]))

	valueRange.Values = append(valueRange.Values, []interface{}{
		req.Symbol,
		price,
		avgVol,
		marketcap,
		req.PERatio,
		req.CompositeRank,
		req.EpsGrowth,
		req.SalesGrowth,
		req.ROE,
		req.GrossMargin,
		req.EPSRanking,
		req.GroupRSRating,
		netScore,
		0.25,
	})
	result, err := s.sheetsClient.Spreadsheets.Values.Append(spreadsheetID, dataRange, &valueRange).
		Context(context.Background()).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").Do()

	if err != nil {
		return model.SheetData{}, err
	}

	jsonResult, err := result.MarshalJSON()
	if err != nil {
		return model.SheetData{}, err
	}

	var data model.SheetData
	err = json.Unmarshal(jsonResult, &data)
	if err != nil {
		return model.SheetData{}, err
	}

	return data, nil
}
func (s SheetsService) RecordActionItems(req model.ActionItem, spreadsheetID string, dataRange string) (model.SheetData, error) {
	valueRange := sheets.ValueRange{}
	valueRange.Values = make([][]interface{}, 0, 50)
	valueRange.MajorDimension = "ROWS"

	marketcap := req.MarketCap
	if req.MarketCap == "" {
		marketcap = fmt.Sprintf("=GOOGLEFINANCE(\"%s\", \"marketcap\")", req.Ticker)
	}
	averageVolume := req.Volume
	if req.MarketCap == "" {
		averageVolume = fmt.Sprintf("=GOOGLEFINANCE(\"%s\", \"volumeavg\")", req.Ticker)
	}

	yield := req.Yield
	if yield == "N/A" {
		yield = ""
	}

	valueRange.Values = append(valueRange.Values, []interface{}{
		req.Ticker,
		req.Sector,
		req.Industry,
		marketcap,
		averageVolume,
		fmt.Sprintf("=GOOGLEFINANCE(\"%s\", \"beta\")", req.Ticker),
		req.ATR,
		req.Revenues,
		req.InsiderActivity,
		yield,
		req.Surprises,
		req.Upgrades,
		req.Downgrades,
		req.NetUpgrades,
	})
	result, err := s.sheetsClient.Spreadsheets.Values.Append(spreadsheetID, dataRange, &valueRange).
		Context(context.Background()).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").Do()

	if err != nil {
		return model.SheetData{}, err
	}

	jsonResult, err := result.MarshalJSON()
	if err != nil {
		return model.SheetData{}, err
	}

	var data model.SheetData
	err = json.Unmarshal(jsonResult, &data)
	if err != nil {
		return model.SheetData{}, err
	}

	return data, nil
}

func (s SheetsService) ClearSheet(spreadsheetID string, dataRange string) (string, error) {
	res, err := s.sheetsClient.Spreadsheets.Values.Clear(spreadsheetID, dataRange, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return "", err
	}
	return res.ClearedRange, err
}
