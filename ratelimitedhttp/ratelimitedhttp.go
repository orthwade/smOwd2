// ratelimitedhttp.go
package ratelimitedhttp

import (
	"context"
	"fmt"
	"net/http"
	"smOwd2/logs"
	"time"
)

func fillTokens(bucket chan struct{}, count int) {
	for i := 0; i < count; i++ {
		select {
		case bucket <- struct{}{}:
		default:
			return
		}
	}
}

const rps = 5
const rpm = 90

var rpsLimiter = make(chan struct{}, rps)
var rpmLimiter = make(chan struct{}, rpm)

type RateLimitedTransport struct {
	BaseTransport http.RoundTripper
}

func NewRateLimitedTransport(base http.RoundTripper) *RateLimitedTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RateLimitedTransport{BaseTransport: base}
}

func (t *RateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-rpsLimiter
	<-rpmLimiter

	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("Failed request: %w", err)
	}

	return resp, nil
}

func StartRefill(ctx context.Context) {
	logger := logs.DefaultFromCtx(ctx)

	go func() {
		ticker := time.NewTicker(1 * time.Second)

		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Stopping RPS refill")
			case <-ticker.C:
				fillTokens(rpsLimiter, rps)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(1 * time.Minute)

		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Stopping RPM refill")
			case <-ticker.C:
				fillTokens(rpmLimiter, rpm)
			}
		}
	}()
}
