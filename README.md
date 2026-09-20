# Crypto Portfolio

Read-only portfolio discovery and diagnostics in **Go 1.27.1** (latest stable verified on 2026-09-20). DDD and Clean Architecture, standard library only.

This is stage 0, not a complete financial report. No transactions, private wallet keys, signatures or paid fallback.

## Run

```sh
go test -race ./...
go vet ./...
go run ./cmd/portfolio
```

With no configuration, the command makes no network requests and saves an explicit failed diagnostic to `output/diagnostics.json`. The successful process exit means the diagnostic was saved, not that the portfolio was collected.

Private runtime configuration:

- `WALLETS_JSON`: array of up to 20 EVM wallet addresses.
- `RPC_CONFIG_JSON`: up to eight objects with integer `chain_id` and an authorized HTTPS `url`.
- `ZERION_API_KEY`: optional discovery key; use a free plan with provider-side spending disabled.

Never commit real configuration or reports. The CLI logs only the diagnostic status.

## Design document

The [Software Design Document](docs/design/Crypto-Portfolio-SDD.md) records requirements, architecture, verified limitations and implementation progress. Keep it updated and committed together with project changes, as required by [AGENTS.md](AGENTS.md).

## Architecture

`cmd/portfolio` composes the application. `internal/domain` holds discovery concepts and report completeness rules. `internal/application` owns the collection use case and provider/storage ports. `internal/adapters` implements JSON-RPC, Zerion, bounded HTTP and private atomic JSON checkpoints. Infrastructure depends on the inner layers, not the reverse. See [architecture](docs/architecture.md).

## Behavior and limits

- RPC: verify chain ID; pin balances to one block number; record block hash/time; check the same height again and invalidate balances on a detected reorg.
- Zerion: inspect chain capability flags; read position pages with `filter[positions]=no_filter` and `filter[trash]=no_filter`; preserve partial candidates; bound pagination and requests; reject unsafe next links and conflicting duplicate IDs.
- Maximum 60 Zerion requests per run, 20 pages per endpoint and 0.4 seconds between calls. Limits do not verify the remaining account quota.
- HTTP 401/402/403/429 stops further requests to the affected host. Errors do not become zero balances. Redirects are disabled and provider error bodies are not logged.
- HTTP requests honor cancellation and a 15-second timeout; the collection context is six minutes. Queue time and the full chat round trip remain unverified.
- Response completeness is not portfolio completeness. Candidates, including spam flags, are not summed or valued. Unknown debt is not treated as zero.
- USD valuation, protocol-specific HF, LP ranges, earned fees, APR, ownership normalization and durable external history remain unimplemented.

## Public repository, private data

The manual GitHub Actions smoke workflow runs synthetic offline tests and an unconfigured command only. It uses a standard Ubuntu runner without artifact uploads or caching, and no wallet/provider secrets. No schedule is configured.

Live collection on a public runner requires a private or encrypted result channel. Raw reports, real wallet addresses and secrets must remain outside public code, logs and artifacts. The public SDD uses wallet placeholders. The public runner alone is not a completed portfolio integration.

## References

- [Go stable releases](https://go.dev/dl/)
- [Zerion positions](https://developers.zerion.io/api-reference/wallets/get-wallet-fungible-positions)
- [Zerion pagination](https://developers.zerion.io/pagination-and-filtering)
- [Zerion chains](https://developers.zerion.io/api-reference/chains/get-list-of-all-chains)
