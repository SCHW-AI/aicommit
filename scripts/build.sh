#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
export GOCACHE="$REPO_ROOT/.gocache"
export GOMODCACHE="$REPO_ROOT/.gomodcache"
GOEXE="$(go env GOEXE)"
OUTPUT="$REPO_ROOT/aicommit$GOEXE"

cd "$REPO_ROOT"
go build -o "$OUTPUT" .
