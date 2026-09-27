#!/usr/bin/env bash
# Runs the end-to-end tests (e2e/) against github.com.
#
# The tests sign in with E2E_GH_USER, E2E_GH_PASSWORD, and
# E2E_GH_TOTP_SECRET, the setup key of the account's authenticator app. The
# environment wins; .env.e2e in this package, when it exists, fills in
# the variables the environment leaves unset. See .env.e2e.example.
# Arguments go to "playwright test", for example --headed to watch the
# browser.
set -euo pipefail

package_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$package_root"

vars=(E2E_GH_USER E2E_GH_PASSWORD E2E_GH_TOTP_SECRET E2E_GH_AUTH_STATE)
if [[ -f .env.e2e ]]; then
  declare -A preset=()
  for name in "${vars[@]}"; do
    [[ -n "${!name:-}" ]] && preset[$name]="${!name}"
  done
  set -a
  # shellcheck source=/dev/null
  source .env.e2e
  set +a
  for name in "${!preset[@]}"; do
    export "$name=${preset[$name]}"
  done
fi

missing=()
for name in E2E_GH_USER E2E_GH_PASSWORD E2E_GH_TOTP_SECRET; do
  [[ -n "${!name:-}" ]] || missing+=("$name")
done
if (( ${#missing[@]} > 0 )); then
  echo "e2e: set ${missing[*]} in the environment or in $package_root/.env.e2e" >&2
  exit 2
fi

if [[ ! -f ../engine/wasm/engine.wasm ]]; then
  echo "e2e: packages/engine/wasm/engine.wasm is missing; run \"make wasm\" at the repository root first" >&2
  exit 2
fi

exec bun run playwright test "$@"
