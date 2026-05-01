package main

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"ibkr/handler"
	"ibkr/service"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	defaultTimeout        = 10
	defaultTimeZone       = "Asia/Singapore"
	dialTimeout           = 30 * time.Second
	keepAlive             = 30 * time.Second
	maxIdleConnections    = 100
	idleConnTimeout       = 24 * time.Hour
	expectContinueTimeout = 50 * time.Second
	clientTimeout         = 50 * time.Second
	maxRetry              = 3
	minRetryWait          = 10 * time.Millisecond
	maxRetryWait          = 100 * time.Millisecond
	spreadsheetID         = "1MlIYs0uha3BrzskcHfchbkLhpFIsNt006YFRl8U4b98"
	credentials           = "golang-api-key.json"
)

func main() {
	os.Setenv("GODEBUG", "http2client=0")
	viper.AutomaticEnv()
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
	}

	viper.SetConfigFile("../stocks/.env")
	if err := viper.MergeInConfig(); err != nil {

	}

	flags := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	file, err := os.OpenFile("log.txt", flags, 0666)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(file)

	//viper.WatchConfig()
	//viper.OnConfigChange(func(in fsnotify.Event) {
	//	fmt.Println("config file changed: ", in.Name)
	//})

	httpClient := initializeHTTPClient()
	ibkrAPIService := service.NewIBKRAPI(httpClient)
	alertService := service.NewAlert(httpClient)
	positionService := service.NewPosition()
	yahooService := service.NewYahoo()
	fileService := service.NewFileWriter()
	ibdService := service.NewIBD(httpClient)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(middleware.Heartbeat("/healthz"))
	r.Use(middleware.Timeout(time.Second * time.Duration(viper.GetInt("TIMEOUT_IN_SECONDS"))))

	yahooHandler := handler.NewYahoo(yahooService, viper.GetStringSlice("US"))
	nasdaqService := service.NewNasdaq(httpClient)
	nasdaqHandler := handler.NewNasdaq(nasdaqService)

	sheetsService := service.NewSheetsService(getSheetsClient())
	sheetsHandler := handler.NewSheetsHandler(sheetsService, fileService, ibkrAPIService, nasdaqService, yahooService, ibdService, spreadsheetID)

	polygonService := service.NewPolygon(httpClient, viper.GetString("POLYGON_API_KEY"))
	polygonHandler := handler.NewPolygon(polygonService)

	tradesService := service.NewTrades()
	timeService := getTimeService()
	tradersHandler := handler.NewIBKRTrades(tradesService, timeService)

	ibkrHandler := handler.NewIbkr(ibkrAPIService, alertService, positionService, fileService, yahooService, polygonService, nasdaqService, ibdService, viper.GetStringSlice("US"), viper.GetFloat64("TOTAL_EQUITY"))

	nyseService := service.NewNYSE()
	nyseHandler := handler.NewNyse(nyseService)
	r.Group(func(r chi.Router) {
		r.Post("/orders", ibkrHandler.Order)
		r.Post("/bracketOrders", ibkrHandler.BracketOrder)
		r.Post("/ocaOrders", ibkrHandler.OCA)
		r.Post("/sell", ibkrHandler.MarketSell2)
		r.Post("/close", ibkrHandler.MarketSell) // closePos
		r.Post("/buy", ibkrHandler.MarketBuy)
		r.Post("/preview", ibkrHandler.Preview)

		r.Route("/nyse", func(r chi.Router) {
			r.Get("/quotes", nyseHandler.GetQuotes)
		})

		r.Get("/contract", ibkrHandler.ContractID)
		r.Delete("/orders/{symbol}", ibkrHandler.Cancel)
		r.Put("/orders", ibkrHandler.Update)
		r.Put("/contracts", ibkrHandler.Contracts)
		r.Put("/merge", ibkrHandler.Merge)
		r.Get("/search", ibkrHandler.Search)
		r.Get("/securities", ibkrHandler.Security)

		// Alerts
		r.Post("/alert", ibkrHandler.Alert)
		r.Get("/alerts", ibkrHandler.Alerts)

		// Positions
		r.Get("/orders", ibkrHandler.GetAll)
		r.Get("/list", ibkrHandler.List)
		r.Get("/positions", ibkrHandler.Positions)
		r.Get("/pnl", ibkrHandler.PnL)
		r.Get("/summary", ibkrHandler.Summary)
		r.Post("/reply/{replyID}", ibkrHandler.Reply)
		r.Get("/similar/{ticker}", ibkrHandler.Similar)

		// Sheets
		r.Get("/sheets/summary", sheetsHandler.Summary)
		r.Get("/sheets/shortlist", sheetsHandler.Shortlist)
		r.Post("/sheets/shortlist", sheetsHandler.ShortlistInsert)
		r.Get("/sheets/trades", sheetsHandler.Trades)
		r.Get("/sheets/performance", sheetsHandler.Performance)
		r.Get("/sheets/analyse", sheetsHandler.Analyse)
		r.Get("/sheets/analyse2", sheetsHandler.Analyse2)
		r.Get("/sheets/analyse3", sheetsHandler.AnalyseEntry)
		r.Post("/sheets", sheetsHandler.Create)
		r.Get("/sheets/actionItems", sheetsHandler.ActiveItems)
		r.Get("/sheets/actionItem", sheetsHandler.GetActionItem)
		r.Post("/record", sheetsHandler.Record)
		r.Post("/recordAll", sheetsHandler.RecordAll)

		r.Route("/yahoo", func(r chi.Router) {
			r.Get("/active", yahooHandler.GetActiveStocks)
		})
		r.Get("/nasdaq", nasdaqHandler.Summary)
		r.Get("/calendar", nasdaqHandler.Calendar)
		r.Get("/earnings", nasdaqHandler.Earnings)
		r.Get("/insider", nasdaqHandler.InsiderActivity)
		r.Get("/news", nasdaqHandler.News)
		r.Get("/tickerInfo", polygonHandler.Summary)
		r.Get("/trades", tradersHandler.Get)
	})

	listenAddr := viper.GetString("LISTEN_ADDR")
	if err := http.ListenAndServe(listenAddr, r); err != nil {
		log.Printf("cannot start server, %s", err)
	}
}

func initializeHTTPClient() *http.Client {
	c := retryablehttp.NewClient()

	h := c.StandardClient()

	// nolint
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = maxIdleConnections
	t.IdleConnTimeout = idleConnTimeout
	t.ExpectContinueTimeout = expectContinueTimeout
	t.DialContext = (&net.Dialer{Timeout: dialTimeout, KeepAlive: keepAlive}).DialContext

	h.Timeout = clientTimeout
	h.Transport = t
	return h
}

func getSheetsClient() *sheets.Service {
	// Load the Google Sheets API credentials from your JSON file.
	creds, err := os.ReadFile(credentials)
	if err != nil {
		log.Fatalf("Unable to read credentials file: %v", err)
	}

	config, err := google.JWTConfigFromJSON(creds, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Unable to create JWT config: %v", err)
	}

	client := config.Client(context.Background())
	//t := http.DefaultTransport.(*http.Transport).Clone()
	//t.MaxIdleConns = maxIdleConnections
	//t.IdleConnTimeout = idleConnTimeout
	//t.ExpectContinueTimeout = expectContinueTimeout
	//t.DialContext = (&net.Dialer{Timeout: dialTimeout, KeepAlive: keepAlive}).DialContext

	//client.Timeout = clientTimeout
	//client.Transport = t
	sheetsService, err := sheets.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to create Google Sheets service: %v", err)
		os.Exit(1)
	}

	return sheetsService
}

func getTimeService() *service.Time {
	timeLocation, err := time.LoadLocation(service.TimezoneSingapore)
	if err != nil {
		os.Exit(1)
	}

	return service.NewTime(timeLocation)
}
