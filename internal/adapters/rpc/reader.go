// Package rpc is an anti-corruption layer for Ethereum JSON-RPC.
package rpc

import (
	"context"
	"encoding/json"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
	"math/big"
	"regexp"
	"strconv"
	"time"
)

type Transport interface {
	Do(context.Context, string, string, string, string) ([]byte, domain.Status)
}
type Reader struct {
	Endpoint string
	ChainID  uint64
	HTTP     Transport
}

var quantityPattern = regexp.MustCompile(`^0x(0|[1-9a-fA-F][0-9a-fA-F]*)$`)
var hashPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

func (r Reader) call(ctx context.Context, method string, params any, result any) domain.Status {
	payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	raw, status := r.HTTP.Do(ctx, "POST", r.Endpoint, string(payload), "")
	if status != domain.OK {
		return status
	}
	var envelope struct {
		Version string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Version != "2.0" || envelope.ID != 1 {
		return domain.InvalidResponse
	}
	if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
		return domain.APIError
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" || json.Unmarshal(envelope.Result, result) != nil {
		return domain.InvalidResponse
	}
	return domain.OK
}

type block struct {
	Number    string `json:"number"`
	Hash      string `json:"hash"`
	Timestamp string `json:"timestamp"`
}

func (r Reader) Read(ctx context.Context, wallets []domain.Address) domain.ChainSnapshot {
	result := domain.ChainSnapshot{ChainID: r.ChainID}
	var identity string
	result.Status = r.call(ctx, "eth_chainId", []any{}, &identity)
	if result.Status != domain.OK {
		return result
	}
	actual, err := strconv.ParseUint(identity, 0, 64)
	if err != nil || !quantityPattern.MatchString(identity) {
		result.Status = domain.InvalidResponse
		return result
	}
	if actual != r.ChainID {
		result.Status = domain.WrongChain
		return result
	}
	var before block
	result.Status = r.call(ctx, "eth_getBlockByNumber", []any{"latest", false}, &before)
	if result.Status != domain.OK {
		return result
	}
	seconds, err := strconv.ParseUint(before.Timestamp, 0, 63)
	if err != nil || !quantityPattern.MatchString(before.Timestamp) || !quantityPattern.MatchString(before.Number) || !hashPattern.MatchString(before.Hash) {
		result.Status = domain.InvalidResponse
		return result
	}
	result.BlockNumber, result.BlockHash, result.BlockTime = before.Number, before.Hash, time.Unix(int64(seconds), 0).UTC()
	for _, wallet := range wallets {
		balance := domain.NativeBalance{Wallet: wallet}
		var encoded string
		balance.Status = r.call(ctx, "eth_getBalance", []any{wallet, before.Number}, &encoded)
		if balance.Status == domain.OK {
			atomic, valid := new(big.Int).SetString(encoded, 0)
			if !valid || !quantityPattern.MatchString(encoded) {
				balance.Status = domain.InvalidResponse
			} else {
				balance.Atomic = atomic.String()
			}
		}
		if balance.Status != domain.OK {
			result.Status = balance.Status
		}
		result.Balances = append(result.Balances, balance)
	}
	// Re-read the same height: do not label balances coherent across a detected reorg.
	var after block
	status := r.call(ctx, "eth_getBlockByNumber", []any{before.Number, false}, &after)
	if status != domain.OK || after.Hash != before.Hash || after.Number != before.Number {
		result.Status = domain.InconsistentSnapshot
		for i := range result.Balances {
			result.Balances[i].Status = domain.InconsistentSnapshot
			result.Balances[i].Atomic = ""
		}
	}
	return result
}
