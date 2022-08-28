package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

const namedCardAPIEndpoint = "https://api.scryfall.com/cards/named"

func main() {

	// Parse command-line flags.
	var (
		foil bool
		set  string
	)
	flag.BoolVar(&foil, "foil", false, "whether to query the foil version of the card")
	flag.StringVar(&set, "set", "", "optional set containing the card")
	flag.Parse()

	// Join remaining command-line words together to make the card name.
	name := strings.Join(flag.Args(), " ")

	u, err := url.Parse(namedCardAPIEndpoint)
	if err != nil {
		log.Fatalf("Error parsing named-card API endpoint: %s", err)
	}
	v := url.Values{}
	v.Set("exact", name)
	if set != "" {
		v.Set("set", set)
	}
	u.RawQuery = v.Encode()

	cl := http.Client{
		Transport: rateLimitedRoundTripper{
			limiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 1),
		},
	}

	fmt.Printf("xxx about to get %s\n", u)

	resp, err := cl.Get(u.String())
	if err != nil {
		log.Fatalf("Error getting %s: %s", u, err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	var obj respObj
	err = dec.Decode(&obj)
	if err != nil {
		log.Fatalf("Error decoding response: %s", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(obj)
}

type respObj struct {
	Name    string    `json:"name"`
	Prices  pricesObj `json:"prices"`
	SetName string    `json:"set_name"`
}

type pricesObj struct {
	USD       string `json:"usd"`
	USDFoil   string `json:"usd_foil"`
	USDEtched string `json:"usd_etched"`
}

type rateLimitedRoundTripper struct {
	limiter *rate.Limiter
}

func (rt rateLimitedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	err := rt.limiter.Wait(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "waiting for the limiter to let us through")
	}
	return http.DefaultTransport.RoundTrip(req)
}
