#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTRACT_DIR=${1:-fib-verifier}
TARGET_PATH="contracts/${CONTRACT_DIR}"

if [ ! -d "${ROOT_DIR}/${TARGET_PATH}" ]; then
  echo "Contract directory '${TARGET_PATH}' not found." >&2
  exit 1
fi

# Ensure the wasm target is available for local builds when needed.
if ! rustup target list --installed | grep -q '^wasm32-unknown-unknown$'; then
  rustup target add wasm32-unknown-unknown
fi

# Pre-build locally for fast feedback (optional but helpful when offline).
( cd "${ROOT_DIR}" && cargo build --release --target wasm32-unknown-unknown -p "${CONTRACT_DIR}" --offline 2>/dev/null || cargo build --release --target wasm32-unknown-unknown -p "${CONTRACT_DIR}" )

# Run the CosmWasm optimizer container to produce the optimized artifact.
docker run --rm \
  -v "${ROOT_DIR}":/code \
  --mount type=volume,source="$(basename "${ROOT_DIR}")_cache",target=/code/target \
  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
  cosmwasm/rust-optimizer-arm64:0.16.0 \
  "./${TARGET_PATH}"
