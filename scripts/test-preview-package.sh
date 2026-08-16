#!/usr/bin/env bash
set -euo pipefail

release_path="$1"
commit="$(jq -r '.commit' "$release_path/build.json")"
test_root="$(mktemp -d)"
cleanup() {
  chmod -R u+w "$test_root" 2>/dev/null || true
  rm -rf "$test_root"
}
trap cleanup EXIT

for output in first second; do
  PR_NUMBER=5 \
  RUN_ID=1234 \
  RUN_ATTEMPT=1 \
  HEAD_REPOSITORY=anicolao/arknova \
  HEAD_SHA="$commit" \
  RELEASE_PATH="$release_path" \
  OUTPUT_DIR="$test_root/$output" \
  PACKAGING_TIMESTAMP=2026-08-16T00:00:00Z \
    package-arknova-preview
done

cmp "$test_root/first/release.tar.gz" "$test_root/second/release.tar.gz"
cmp "$test_root/first/deployment.json" "$test_root/second/deployment.json"
sha256sum --check <(
  jq -r '.payloadSha256 + "  " + $payload' \
    --arg payload "$test_root/first/release.tar.gz" \
    "$test_root/first/deployment.json"
)

mkdir "$test_root/extracted"
tar -xzf "$test_root/first/release.tar.gz" -C "$test_root/extracted"
test -x "$test_root/extracted/bin/arknova"
test -r "$test_root/extracted/build.json"
test -r "$test_root/extracted/web/index.html"
if find "$test_root/extracted" -type l -print -quit | grep -q .; then
  printf 'release archive contains a symbolic link\n' >&2
  exit 1
fi
