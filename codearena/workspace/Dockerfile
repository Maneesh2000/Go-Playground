# codearena-workspace — the base image every user workspace pod runs.
# Contains a polyglot toolchain (Node + Python) plus the workspace agent, which
# serves the filesystem + PTY over a WebSocket to the control plane.
# Build from the repo root:  docker build -f workspace/Dockerfile -t codearena-workspace .
# (Go/other languages come later via a Nix-based image — see the plan.)

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /agent ./cmd/agent

FROM node:20-bookworm
# python3 + git + bash round out a usable dev environment; slim the image after.
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 python3-pip git ca-certificates bash curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /agent /usr/local/bin/agent
RUN mkdir -p /workspace
WORKDIR /workspace
ENV AGENT_PORT=8081 \
    WORKSPACE_ROOT=/workspace \
    PREVIEW_PORT=3000 \
    SHELL=/bin/bash
EXPOSE 8081 3000
ENTRYPOINT ["/usr/local/bin/agent"]
