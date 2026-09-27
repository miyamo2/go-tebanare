#!/usr/bin/env bash
# Runs the live end-to-end tests (e2e-live/) against github.com.
#
# The tests sign in with E2E_GH_USERNAME and E2E_GH_PASSWORD (and
# E2E_GH_TOTP_SECRET when the account uses an authenticator app). The
# environment wins; .env.e2e-live in this package, when it exists, fills in
# the variables the environment leaves unset. See .env.e2e-live.example. Arguments go to "playwright test", for
# example --headed to watch the browser or to enter a device verification
# code by hand.
set -euo pipefail

package_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$package_root"

vars=(E2E_GH_USERNAME E2E_GH_PASSWORD E2E_GH_TOTP_SECRET E2E_GH_AUTH_STATE)
if [[ -f .env.e2e-live ]]; then
  declare -A preset=()
  for name in "${vars[@]}"; do
    [[ -n "${!name:-}" ]] && preset[$name]="${!name}"
  done
  set -a
  # shellcheck source=/dev/null
  source .env.e2e-live
  set +a
  for name in "${!preset[@]}"; do
    export "$name=${preset[$name]}"
  done
fi

missing=()
for name in E2E_GH_USERNAME E2E_GH_PASSWORD; do
  [[ -n "${!name:-}" ]] || missing+=("$name")
done
if (( ${#missing[@]} > 0 )); then
  echo "e2e-live: set ${missing[*]} in the environment or in $package_root/.env.e2e-live" >&2
  exit 2
fi

if [[ ! -f ../engine/wasm/engine.wasm ]]; then
  echo "e2e-live: packages/engine/wasm/engine.wasm is missing; run \"make wasm\" at the repository root first" >&2
  exit 2
fi

exec bun run playwright test -c playwright.live.config.ts "$@"
