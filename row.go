package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/api/sheets/v4"
)

type rowHandler struct {
	sheetKey                                                   string
	rows                                                       [][]any
	cardNameCol, setCodeCol, foilCol, lastUpdatedCol, priceCol int
	valuesSvc                                                  *sheets.SpreadsheetsValuesService
	cardAPIClient                                              *http.Client
	oneDayAgo                                                  time.Time
	baseURL                                                    *url.URL
}

func (rh rowHandler) processRow(ctx context.Context, rownum int) error {
	row := rh.rows[rownum]

	if len(row) > rh.lastUpdatedCol {
		if lastUpdated, ok := row[rh.lastUpdatedCol].(string); ok {
			when, err := time.Parse(time.RFC3339, lastUpdated)
			if err == nil && when.After(rh.oneDayAgo) {
				// If this row was updated less than one day ago,
				// skip it as requested in the scryfall API docs.
				return nil
			}
		}
	}

	var (
		cardName, setCode string
		foil, ok          bool
	)
	if len(row) <= rh.cardNameCol {
		// This row does not have a card name in it.
		return nil
	}
	cardName, ok = row[rh.cardNameCol].(string)
	if !ok {
		// The value in this row's Card Name column is somehow not a string.
		return nil
	}
	if len(row) > rh.setCodeCol {
		setCode, _ = row[rh.setCodeCol].(string)
	}

	// Make a copy of the baseURL.
	u := *rh.baseURL

	// Set the URL's query string.
	v := url.Values{}
	v.Set("exact", cardName)
	if setCode != "" {
		v.Set("set", setCode)
	}
	u.RawQuery = v.Encode()

	resp, err := rh.cardAPIClient.Get(u.String())
	if err != nil {
		return errors.Wrap(err, "querying scryfall API")
	}
	defer resp.Body.Close()

	var (
		dec = json.NewDecoder(resp.Body)
		obj respObj
	)
	err = dec.Decode(&obj)
	if err != nil {
		return errors.Wrap(err, "JSON-decoding scryfall response")
	}

	var price string
	if foil {
		price = obj.Prices.USDFoil
	} else {
		price = obj.Prices.USD
	}

	// Set the price in the spreadsheet.
	cell := cellName(rownum, rh.priceCol)
	vr := &sheets.ValueRange{Range: cell, Values: [][]any{{price}}}
	_, err = rh.valuesSvc.Update(rh.sheetKey, cell, vr).Context(ctx).ValueInputOption("RAW").Do()
	if err != nil {
		return errors.Wrapf(err, "setting price in cell %s", cell)
	}

	// Set the last-updated time.
	cell = cellName(rownum, rh.lastUpdatedCol)
	vr = &sheets.ValueRange{Range: cell, Values: [][]any{{time.Now().Format(time.RFC3339)}}}
	_, err = rh.valuesSvc.Update(rh.sheetKey, cell, vr).Context(ctx).ValueInputOption("RAW").Do()
	if err != nil {
		return errors.Wrapf(err, "setting last-updated time in cell %s", cell)
	}

	return nil
}

// This defines a type to contain the information we parse from the /cards/named endpoint.
// The actual response has many more data fields than the ones we're pulling out here.
// The complete description is at https://scryfall.com/docs/api/cards.
type respObj struct {
	Name    string    `json:"name"`
	Prices  pricesObj `json:"prices"`
	SetName string    `json:"set_name"`
}

// This defines the type of the "prices" field in a respObj.
type pricesObj struct {
	USD       string `json:"usd"`
	USDFoil   string `json:"usd_foil"`
	USDEtched string `json:"usd_etched"`
}

// Row and col are both zero-based.
func cellName(row, col int) string {
	return fmt.Sprintf("%s%d", colName(col), row+1)
}

func colName(col int) string {
	if col < 26 {
		return string(byte(col) + 'A')
	}
	return colName(col/26-1) + colName(col%26)
}
