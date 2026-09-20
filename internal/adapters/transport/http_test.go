package transport

import (
	"context"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDenialStopsProviderAndHidesBody(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(403)
		w.Write([]byte("secret reflected in error"))
	}))
	defer srv.Close()
	c := New()
	c.HTTP = srv.Client()
	body, status := c.Do(context.Background(), "GET", srv.URL, "", "")
	if status != domain.AccessDenied || len(body) != 0 {
		t.Fatal(status)
	}
	_, status = c.Do(context.Background(), "GET", srv.URL+"/other", "", "")
	if status != domain.ProviderStopped || calls != 1 {
		t.Fatal(status, calls)
	}
}
func TestRedirectDoesNotForwardKey(t *testing.T) {
	destinationCalls := 0
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destinationCalls++ }))
	defer destination.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer source.Close()
	c := New()
	c.HTTP.Transport = source.Client().Transport
	_, status := c.Do(context.Background(), "GET", source.URL, "", "fixture-key")
	if status == domain.OK || destinationCalls != 0 {
		t.Fatal("redirect followed")
	}
}
func TestContextCancelsHTTPRequest(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	c := New()
	c.HTTP = srv.Client()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, status := c.Do(ctx, "GET", srv.URL, "", "")
	if status != domain.Timeout {
		t.Fatal(status)
	}
}
func TestPaymentAndRateLimits(t *testing.T) {
	if Classify(402) != domain.PaymentRequired || !Classify(429).StopsProvider() {
		t.Fatal("no paid fallback")
	}
	for _, code := range []int{400, 404, 422} {
		if Classify(code) != domain.APIError {
			t.Fatalf("HTTP %d was not classified as provider API error", code)
		}
	}
}
