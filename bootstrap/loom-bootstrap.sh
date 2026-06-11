#!/bin/sh
# loom-bootstrap — thin POSIX-sh first-touch (ADR-0008).
#
# Its only job is to make the loom engine present, then hand off to it. No engine
# logic lives here: detect situation, ensure the binary, exec. This is the only
# part of Loom that must run on a bare machine before the engine exists.
set -eu

REPO_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
BIN="${LOOM_BIN:-$REPO_ROOT/bin/loom}"

if [ ! -x "$BIN" ]; then
	if ! command -v go >/dev/null 2>&1; then
		echo "loom-bootstrap: no engine and no Go toolchain — install Go >=1.26, or provide a prebuilt binary at bin/loom or via LOOM_BIN" >&2
		exit 1
	fi
	echo "loom-bootstrap: building engine -> $BIN" >&2
	(cd "$REPO_ROOT" && go build -o "$BIN" ./cmd/loom)
fi

exec "$BIN" "$@"
