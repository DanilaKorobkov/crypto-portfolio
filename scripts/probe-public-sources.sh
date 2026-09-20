#!/usr/bin/env bash
# Non-sensitive D0 connectivity probe. It never accepts wallet addresses, API keys,
# arbitrary URLs, or prints response bodies.
set -u

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

probe_get() {
  local name="$1" url="$2" code exit_code
  code="$(curl --location --silent --show-error --max-time 15 \
    --output "$workdir/body" --write-out '%{http_code}' "$url" \
    2>"$workdir/error")"
  exit_code=$?
  printf '%s\tGET\tcurl_exit=%d\thttp=%s\n' "$name" "$exit_code" "${code:-000}"
}

probe_rpc() {
  local name="$1" url="$2" code exit_code
  code="$(curl --silent --show-error --max-time 15 \
    --header 'content-type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' \
    --output "$workdir/body" --write-out '%{http_code}' "$url" \
    2>"$workdir/error")"
  exit_code=$?
  printf '%s\tJSON-RPC eth_chainId\tcurl_exit=%d\thttp=%s\n' "$name" "$exit_code" "${code:-000}"
}

probe_get zerion-catalog https://api.zerion.io/v1/chains/
probe_get base-blockscout https://base.blockscout.com/api/v2/stats
probe_get ethereum-blockscout https://eth.blockscout.com/api/v2/stats
probe_rpc base-official https://mainnet.base.org
probe_rpc ethereum-cloudflare https://cloudflare-eth.com
probe_rpc ethereum-robinhood https://rpc.mainnet.chain.robinhood.com
