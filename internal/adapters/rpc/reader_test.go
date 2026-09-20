package rpc

import (
	"context"
	"encoding/json"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"strings"
	"testing"
)

type fakeTransport struct {
	replies []string
	methods []string
	params  []json.RawMessage
}

func (f *fakeTransport) Do(_ context.Context, _ string, _ string, body string, _ string) ([]byte, domain.Status) {
	var call struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	json.Unmarshal([]byte(body), &call)
	f.methods = append(f.methods, call.Method)
	f.params = append(f.params, call.Params)
	raw := f.replies[0]
	f.replies = f.replies[1:]
	return []byte(raw), domain.OK
}
func response(value any) string {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": value})
	return string(b)
}
func blockReply(hash string) string {
	return response(map[string]string{"number": "0x10", "hash": "0x" + strings.Repeat(hash, 64), "timestamp": "0x64"})
}
func TestWrongChainStopsBeforeWalletRead(t *testing.T) {
	f := &fakeTransport{replies: []string{response("0x1")}}
	r := (Reader{ChainID: 8453, HTTP: f}).Read(context.Background(), []domain.Address{"fixture"})
	if r.Status != domain.WrongChain || len(f.methods) != 1 {
		t.Fatal(r, f.methods)
	}
}
func TestBalanceExactAndSameBlock(t *testing.T) {
	f := &fakeTransport{replies: []string{response("0x2105"), blockReply("a"), response("0xffffffffffffffffffffffffffffffff"), blockReply("a")}}
	r := (Reader{ChainID: 8453, HTTP: f}).Read(context.Background(), []domain.Address{"fixture"})
	if r.Status != domain.OK || r.Balances[0].Atomic != "340282366920938463463374607431768211455" {
		t.Fatal(r)
	}
	if string(f.params[2]) != `["fixture","0x10"]` || r.BlockHash == "" || r.BlockTime.Unix() != 100 {
		t.Fatal("snapshot metadata or block pinning")
	}
}
func TestReorgDiscardsUnverifiedBalances(t *testing.T) {
	f := &fakeTransport{replies: []string{response("0x1"), blockReply("a"), response("0x10"), blockReply("b")}}
	r := (Reader{ChainID: 1, HTTP: f}).Read(context.Background(), []domain.Address{"fixture"})
	if r.Status != domain.InconsistentSnapshot || r.Balances[0].Atomic != "" || r.Balances[0].Status == domain.OK {
		t.Fatal(r)
	}
}
func TestRPCErrorAndNullAreNotZero(t *testing.T) {
	for _, raw := range []string{`{"jsonrpc":"2.0","id":1,"error":{"code":-1}}`, response(nil), `{"jsonrpc":"2.0","id":2,"result":"0x1"}`} {
		f := &fakeTransport{replies: []string{raw}}
		r := (Reader{ChainID: 1, HTTP: f}).Read(context.Background(), nil)
		if r.Status == domain.OK {
			t.Fatal("invalid response accepted")
		}
	}
}
