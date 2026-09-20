# Project instructions

- Implement application code in the latest verified stable Go release. Currently Go 1.27.1; verify official releases before upgrading the toolchain.
- Follow DDD and Clean Architecture: discovery domain, application use cases and ports, infrastructure adapters, composition root. Dependencies point inward.
- Keep on-chain quantities exact. Never convert amounts to float64 for financial calculations.
- Preserve partial data and explicit uncertainty. No missing data as zero; no complete portfolio claim from API pagination alone.
- Zero monetary budget. No paid fallback, schedules, transaction signing or private wallet keys.
- Public source only: no real wallet addresses, secrets, raw diagnostics, or private financial data in commits, logs or artifacts.
- Public Actions run synthetic checks only until a private result transport is implemented and verified.
- Run gofmt, go vet ./... and go test -race ./... for meaningful code changes.

## Design document maintenance

- Always version and commit the SDD at `docs/design/Crypto-Portfolio-SDD.md` in this repository. Do not leave SDD changes only in chat or a local file.
- For every meaningful project change, update the SDD requirements, architecture, limitations, validation results and next steps as applicable, and commit those updates together with the corresponding change.
- Preserve the dated implementation history and distinguish historical limitations from the current status. Record what was actually verified; never claim live collection from synthetic tests.
- Publish the full engineering document with wallet placeholders (`WALLET_1`, `WALLET_2`). Keep real wallet addresses, API keys, private configuration and financial reports out of it.
