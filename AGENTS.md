# Project instructions

- Implement application code in the latest verified stable Go release. Currently Go 1.27.1; verify official releases before upgrading the toolchain.
- Follow DDD and Clean Architecture: discovery domain, application use cases and ports, infrastructure adapters, composition root. Dependencies point inward.
- Keep on-chain quantities exact. Never convert amounts to float64 for financial calculations.
- Preserve partial data and explicit uncertainty. No missing data as zero; no complete portfolio claim from API pagination alone.
- Zero monetary budget. No paid fallback, schedules, transaction signing or private wallet keys.
- Public source only: no real wallet addresses, secrets, personal SDD, raw diagnostics, or private financial data in commits, logs or artifacts.
- Public Actions run synthetic checks only until a private result transport is implemented and verified.
- Run gofmt, go vet ./... and go test -race ./... for meaningful code changes.
