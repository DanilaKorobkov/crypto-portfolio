# Private collection readiness

## Implemented

The CLI can write standard age-encrypted JSON checkpoints with `REPORT_RECIPIENT=age1...`. Only the public recipient enters the collector; its corresponding private identity must stay with the report consumer. Plaintext is never written by this adapter. Invalid encryption configuration is rejected before any provider access; there is no fallback to plaintext in a configured Actions run.

Encryption is tested with ephemeral, artificial fixtures. No persistent user encryption identity has been created, no live secrets have been provisioned, and no encrypted artifact delivery has been verified.

## Optional encrypted-output configuration

| Setting | Purpose | Handling |
| --- | --- | --- |
| `WALLETS_JSON` | Two portfolio EVM addresses | Private runtime configuration |
| `RPC_CONFIG_JSON` | Authorized free HTTPS RPC endpoints with stable chain slugs and chain IDs | Private runtime configuration; URLs may include API keys |
| `ZERION_API_KEY` | Optional indexed token and position discovery | Private runtime configuration; verify free quota and disable overage |
| `REPORT_RECIPIENT` | Optional age X25519 encryption recipient | Set only when the user already has a private consumer identity |

The project does not ask the user to create a consumer identity. The existing age adapter may be used only if a suitable identity and private delivery channel already exist. Do not paste provider keys, wallet configuration or any decryption identity into issues, commit messages or logs.

## Data-first priority

Live-data feasibility now precedes additional generic metric work. First qualify candidate sources without wallet data (connectivity, free-tier limits, required methods, pagination and error semantics), then add contract tests from synthetic or official examples. Only a source that passes those gates may be used for one minimal private inventory run. The first run deliberately excludes valuation, HF and APR: its purpose is to measure networks, protocols, pagination, provenance and gaps.

If no simple private input/result channel is already available when a source is ready, stop and report that single blocker. Do not compensate with public logs, artifacts, commits, more generic financial models or repeated provider retries.

## Open integration gates

1. **Supported execution API:** authenticated GitHub workflow dispatch was successfully verified historically, then its temporary OAuth credentials were removed. The current session must not be assumed authenticated. Public Actions remain synthetic-only; commits are not collection triggers.
2. **Authorized sources:** no RPC/provider credentials are configured. The historical probe of Robinhood's documented public RPC timed out; the current runtime rejects no-wallet JSON-RPC probes at its CONNECT tunnel with HTTP 403. No wallet or protocol coverage can be inferred from either result.
3. **Private delivery:** define a simple verified result channel that does not expose wallet or financial data and does not require new key custody from the user.
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

GitHub workflow dispatch was verified through temporary least-privilege OAuth access in run 35514438290. Those credentials were subsequently removed; the current session must not infer authentication from that historical success. Public unauthenticated access to `api.github.com` remains separate from repository mutations or secret administration.

Do not substitute alternate network routes, public logs or artificial trigger commits for missing private execution and delivery capabilities.

The remaining external prerequisites are private provider/wallet configuration, a simple verified private result channel, and confirmation of zero-cost account controls. Public GitHub API access alone satisfies none of these gates. The Robinhood public RPC still cannot be used from this runtime: no-wallet JSON-RPC probes are rejected by the environment's CONNECT tunnel with HTTP 403. Do not retry it through alternate routes or use public logs as a substitute for private result transport.

## Remote collection decision

The manual artifact-based collection workflow has been removed. It required the user to manage a separate decryption identity and added operational setup before the project had proved a simple private result channel. No live run was made and no portfolio secret or report was uploaded.

The existing age adapter remains an optional, tested capability; it is not a user prerequisite and must not trigger another key-setup request. Public Actions remain synthetic-only. Real collection may resume only when an already-available private result channel can return the report without publishing wallet or financial data and without requiring additional key custody from the user.
