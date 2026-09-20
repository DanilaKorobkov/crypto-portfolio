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

## Authenticated Zerion D1 gate

`cmd/zerionprobe` is a bounded contract check. It requires `ZERION_API_KEY`
from the process environment and uses only the fixed, publicly declared zero
address for the positions-envelope check; it cannot accept a user wallet. It
prints only normalized statuses and counts. It fails closed on a missing
credential, provider/transport error, incomplete or empty catalog, or an
incomplete positions response. Response bodies, chain identifiers, position
identifiers, headers, quantities, and credentials are not printed or uploaded.

The manual `zerion-contract-probe.yml` workflow is synthetic: it has read-only
repository permission, no wallet input, no artifact, no schedule, and no commit
trigger. A credential disclosed in chat may be used only for an explicitly
authorized, transient local feasibility check that neither persists nor prints
it. It must not be committed, logged, uploaded as an artifact, or installed as a
long-lived CI/production secret, and it must be rotated before production use.
Passing this gate proves only authenticated catalog reachability and the
envelope contract. It does not prove wallet-position coverage, quota safety,
pagination correctness, or portfolio completeness.

## Authenticated local result — 20 September 2026

At the user's explicit direction, the disclosed test credential was supplied to
`cmd/zerionprobe` through one process environment and immediately unset. The
probe made one catalog request and returned `transport_error`, with an incomplete
catalog, zero parsed chains, and a non-zero process exit. The credential was not
printed, written to a file, committed, or installed in GitHub.

This result is consistent with the earlier CONNECT restriction and does not
establish whether Zerion accepted the credential. Repeating authenticated calls
from this shell cannot distinguish provider behavior and is therefore stopped.
The next useful test remains a single manual run on a permitted executor; the
credential must be rotated no later than the end of the feasibility phase and
before any production or wallet-data use.

## GitHub executor publication status — 20 September 2026

The account owner completed a fresh device authorization after configuring the
test secret through GitHub's UI. Identity and repository metadata reads worked,
but capability checks for secret metadata, branch push, and workflow dispatch
all returned HTTP 403 `Resource not accessible by integration`. The secret value
was not read and the catalog workflow has therefore not run on GitHub.

The local branch is synchronized with the current `develop` history, but the
probe workflow is not present on the remote default branch. Repeating device
authorization with the same integration cannot change its permissions. The next
useful action is repository contents-write plus Actions workflow-dispatch access,
or publication of the prepared change through an already authorized GitHub UI.

## GitHub-hosted authenticated result — 20 September 2026

Persistent official GitHub CLI OAuth authorization was established with
`repo`, `workflow`, `read:org`, and `gist` scopes. PR #3 passed the synthetic
race-test workflow and was squash-merged into `develop` as
`f953ff3e9c9884893872b09e284c6f998cf2caaf`.

Manual run `35520749102` then executed the authenticated catalog probe on a
GitHub-hosted runner. It completed successfully in 26 seconds with
`catalog_status=ok`, a complete catalog, 64 parsed chains, and one request. No
wallet address, position endpoint, response body, or artifact was used in this
run. This closes authenticated catalog reachability only; it does not prove
position-schema compatibility or coverage.

The next D1 check extends the same bounded command to the fixed public zero
address. It validates only the positions envelope and pagination termination,
logs counts rather than identifiers or quantities, and still cannot accept a
user wallet. A zero-candidate result is valid contract evidence but proves no
real portfolio coverage.
