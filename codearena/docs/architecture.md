# CodeArena Architecture — Deep Dive

CodeArena is a web Go playground: a logged-in user writes arbitrary Go code, clicks Run, and output streams into a browser terminal. This document covers component responsibilities, the Kafka message contracts, the streaming design, the executor abstraction, the 10-second kill mechanics, failure modes, and scaling.

## Components and responsibilities

### API server (`./cmd/server`)

- Serves the built React UI from `./web`, REST under `/api/*`, WebSocket at `/ws`, health at `/healthz`
- At startup: applies SQL files from `MIGRATIONS_DIR` in lexical order (idempotent), then ensures the demo user `demo/demo123` exists
- Auth: bcrypt hashes in `users.password_hash`, JWT (HMAC, `JWT_SECRET`) issued at login
- On `POST /api/runs`: inserts a `runs` row with `status=queued` (DB write commits first), then produces `{"run_id":"<uuid>"}` to the `runs` topic
- Consumes `run-events` in group `codearena-server` and forwards each event to the WebSocket connections of that run's user — this is how output chunks reach the terminal
- **Never executes user code** (`EXECUTOR` unset)

### Run worker (`./cmd/worker`)

- Consumes `runs` in group `codearena-workers`
- Per message: loads the run from Postgres, marks it `running` (emitting a `status` event), invokes the executor, forwards output as `chunk` events **while the program runs**, then persists the final result to Postgres and emits a `done` event
- Accumulates the streamed output and stores it in `runs.output` (with `error`, `exit_code`, `runtime_ms`) so `GET /api/runs/{id}` can replay a finished run without Kafka
- Commits the Kafka offset only after the result is persisted

### Runner (`runner/`)

- Image: `golang:1.26-alpine` + bash + coreutils (GNU `timeout`, ms-precision `date`); runs as `nobody`; entrypoint `/run.sh`
- Reads `/input/main.go`, honors `TIME_LIMIT_MS` (default 10000, set by the worker from `RUN_TIMEOUT_MS`)
- Streams everything (compiler output on failure, then program stdout+stderr merged) directly to its own stdout → the pod log, unbuffered
- Always ends with the trailer line and always exits 0 (the outcome travels in the trailer, not the exit code)

### Postgres / Kafka

- Postgres is the source of truth (`users`, `runs`); Kafka is transport. `chunk` events are ephemeral by design — the durable copy of the output is the worker's final write to `runs.output`

## Kafka topics and message schemas

### `runs` — server → workers

```json
{ "run_id": "0d9c2f4e-6a1b-4c1e-9f3a-2b7d8e5f1a23" }
```

Minimal on purpose: the worker re-reads the code from Postgres, so redelivery always executes current truth and large code blobs stay out of the log. Key by `run_id`.

### `run-events` — workers → server

One schema, three `type`s:

```json
{
  "run_id":     "0d9c2f4e-6a1b-4c1e-9f3a-2b7d8e5f1a23",
  "user_id":    42,
  "type":       "status" | "chunk" | "done",
  "status":     "running | success | compile_error | runtime_error | time_limit_exceeded | internal_error",
  "stream":     "stdout",
  "data":       "hello\n",
  "exit_code":  0,
  "runtime_ms": 1372,
  "error":      ""
}
```

- **`status`** — lifecycle change (in practice: `running`); `data`/`exit_code`/`runtime_ms` unused
- **`chunk`** — a piece of live output; `data` carries the bytes, `stream` labels the source (`stdout` — the runner merges stderr into stdout so ordering is preserved); many per run
- **`done`** — terminal event; `status` is final, `exit_code`/`runtime_ms`/`error` filled, `data` unused
- `user_id` lets the server route to the right WebSocket connections without a DB lookup
- Key by `run_id` so all events of a run stay ordered within one partition

## Streaming design

The core trick is **the pod log is the stream**:

1. The runner's process writes program output straight to stdout (no capture buffer). Go programs write to `os.Stdout` unbuffered, so bytes hit the container log as they are produced
2. The worker follows the pod log (`GET pods/log?follow=true`, the same mechanism as `kubectl logs -f`) and republishes each piece as a `chunk` event
3. When the program exits, `run.sh` prints the trailer as the final line:

   ```
   __CODEARENA_RESULT__{"status":"success","exit_code":0,"runtime_ms":1372,"error":""}
   ```

   The worker scans each line for the `__CODEARENA_RESULT__` prefix; everything before it is user output, the trailer itself is never forwarded as a chunk. Because the trailer is printed by the same process *after* the program has exited, it can never race ahead of real output. A leading newline guarantees it sits on its own line even if the program's last write had no `\n`
4. On `compile_error` the compiler's diagnostics are streamed the same way (they are the "output" of the run), followed by the trailer with `"error":"compilation failed"`

## Executor abstraction

```go
type Executor interface {
    // Run executes the code, invoking onChunk for each piece of live output,
    // and returns the parsed final result (the trailer contract).
    Run(ctx context.Context, run Run, onChunk func(data string)) (Result, error)
}
```

| | `EXECUTOR=local` | `EXECUTOR=k8s` |
|---|---|---|
| Where code runs | Subprocess of the worker (worker image ships the Go toolchain for this) | Ephemeral Job pod from `RUNNER_IMAGE` in `K8S_NAMESPACE` |
| Streaming source | Pipes from the child process | Pod log with follow |
| 10s kill | Worker-side context timeout / GNU `timeout` on the subprocess | `timeout` inside `run.sh` **plus** the Job's `activeDeadlineSeconds` backstop |
| Isolation | None — **dev only** | Per-run pod: fresh fs, cgroup limits, `nobody` user |
| Used in | `docker-compose.yml`, `make run-worker` | `deploy/k8s/04-worker.yaml` (production) |

Both paths produce identical event sequences and statuses.

## The 10-second kill, precisely

- `RUN_TIMEOUT_MS` (default 10000) is worker config; the worker passes it to the runner as `TIME_LIMIT_MS`
- **k8s:** inside the pod, `timeout -k 2 10.000s ./prog` sends SIGTERM at the limit and SIGKILL 2s later if ignored; `run.sh` sees exit 124 and emits `time_limit_exceeded` with `error: "execution exceeded 10s and was killed"`. The Job's `activeDeadlineSeconds` (compile budget + limit + slack) is the backstop if the pod itself wedges — the worker then records `time_limit_exceeded`/`internal_error` from Job conditions
- **local:** the worker enforces the same wall clock on the subprocess and synthesizes the same result
- Compilation has its own 30s budget inside `run.sh` (a `timeout` around `go build`), reported as `compile_error`

## K8s run flow (EXECUTOR=k8s)

Per run the worker:

1. Creates ConfigMap `run-<run_id>` containing `main.go`
2. Creates Job `run-<run_id>`: runner image, ConfigMap mounted at `/input`, `TIME_LIMIT_MS` env, `backoffLimit: 0`, `activeDeadlineSeconds`, CPU/memory limits
3. Waits for the pod to start, then follows its log, forwarding chunks and watching for the trailer
4. Parses the trailer → final status/exit_code/runtime_ms/error
5. Deletes the Job (propagating to the pod) and the ConfigMap, success or failure

This maps exactly onto the RBAC Role (`deploy/k8s/05-rbac.yaml`): create/get/list/watch/delete on `jobs`, `pods`, `configmaps`; get/list/watch on `pods/log` (`get` on `pods/log` is what authorizes log reads, including `follow=true` streaming).

## Failure modes

| Failure | Symptom | Current behavior | Mitigation ideas |
|---|---|---|---|
| Kafka down at run submit | Produce fails after DB insert | Row stays `queued`, client sees an error | Retry produce; sweeper that re-produces stale `queued` rows |
| Kafka down mid-run | Chunks/done can't be produced | Result still lands in Postgres; terminal stalls until the UI falls back to `GET /api/runs/{id}` | Outbox relay; client-side polling fallback on WS silence |
| Pod never schedules (quota, image pull) | No log to follow | Worker times out waiting → `internal_error` with the Job condition message | Alert on rate; pre-pull runner image via DaemonSet |
| Trailer never appears (runner OOM-killed) | Log ends without `__CODEARENA_RESULT__` | Worker records `internal_error` (or `time_limit_exceeded` if the deadline fired), keeps captured output | Raise runner memory limit; report OOM from pod status |
| Worker crash mid-run | Offset uncommitted | Redelivery → the run executes again from scratch (side-effect-free by design); orphan Job with deterministic name `run-<id>` is adopted or replaced | Short-circuit if run already terminal |
| Run stuck `running` | Crash windows above | **Known gap:** stays `running` until redelivery; sticks forever if the offset was somehow committed | Reaper: requeue runs `running` > N minutes; `ttlSecondsAfterFinished` on Jobs for orphan cleanup |
| Postgres down | Everything degrades | Server 5xx; worker backs off without committing offsets | HA Postgres; queue buffers meanwhile |
| Malicious code (fork bombs, net access) | Resource abuse | Pod CPU/memory limits + 10s kill contain most of it; **egress is currently open** | NetworkPolicy deny-all egress, gVisor, dedicated node pool (see README hardening notes) |

## Scaling story

- **Partitions are the unit of parallelism**: give `runs` N partitions keyed by `run_id`; the `codearena-workers` group spreads them across replicas. `run-events` needs at least as many partitions to not bottleneck chunk throughput
- **Workers are stateless** — scale `04-worker.yaml` replicas freely; per-run work happens in runner pods, so worker CPU stays low (it's mostly log relaying)
- **HPA signal:** Kafka consumer lag on `codearena-workers` (KEDA), not CPU
- **Cluster capacity is the real throughput limit:** one concurrent run ≈ one pod ≈ one CPU for ≤10s plus compile. A dedicated/autoscaled node pool for runner pods isolates burst load
- **Server replicas and WS routing:** all replicas consume `run-events` in one group, so each event reaches exactly one replica — which may not hold that user's socket. Simplest fix at small scale: per-replica unique consumer group IDs so every replica sees every event and forwards only to its own sockets; beyond that, a pub/sub relay (or Kafka key-based sticky WS routing)
- **Postgres** scales vertically for a long time; `runs` is append-heavy with an index on `(user_id, created_at DESC)`, and old run outputs can be TTL-pruned or moved to object storage
