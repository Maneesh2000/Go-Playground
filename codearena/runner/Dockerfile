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
    && mkdir -p /tmp/gocache /tmp/gopath \
    && chmod -R 0777 /tmp

# Run untrusted code as an unprivileged user, never root.
USER nobody

ENV HOME=/tmp \
    GOCACHE=/tmp/gocache \
    GOPATH=/tmp/gopath \
    GOFLAGS=-mod=mod

ENTRYPOINT ["/run.sh"]
