# codearena-runner — sandboxed execution image for playground runs.
# Contains the Go toolchain plus bash/coreutils for run.sh.
# The worker mounts /input (main.go) into the Job pod, follows the pod log to
# stream output live, and parses the final __CODEARENA_RESULT__ trailer line.
FROM golang:1.26-alpine

# bash       — run.sh interpreter
# coreutils  — GNU `timeout` and millisecond `date +%s%3N` (busybox differs)
RUN apk add --no-cache bash coreutils

COPY run.sh /run.sh
RUN chmod 0755 /run.sh \
    # Pre-create writable scratch space for the unprivileged user:
    # go build needs GOCACHE/GOPATH and a work dir it can write to.
    && mkdir -p /tmp/gocache /tmp/gopath

# Build env — shared by the cache pre-warm below AND every runtime `go build`,
# so the keys match and runtime builds are cache hits.
ENV HOME=/tmp \
    GOCACHE=/tmp/gocache \
    GOPATH=/tmp/gopath \
    GOFLAGS=-mod=mod \
    # Offline builds: the runner pod has NO network egress (deny-egress
    # NetworkPolicy). GOPROXY=off => never contact a module proxy; stdlib
    # programs compile with zero network, third-party imports fail fast with
    # a clear "cannot find module" error instead of hanging until timeout.
    # GOTOOLCHAIN=local => never try to download a different Go toolchain.
    GOPROXY=off \
    GOTOOLCHAIN=local \
    # Pure-Go (no cgo): builds need no C toolchain and match the pre-warmed
    # cache regardless of gcc availability.
    CGO_ENABLED=0

# Pre-warm the build cache with the whole standard library so each run's
# `go build` is a cache hit (~0.4s) instead of a cold ~20s+ compile that blows
# the 30s compile budget under the runner Job's 500m CPU limit. Runs as root,
# then opens /tmp so the unprivileged 'nobody' user can read and extend it.
RUN go build std \
    && chmod -R 0777 /tmp

# Run untrusted code as an unprivileged user, never root.
USER nobody

ENTRYPOINT ["/run.sh"]
