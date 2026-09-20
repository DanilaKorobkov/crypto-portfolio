package zerion

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"net/url"
	"testing"
	"time"
)

type fakeHTTP struct {
	bodies   []string
	statuses []domain.Status
	urls     []string
}

func (f *fakeHTTP) Do(_ context.Context, _ string, endpoint string, _ string, _ string) ([]byte, domain.Status) {
	f.urls = append(f.urls, endpoint)
	if len(f.statuses) > 0 {
		s := f.statuses[0]
		f.statuses = f.statuses[1:]
		if s != domain.OK {
			return nil, s
		}
	}
	b := f.bodies[0]
	f.bodies = f.bodies[1:]
	return []byte(b), domain.OK
}
func listPage(kind string, ids []string, next any) string {
	rows := []map[string]any{}
	for _, id := range ids {
		rows = append(rows, map[string]any{"type": kind, "id": id, "attributes": map[string]any{}})
	}
	raw, _ := json.Marshal(map[string]any{"data": rows, "links": map[string]any{"next": next}})
	return string(raw)
}
func sessionFor(f *fakeHTTP) *session {
	d := New(f, "fixture")
	d.Interval = 0
	return &session{adapter: d}
}
func noop(list) error { return nil }
func TestPagesDedupeAndComplete(t *testing.T) {
	f := &fakeHTTP{bodies: []string{listPage("positions", []string{"a"}, "https://api.zerion.io/v1/positions/?page%5Bafter%5D=2"), listPage("positions", []string{"a", "b"}, nil)}}
	r, err := sessionFor(f).collect(context.Background(), "/v1/positions/", "positions", nil, noop)
	if err != nil || !r.complete || len(r.rows) != 2 || len(f.urls) != 2 {
		t.Fatal(r, err)
	}
}
func TestPartialSurvivesDenial(t *testing.T) {
	f := &fakeHTTP{bodies: []string{listPage("positions", []string{"a"}, "https://api.zerion.io/v1/positions/?page%5Bafter%5D=2")}, statuses: []domain.Status{domain.OK, domain.AccessDenied}}
	s := sessionFor(f)
	r, _ := s.collect(context.Background(), "/v1/positions/", "positions", nil, noop)
	if r.complete || len(r.rows) != 1 || r.status != domain.AccessDenied {
		t.Fatal(r)
	}
	_, status := s.fetch(context.Background(), "https://api.zerion.io/v1/chains/")
	if status != domain.ProviderStopped || len(f.urls) != 2 {
		t.Fatal("provider retried")
	}
}
func TestUnsafeNextURLs(t *testing.T) {
	required := url.Values{"currency": {"usd"}}
	for _, link := range []string{"https://attacker.invalid/v1/positions/?currency=usd", "http://api.zerion.io/v1/positions/?currency=usd", "https://api.zerion.io/other?currency=usd", "https://api.zerion.io/v1/positions/?currency=eur", "https://api.zerion.io/v1/positions/?currency=usd&currency=eur"} {
		if validNext(link, "/v1/positions/", required) {
			t.Fatal("unsafe URL accepted")
		}
	}
}
func TestCycleAndPageBudget(t *testing.T) {
	for _, limit := range []int{1, 20} {
		f := &fakeHTTP{bodies: []string{listPage("positions", nil, "https://api.zerion.io/v1/positions/")}}
		s := sessionFor(f)
		s.adapter.MaxPages = limit
		r, _ := s.collect(context.Background(), "/v1/positions/", "positions", nil, noop)
		if r.status != domain.PageLimit && r.status != domain.PaginationCycle {
			t.Fatal(r)
		}
		if len(f.urls) != 1 {
			t.Fatal("cycle fetched again")
		}
	}
}
func TestRequestBudgetAndCancelledContext(t *testing.T) {
	f := &fakeHTTP{}
	s := sessionFor(f)
	s.adapter.MaxRequests = 0
	if _, status := s.fetch(context.Background(), "https://api.zerion.io"); status != domain.RequestLimit {
		t.Fatal(status)
	}
	s.adapter.MaxRequests = 1
	s.next = time.Now().Add(time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, status := s.fetch(ctx, "https://api.zerion.io"); status != domain.Timeout {
		t.Fatal(status)
	}
	if len(f.urls) != 0 {
		t.Fatal("network call on exhausted/cancelled budget")
	}
}
func TestMalformedEnvelopeNotEmptySuccess(t *testing.T) {
	for _, body := range []string{`{"data":[]}`, `{"data":null,"links":{}}`, `{"data":[{}],"links":{}}`} {
		f := &fakeHTTP{bodies: []string{body}}
		r, _ := sessionFor(f).collect(context.Background(), "/v1/chains/", "chains", nil, noop)
		if r.complete || r.status != domain.InvalidResponse {
			t.Fatal(r)
		}
	}
}
func TestMissingKeyAndProviderStopMarkBothWallets(t *testing.T) {
	for _, key := range []string{"", "fixture"} {
		f := &fakeHTTP{statuses: []domain.Status{domain.PaymentRequired}}
		d := New(f, key)
		d.Interval = 0
		r, err := d.Discover(context.Background(), []domain.Address{"a", "b"}, func(domain.Discovery) error { return nil })
		if err != nil || len(r.Wallets) != 2 || r.CoverageComplete {
			t.Fatal(r, err)
		}
		if key == "" && len(f.urls) != 0 {
			t.Fatal("network without key")
		}
		if key != "" && len(f.urls) != 1 {
			t.Fatal("network after payment requirement")
		}
	}
}
func TestCheckpointErrorPropagates(t *testing.T) {
	f := &fakeHTTP{bodies: []string{listPage("chains", nil, nil)}}
	expected := errors.New("disk full")
	_, err := New(f, "fixture").Discover(context.Background(), nil, func(domain.Discovery) error { return expected })
	if !errors.Is(err, expected) {
		t.Fatal(err)
	}
}
