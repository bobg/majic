package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bobg/oauther/v3"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const namedCardAPIEndpoint = "https://api.scryfall.com/cards/named"

// The function "main" in the package "main"
// is where a Go program begins execution.
//
// This main function does nothing except call a run function.
// This is only so run can return any errors it encounters,
// like a normal Go function.
// By contrast,
// main has to do something else with errors,
// like print them
// (which is what it does with anything that run returns).
func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Parse the command-line flags.
	var (
		authcode  string // Auth code if needed to obtain an OAuth token.
		credsFile string // The file containing Google auth credentials for this application.
		sheetKey  string // The "key" of the spreadsheet - in a "docs.google.com/spreadsheets/d/KEY/edit" URL, it's the "KEY" part.
		sheetName string // The name of the sheet to operate on within the spreadsheet.
		tokenFile string // The file in which to store an OAuth token.
	)
	flag.StringVar(&authcode, "authcode", "", "auth code if needed to obtain an OAuth token")
	flag.StringVar(&credsFile, "creds", "creds.json", "path of JSON credentials file")
	flag.StringVar(&sheetKey, "sheetkey", "10ie9Wze3Byo_YqayMxNWnEWhlsn1ir2C10gO-fjsaUE", "spreadsheet key")
	flag.StringVar(&sheetName, "sheetname", "", "sheet name")
	flag.StringVar(&tokenFile, "token", "token.json", "path of OAuth token file")
	flag.Parse()

	creds, err := os.ReadFile(credsFile)
	if err != nil {
		return errors.Wrapf(err, "reading credentials from %s", credsFile)
	}

	// We need two rate-limiters.
	// One limits calls to the scryfall API to no more than ten per second
	// (as requested in the "Good Citizenship" section at
	// https://scryfall.com/docs/api).
	// The other limits calls to the Google spreadsheets API to no more than one per second.
	var (
		cardAPILimiter = rate.NewLimiter(10, 1)
		ssAPILimiter   = rate.NewLimiter(1, 1)
	)

	// This is the HTTP client to use for scryfall API calls.
	// It contains the limiter above.
	cardAPIClient := &http.Client{
		Transport: rateLimitedRoundTripper{
			limiter: cardAPILimiter,
		},
	}

	ctx := context.Background()

	// Creating the spreadsheet-API client is trickier.
	// We first need to get an OAuth-authenticated HTTP client.
	ssAPIClient, err := oauther.Client(ctx, tokenFile, authcode, creds, sheets.SpreadsheetsScope)
	if err != nil {
		return errors.Wrap(err, "authenticating")
	}

	// Now that we have an OAuth-authenticated HTTP client,
	// we can wrap its existing Transport field in a rateLimitedRoundTripper.
	origTransport := ssAPIClient.Transport
	if origTransport == nil {
		origTransport = http.DefaultTransport
	}
	ssAPIClient.Transport = rateLimitedRoundTripper{
		limiter: ssAPILimiter,
		next:    origTransport,
	}

	// Now that we have an OAuth-authenticated HTTP client that is also rate-limited,
	// we can use it to get a "sheets service" object.
	s, err := sheets.NewService(ctx, option.WithHTTPClient(ssAPIClient))
	if err != nil {
		return errors.Wrap(err, "creating sheets service")
	}

	// Now we can use that to request the full contents of the desired sheet.
	resp, err := s.Spreadsheets.Values.Get(sheetKey, sheetName+"!A-Z").Do()
	if err != nil {
		return errors.Wrap(err, "reading spreadsheet data")
	}
	if len(resp.Values) == 0 {
		return fmt.Errorf("zero rows in spreadsheet")
	}

	// We require row 0 to contain column headings.
	// Let's read those column headings and map them to column numbers;
	// e.g. "card name" -> 0, "set code" -> 1, etc.
	columnHeadings := make(map[string]int) // maps lowercase heading to column number
	for i, raw := range resp.Values[0] {
		if heading, ok := raw.(string); ok {
			heading = strings.ToLower(heading)
			columnHeadings[heading] = i
		}
	}

	// Let's pull out the column numbers, by name,
	// of the columns we'll care about when constructing scryfall-API queries.
	cardNameCol, ok := columnHeadings["card name"]
	if !ok {
		return fmt.Errorf(`no "Card name" column`)
	}
	setCodeCol, ok := columnHeadings["set code"]
	if !ok {
		return fmt.Errorf(`no "Set code" column`)
	}
	foilCol, ok := columnHeadings["foil"]
	if !ok {
		return fmt.Errorf(`no "Foil" column`)
	}
	lastUpdatedCol, ok := columnHeadings["last updated"]
	if !ok {
		return fmt.Errorf(`no "Last updated" column`)
	}
	priceCol, ok := columnHeadings["price"]
	if !ok {
		return fmt.Errorf(`no "Price" column`)
	}

	// This is a value representing the moment in time one day earlier than right now.
	// We'll use it in the loop below to skip rows that have been updated more recently.
	// The scryfall API docs ask that we not query the price of the same card more than once per day.
	oneDayAgo := time.Now().Add(-24 * time.Hour)

	// The base URL for contacting the scryfall Card API.
	baseURL, err := url.Parse("https://api.scryfall.com/cards/named")
	if err != nil {
		return errors.Wrap(err, "parsing base scryfall URL")
	}

	rh := rowHandler{
		sheetKey: sheetKey,
		rows:     resp.Values,

		cardNameCol:    cardNameCol,
		setCodeCol:     setCodeCol,
		foilCol:        foilCol,
		lastUpdatedCol: lastUpdatedCol,
		priceCol:       priceCol,

		valuesSvc:     s.Spreadsheets.Values,
		cardAPIClient: cardAPIClient,

		oneDayAgo: oneDayAgo,
		baseURL:   baseURL,
	}

	// Process remaining rows.
	for rownum := 1; rownum < len(resp.Values); rownum++ {
		err = rh.processRow(ctx, rownum)
		if err != nil {
			return err
		}
	}

	return nil
}
