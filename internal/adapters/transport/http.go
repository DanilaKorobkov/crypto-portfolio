// Package transport provides bounded HTTPS reads without redirects or error-body leaks.
package transport

import (
	"context"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	HTTP    *http.Client
	mu      sync.Mutex
	stopped map[string]bool
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, stopped: map[string]bool{}}
}
func (c *Client) Do(ctx context.Context, method, endpoint, body, key string) ([]byte, domain.Status) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, domain.MissingConfiguration
	}
	c.mu.Lock()
	blocked := c.stopped[u.Hostname()]
	c.mu.Unlock()
	if blocked {
		return nil, domain.ProviderStopped
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, domain.MissingConfiguration
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.SetBasicAuth(key, "")
	}
	response, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, domain.Timeout
		}
		if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
			return nil, domain.Timeout
		}
		return nil, domain.TransportError
	}
	defer response.Body.Close()
	status := Classify(response.StatusCode)
	if status.StopsProvider() {
		c.mu.Lock()
		if c.stopped == nil {
			c.stopped = map[string]bool{}
		}
		c.stopped[u.Hostname()] = true
		c.mu.Unlock()
	}
	if status != domain.OK {
		return nil, status
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 2_000_001))
	if err != nil {
		if ctx.Err() != nil {
			return nil, domain.Timeout
		}
		return nil, domain.TransportError
	}
	if len(raw) > 2_000_000 {
		return nil, domain.InvalidResponse
	}
	return raw, domain.OK
}
func Classify(code int) domain.Status {
	switch code {
	case 200:
		return domain.OK
	case 401:
		return domain.AuthRequired
	case 402:
		return domain.PaymentRequired
	case 403:
		return domain.AccessDenied
	case 429:
		return domain.RateLimited
	default:
		return domain.TransportError
	}
}
