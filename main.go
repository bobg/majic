package main

import (
	"encoding/json"
	"flag"
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

// The function "main" in the package "main"
// is where a Go program begins execution.
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

	// This constructs a URL for querying the API.
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

	// The creates a new client for performing HTTP requests.
	// Unlike the default HTTP client in Go,
	// this one has a custom "Transport" object.
	// This is the component of an HTTP client
	// that is responsible for actually conducting the conversation
	// with the remote server.
	// Our custom Transport object,
	// rateLimitedRoundTripper,
	// makes sure that no two requests to the server
	// happen less than 100 milliseconds apart
	// (as requested in the "Good Citizenship" section at
	// https://scryfall.com/docs/api).
	cl := http.Client{
		Transport: rateLimitedRoundTripper{
			limiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 1),
		},
	}

	// This performs an HTTP GET request,
	// using the custom client created above,
	// to query the API using the URL created further above.
	resp, err := cl.Get(u.String())
	if err != nil {
		log.Fatalf("Error getting %s: %s", u, err)
	}
	defer resp.Body.Close()

	// The response from the server is in JSON format
	// ("JavaScript Object Notation," see https://www.json.org).
	// This creates a JSON decoder that reads from the response body,
	// then uses the decoder to parse that response into the variable "obj".
	var (
		dec = json.NewDecoder(resp.Body)
		obj respObj
	)
	err = dec.Decode(&obj)
	if err != nil {
		log.Fatalf("Error decoding response: %s", err)
	}

	// This creates a JSON encoder.
	// For now, we're just re-encoding the data we parsed above,
	// and showing it to the user.
	// In this case, the encoder writes to "os.Stdout",
	// which is simply this program's "standard output"
	// (see https://en.wikipedia.org/wiki/Standard_streams#Standard_output_(stdout)).
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(obj)
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

