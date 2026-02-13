#!/bin/bash
# run.sh — CodeArena playground runner entrypoint.
#
# Inputs (provided by the worker):
#   /input/main.go   the user's Go program
#   TIME_LIMIT_MS    wall-clock limit in ms (default 10000)
#
# Behavior:
#   1. go build (30s budget). On failure the compiler output has already been
#      streamed to stdout as-is, then the result trailer reports compile_error.
#   2. On success the binary runs with its stdout/stderr connected DIRECTLY to
#      this script's stdout (-> the pod log), so output streams in real time.
#      GNU `timeout` kills it at TIME_LIMIT_MS (exit 124).
#
# CONTRACT (what the worker parses while following the pod log):
#   - everything before the trailer is the program's live output
#   - the FINAL line is exactly:
#       __CODEARENA_RESULT__{"status":"...","exit_code":N,"runtime_ms":N,"error":"..."}
#     status: success | compile_error | runtime_error | time_limit_exceeded | internal_error
#   - the script ALWAYS exits 0; the outcome travels in the trailer JSON.
#   The trailer is printed only after the program has exited, so it can never
#   precede real output. A leading newline guarantees it starts on its own line
#   even if the program's last write had no trailing newline.
#
# -e is deliberately NOT set: non-zero exits from the user program and from
# `timeout` are expected control flow.
set -uo pipefail

TIME_LIMIT_MS="${TIME_LIMIT_MS:-10000}"
INPUT_DIR="${INPUT_DIR:-/input}"
WORK="$(mktemp -d /tmp/run.XXXXXX)"

export GOCACHE="${GOCACHE:-/tmp/gocache}"
export GOPATH="${GOPATH:-/tmp/gopath}"
export GOFLAGS="${GOFLAGS:--mod=mod}"
export HOME="${HOME:-/tmp}"

# trailer <status> <exit_code> <runtime_ms> <error>
# All error strings are fixed, runner-controlled text (never user output),
# so plain printf is JSON-safe here.
trailer() {
    printf '\n__CODEARENA_RESULT__{"status":"%s","exit_code":%d,"runtime_ms":%d,"error":"%s"}\n' \
        "$1" "$2" "$3" "$4"
    exit 0
}

if [ ! -f "$INPUT_DIR/main.go" ]; then
    trailer "internal_error" 1 0 "runner: $INPUT_DIR/main.go not found"
fi

cp "$INPUT_DIR/main.go" "$WORK/main.go"
cd "$WORK"

# --- compile (30s budget) ----------------------------------------------------
# Compiler output goes straight to stdout, line by line, exactly as emitted.
timeout 30s go build -o "$WORK/prog" "$WORK/main.go" 2>&1
BUILD_RC=$?
if [ "$BUILD_RC" -ne 0 ]; then
    trailer "compile_error" "$BUILD_RC" 0 "compilation failed"
fi

# --- run ----------------------------------------------------------------------
TIMEOUT_SECS="$(awk -v ms="$TIME_LIMIT_MS" 'BEGIN { printf "%.3f", ms / 1000 }')"
LIMIT_HUMAN="$(( TIME_LIMIT_MS / 1000 ))"

# stdout/stderr are inherited (2>&1 merges them into one ordered stream), so
# the pod log receives output as the program produces it — no capture buffer.
# Go writes to os.Stdout unbuffered, so no stdbuf shim is needed.
# -k 2: if SIGTERM at the limit is ignored, SIGKILL 2s later (still exit 124).
START_MS="$(date +%s%3N)"
timeout -k 2 "${TIMEOUT_SECS}s" "$WORK/prog" </dev/null 2>&1
RC=$?
END_MS="$(date +%s%3N)"
RUNTIME_MS=$(( END_MS - START_MS ))

if [ "$RC" -eq 124 ]; then
    trailer "time_limit_exceeded" "$RC" "$RUNTIME_MS" "execution exceeded ${LIMIT_HUMAN}s and was killed"
elif [ "$RC" -eq 0 ]; then
    trailer "success" 0 "$RUNTIME_MS" ""
else
    trailer "runtime_error" "$RC" "$RUNTIME_MS" "process exited with code $RC"
fi
