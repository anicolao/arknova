#!/usr/bin/env bash
set -euo pipefail

required=(PR_NUMBER PULL_REQUEST_TITLE RUN_ID RUN_ATTEMPT HEAD_REPOSITORY HEAD_SHA RELEASE_PATH OUTPUT_DIR)
for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    printf '%s is required\n' "$name" >&2
    exit 2
  fi
done

if [[ ! "$PR_NUMBER" =~ ^[1-9][0-9]*$ ]]; then
  printf 'PR_NUMBER must be a positive integer\n' >&2
  exit 2
fi
if [[ ! "$RUN_ID" =~ ^[1-9][0-9]*$ ]] || [[ ! "$RUN_ATTEMPT" =~ ^[1-9][0-9]*$ ]]; then
  printf 'RUN_ID and RUN_ATTEMPT must be positive integers\n' >&2
  exit 2
fi
if [[ ! "$HEAD_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  printf 'HEAD_SHA must be a lowercase 40-character hexadecimal commit\n' >&2
  exit 2
fi
if [[ ! -x "$RELEASE_PATH/bin/arknova" ]] || [[ ! -r "$RELEASE_PATH/build.json" ]] || [[ ! -r "$RELEASE_PATH/web/index.html" ]]; then
  printf 'RELEASE_PATH is not a complete Ark Nova release\n' >&2
  exit 2
fi
if ! file "$RELEASE_PATH/bin/arknova" | grep -Eq 'ELF 64-bit.*x86-64.*statically linked'; then
  printf 'Ark Nova server is not a static x86-64 Linux executable\n' >&2
  exit 2
fi
if [[ "$(jq -r '.commit' "$RELEASE_PATH/build.json")" != "$HEAD_SHA" ]]; then
  printf 'build.json commit does not match HEAD_SHA\n' >&2
  exit 2
fi

mkdir -p "$OUTPUT_DIR"
payload="$OUTPUT_DIR/release.tar.gz"
envelope="$OUTPUT_DIR/deployment.json"
if [[ -e "$payload" || -e "$envelope" ]]; then
  printf 'refusing to overwrite an existing preview package\n' >&2
  exit 2
fi

tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner \
  -C "$RELEASE_PATH" -cf - . | gzip -n -9 >"$payload"
payload_sha256="$(sha256sum "$payload" | cut -d ' ' -f 1)"
payload_size="$(wc -c <"$payload" | tr -d ' ')"
packaging_timestamp="${PACKAGING_TIMESTAMP:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"

jq -n \
  --argjson pullRequest "$PR_NUMBER" \
  --arg pullRequestTitle "$PULL_REQUEST_TITLE" \
  --argjson sourceRunId "$RUN_ID" \
  --argjson sourceRunAttempt "$RUN_ATTEMPT" \
  --arg headRepository "$HEAD_REPOSITORY" \
  --arg commit "$HEAD_SHA" \
  --arg packagingTimestamp "$packaging_timestamp" \
  --arg payloadSha256 "$payload_sha256" \
  --argjson payloadSize "$payload_size" \
  '{
    artifactFormatVersion: 1,
    pullRequest: $pullRequest,
    pullRequestTitle: $pullRequestTitle,
    sourceRunId: $sourceRunId,
    sourceRunAttempt: $sourceRunAttempt,
    headRepository: $headRepository,
    commit: $commit,
    packagingTimestamp: $packagingTimestamp,
    payloadFile: "release.tar.gz",
    payloadSha256: $payloadSha256,
    payloadSize: $payloadSize
  }' >"$envelope"
