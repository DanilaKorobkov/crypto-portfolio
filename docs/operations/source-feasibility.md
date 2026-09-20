# Public source feasibility matrix

This document records D0 transport evidence only. It contains no wallet address, API key, response body, balance, position, or financial result. HTTP reachability does not prove API coverage, correctness, completeness, or a usable free allowance.

## Probe contract

`scripts/probe-public-sources.sh` accepts no input and calls a fixed allowlist. RPC probes use only `eth_chainId`; HTTP probes use non-wallet catalog/statistics endpoints. The script prints only the source label, request kind, curl exit code, and HTTP status. Response bodies and transport diagnostics are deleted and never uploaded.

The manual `source-probe.yml` workflow is allowed under the public synthetic-only policy because it has no secrets or wallet data, has read-only repository permission, produces no artifact, and is not scheduled or triggered by commits.

## Current shell result — 20 September 2026

| Candidate | Safe probe | Result | D0 decision for this runtime |
| --- | --- | --- | --- |
| Zerion catalog | `GET /v1/chains/` without credentials | CONNECT tunnel rejected the request with HTTP 403 before an origin response; curl exit 56 / reported HTTP 000 | Blocked; API authentication and schema were not tested |
| Base Blockscout | `GET /api/v2/stats` | Same CONNECT rejection | Blocked |
| Ethereum Blockscout | `GET /api/v2/stats` | Same CONNECT rejection | Blocked |
| Base official RPC | `eth_chainId` | Same CONNECT rejection | Blocked |
| Ethereum Cloudflare RPC | `eth_chainId` | Same CONNECT rejection | Blocked |
| Ethereum Robinhood RPC | `eth_chainId` | Same CONNECT rejection | Blocked |

The common failure before the destination means this shell cannot discriminate between providers. Retrying wallet endpoints here would add no evidence and is prohibited. It also means no candidate has passed D0: `blocked` is not the same as `provider rejected`, `unsupported`, or `empty portfolio`.

## Next falsifiable test

Run the manual public-source workflow once on the standard GitHub-hosted runner. Compare its status-only matrix with this shell result:

- if every candidate is transport-blocked, reject GitHub Actions as the data-plane executor;
- if RPC works, validate chain identity and required JSON-RPC methods before any wallet input;
- if Zerion reaches the origin but requires authentication, classify it as `requires free key` and verify current quota/overage controls before requesting configuration;
- if Blockscout works, proceed to D1 schema and pagination contract tests without wallet data;
- do not begin D2 or request user secrets until at least one source passes D0 and D1.

A successful status probe does not authorize live collection. D2 still requires the separate private-input and private-result gates.
