# Private collection readiness

## Implemented

The CLI can write standard age-encrypted JSON checkpoints with `REPORT_RECIPIENT=age1...`. Only the public recipient enters the collector; its corresponding private identity must stay with the report consumer. Plaintext is never written by this adapter. Invalid encryption configuration is rejected before any provider access; there is no fallback to plaintext in a configured Actions run.

Encryption is tested with ephemeral, artificial fixtures. No persistent user encryption identity has been created, no live secrets have been provisioned, and no encrypted artifact delivery has been verified.

## Configuration required for the first live run

| Setting | Purpose | Handling |
| --- | --- | --- |
| `WALLETS_JSON` | Two portfolio EVM addresses | Private runtime configuration / repository secret |
| `RPC_CONFIG_JSON` | Authorized free HTTPS RPC endpoints with chain IDs | Repository secret; URLs may include API keys |
| `ZERION_API_KEY` | Optional indexed token and position discovery | Free provider key in repository secret; verify quota and disable overage |
| `REPORT_RECIPIENT` | Public age X25519 encryption recipient | Public runtime configuration; never substitute the private identity |
| Consumer identity | Decrypt reports | Durable private consumer storage, never the public repository or collecting runner |

Generate an identity using the official `age-keygen` tool in the private consuming environment and retain it before configuring its public recipient. Use the official `age` CLI to decrypt a downloaded report. Do not paste provider keys or the decryption identity into issues, commit messages or logs.

## Open integration gates

1. **Supported execution API:** the current GitHub connector can write code and read run logs/artifacts, but does not expose secret administration or initial workflow dispatch. The current runtime has no separately authenticated GitHub CLI. GitHub itself supports both APIs; this is a limitation of the connected surface. Recurring reports should use proper workflow dispatch or another authenticated job API, not trigger commits.
2. **Authorized sources:** no RPC/provider credentials are configured. The historical probe of Robinhood's documented public RPC timed out; the current runtime rejects no-wallet JSON-RPC probes at its CONNECT tunnel with HTTP 403. No wallet or protocol coverage can be inferred from either result.
3. **Private delivery:** define a verified result channel and consumer key custody. Downloaded ciphertext must be tied to an expected run and commit. Remote collection is not enabled merely because local encryption tests passed.
4. **Zero budget:** standard public Ubuntu runners are free, but artifact storage shares a metered allowance with GitHub Packages. Verify account limits and spending controls before publishing artifacts; short retention alone does not prove zero spend. No artifact upload is enabled by the current smoke workflow.
5. **History:** encrypted run artifacts are a transport candidate, not a durable database. Choose and test persistent private storage independently.

## Verification completed

- Standard age round trip; wrong identity, modified ciphertext and truncation rejected.
- No plaintext report or leftover temporary file in a fresh encrypted output directory.
- Failed checkpoint write preserves the previous file.
- Configured Actions invocation without a recipient fails before writes/network.
- CLI produces a decryptable artificial diagnostic with the expected incomplete status.

## Sources

- [age v1.3.2](https://github.com/FiloSottile/age/releases/tag/v1.3.2)
- [age Go package](https://pkg.go.dev/filippo.io/age)
- [GitHub Actions secrets API](https://docs.github.com/en/rest/actions/secrets)
- [GitHub Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)
- [Robinhood network endpoints](https://docs.robinhood.com/chain/connecting/)


## Current verification and access setup

[CI run 35509859451](https://github.com/DanilaKorobkov/crypto-portfolio/actions/runs/35509859451) passed all 31 named tests with the race detector on Go 1.27.1. Encryption remains verified with artificial fixtures only.

The current runtime has GitHub CLI 2.96.0, but `gh auth status` reports no authenticated account and no token-bearing environment variable is configured. A public unauthenticated request to `api.github.com` now succeeds, so the earlier blanket network block is no longer current. This does not restore the interrupted device flow or grant repository mutations, secret administration, or workflow dispatch. A new login would require an external account-owner authorization and must not be treated as completed until `gh auth status` and least-privilege repository access both succeed.

The existing GitHub connector still supports its advertised repository operations. It does not supply the missing secret administration or initial workflow dispatch operations. Do not substitute alternate network routes or artificial trigger commits for these missing capabilities.

The remaining external prerequisites are authenticated, least-privilege access to the execution API; private provider/wallet configuration; verified consumer-key custody and ciphertext delivery; and confirmation of zero-cost account controls. Public GitHub API access alone satisfies none of these gates. The Robinhood public RPC still cannot be used from this runtime: no-wallet JSON-RPC probes are rejected by the environment's CONNECT tunnel with HTTP 403. Do not retry it through alternate routes or use public logs as a substitute for private result transport.

## Manual encrypted collection workflow

`.github/workflows/collect.yml` is manual-only and targets the `live-collection` GitHub environment. It has read-only repository permissions, an eight-minute job timeout and concurrency protection. The job refuses to collect unless wallets, at least one source configuration and the public age recipient are present. It then verifies that the output directory contains only `diagnostics.json.age` before uploading a one-day artifact.

The workflow's presence does not authorize a live run. Before the first dispatch, configure environment protection, verify zero-cost Actions and artifact controls, provision secrets without exposing their values, and retain the consumer identity outside GitHub. An artifact is transport, not durable history. Bind every downloaded ciphertext to the expected repository, workflow run and commit before decrypting it.

## Published environment status

PR #1 was merged into `develop` as `798f7aa1f3ec7a6279600167c88239979ca4ed44`. The `live-collection` environment exists and its deployment branch policy permits only `develop`. It has no portfolio secrets or recipient variable yet, and the encrypted workflow has not been dispatched.

The consumer must create and durably retain an age identity outside GitHub. Only its public recipient belongs in the environment variable `REPORT_RECIPIENT`. Wallets and provider configuration belong in environment secrets and must never be pasted into chat or committed. Confirm Actions spending controls before the first artifact-producing run.
