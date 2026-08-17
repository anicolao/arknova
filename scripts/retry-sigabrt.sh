#!/usr/bin/env bash

set -uo pipefail

max_attempts=3
attempt=1

while true; do
  if "$@"; then
    exit 0
  else
    status=$?
  fi

  if [[ "$status" -ne 134 || "$attempt" -ge "$max_attempts" ]]; then
    exit "$status"
  fi

  printf 'Command aborted with SIGABRT; retrying (%d/%d).\n' \
    "$((attempt + 1))" "$max_attempts" >&2
  attempt=$((attempt + 1))
done
