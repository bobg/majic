package main

import (
	"net/http"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

// A rateLimitedRoundTripper is a RoundTripper with a rate limit.
// Its RoundTrip method calls Wait on the rate limiter
// to make sure enough time has passed since the last call.
// It wraps another RoundTripper and,
// after the Wait,
// delegates to its RoundTrip method.
// If there is no wrapped RoundTripper,
// http.DefaultTransport is used instead.
//
// See https://pkg.go.dev/net/http#RoundTripper
// for a description of the RoundTripper interface
// that this type implements.
type rateLimitedRoundTripper struct {
	limiter *rate.Limiter
	next    http.RoundTripper
}

func (rt rateLimitedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	err := rt.limiter.Wait(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "waiting for the limiter to let us through")
	}
	next := rt.next
	if next == nil {
		next = http.DefaultTransport
	}
	return next.RoundTrip(req)
}
