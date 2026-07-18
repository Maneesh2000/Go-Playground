# CodeArena Architecture — Deep Dive

CodeArena is two subsystems that share one server binary, one database, and one auth layer:

1. **The Go playground** — a logged-in user writes arbitrary Go code, clicks Run, and output streams into a browser terminal. Each run executes in an **ephemeral** Kubernetes Job pod (or a local subprocess in dev). This is the original product and the subject of **Part I** below.
2. **Workspaces (a Replit-like cloud IDE)** — a user creates a **persistent** project (a long-lived Pod + PersistentVolumeClaim), edits files in a browser IDE (Monaco + file tree + terminal), runs real servers, and reaches them at a public preview URL. This is **Part II** at the end of the document.

Both are served by `./cmd/server` and gated by the same JWT auth; they differ in workload shape — *ephemeral job per run* vs *persistent pod per project*.

Part I covers component responsibilities, the Kafka message contracts, the streaming design, the executor abstraction, the 10-second kill mechanics, failure modes, and scaling. Part II covers the control-plane / agent / proxy design, the in-pod agent protocol, persistence, and preview routing.

---

# Part I — The Go playground

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
- **Offline, cache-warmed builds** (deployed as `runner:v3`): the image sets `GOPROXY=off`, `GOTOOLCHAIN=local`, `CGO_ENABLED=0`, and **pre-compiles the standard library** (`go build std`) into `GOCACHE` at build time. This is what lets the runner work under the **deny-all-egress NetworkPolicy** — `go build` never touches the network (a network hang would blow the 30s compile budget), and stdlib programs compile in ~0.4s instead of a cold ~20s+ (which, under the pod's 500m CPU limit, would time out). Third-party `import`s fail fast with a clear "cannot find module" error, which is the intended sandbox behavior

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
| Malicious code (fork bombs, net access) | Resource abuse | Pod CPU/memory limits + 10s kill contain most of it; **egress is denied** — `deploy/gke-kubeadm/runner-egress-netpol.yaml` is a deny-all-egress NetworkPolicy on runner pods (`app=codearena-run`), and the runner builds offline (`GOPROXY=off`), so submissions can neither call out nor scan the cluster | gVisor/Kata runtimeClass, dedicated node pool, seccomp (see README hardening notes) |

## Scaling story

- **Partitions are the unit of parallelism**: give `runs` N partitions keyed by `run_id`; the `codearena-workers` group spreads them across replicas. `run-events` needs at least as many partitions to not bottleneck chunk throughput
- **Workers are stateless** — scale `04-worker.yaml` replicas freely; per-run work happens in runner pods, so worker CPU stays low (it's mostly log relaying)
- **HPA signal:** Kafka consumer lag on `codearena-workers` (KEDA), not CPU
- **Cluster capacity is the real throughput limit:** one concurrent run ≈ one pod ≈ one CPU for ≤10s plus compile. A dedicated/autoscaled node pool for runner pods isolates burst load
- **Server replicas and WS routing:** all replicas consume `run-events` in one group, so each event reaches exactly one replica — which may not hold that user's socket. Simplest fix at small scale: per-replica unique consumer group IDs so every replica sees every event and forwards only to its own sockets; beyond that, a pub/sub relay (or Kafka key-based sticky WS routing)
- **Postgres** scales vertically for a long time; `runs` is append-heavy with an index on `(user_id, created_at DESC)`, and old run outputs can be TTL-pruned or moved to object storage

---

# Part II — Workspaces (cloud IDE)

Where the playground runs a single snippet in an **ephemeral** Job pod, a **workspace** is a **persistent** project: a long-lived Pod + PersistentVolumeClaim you edit in the browser (Monaco + file tree + terminal) and run real servers in, reachable at a public preview URL. It is the Replit-shaped half of CodeArena.

Conceptually it borrows Replit's split: a trusted **control plane** that manages and proxies, and an untrusted **data plane** (the workspace pods) that runs user code. Nothing here uses Kafka or the runner — it's server ↔ pod-agent over WebSockets.

## Components and responsibilities

### Control-plane server (`./cmd/server`, workspace parts)

The same server binary that serves the playground also hosts the workspace control plane. It is an ordinary Deployment on a **worker node** (it is "control plane" by role, not the Kubernetes control-plane node). Three responsibilities:

- **Workspace REST API** (`internal/httpapi/workspaces.go`): `POST/GET/DELETE /api/workspaces`, `POST /api/workspaces/{id}/start|stop`. Ownership-scoped (you can only touch your own).
- **Reconciler** (`internal/workspace`): translates a workspace row into Kubernetes objects via `client-go` — a **Deployment (0/1 replicas) + PVC (RWO) + Service + Ingress** per workspace. `Create/Start/Stop/Delete`; hibernate = scale the Deployment to 0 (PVC kept); resume = scale to 1.
- **WebSocket proxy** (`internal/httpapi/wsproxy.go`, `GET /ws/workspace/{id}`): the "Eval" role. Verifies the JWT (query param, since browsers can't set WS headers), checks ownership, **resumes the workspace if hibernated**, dials the in-pod agent, and copies frames both ways.

Backed by Postgres (`workspaces` table, migration `002_workspaces.sql`). Requires broader RBAC than the worker — `deploy/k8s/07-server-rbac.yaml` grants the `codearena-server` ServiceAccount create/get/list/watch/delete on deployments (+`/scale`), pvc, services, ingresses; read-only on pods/log.

### Agent (`./cmd/agent`, `internal/agent`)

A small, dependency-light Go binary that is the **entrypoint of every workspace pod**. It serves one WebSocket at `/agent` (ClusterIP-only, port 8081) that the control plane dials, multiplexing two services:

- **fs** (`internal/agent/fs.go`): `list/read/write/mkdir/delete/rename`, confined to `/workspace` (path-escape guarded). Feeds the file tree and Monaco.
- **term** (`internal/agent/term.go`): a real **PTY shell** (`creack/pty`) — `start/stdin/resize`, streamed bytes both ways. Drives xterm.js.

It holds none of the control plane's power (no DB, no k8s client) — just a filesystem and a shell.

### Workspace base image (`workspace/Dockerfile`)

`node:20-bookworm` (full, so it includes gcc/g++/make) + python3 + git + the agent binary. Ships Node/JS, Python, and C/C++ out of the box; Go/Java/Ruby/etc. are not preinstalled (the Nix-based image is the planned path to true polyglot support). Mounts the PVC at `/workspace`; exposes 8081 (agent, internal) and 3000 (preview, public).

### Frontend IDE (`frontend/`, React)

Monaco (editor + tabs, workers bundled locally), react-arborist (file tree), xterm.js (terminal), served as static files by the server (same origin). `agentConn.js` speaks the agent protocol over the proxy: `fs` as request/response (correlated by id), `term` as a stream. Routes: `#/workspaces`, `#/workspace/<id>`.

## Three tiers

```
                          BROWSER  (React IDE: tree | Monaco | xterm)
                                 |                         |
             (1) HTTPS + WebSocket (JWT)         (2) preview HTTP (public)
                                 v                         v
  ============================ KUBERNETES CLUSTER ============================
                                 |                         |
                   +-------------+-----------+   +----------+-----------+
                   | control-plane server    |   | ingress-nginx        |
                   |  API + WS proxy +        |   |  routes by subdomain |
                   |  reconciler + Postgres   |   +----------+-----------+
                   +-------------+-----------+              | to :3000
                                 | dials agent              v
                   +-------------+--------------------------+-----------+
                   |   WORKSPACE PODS (one per project)                 |
                   |   [ ws-A: agent + code + PVC ]  [ ws-B: ... ] ...  |
                   +---------------------------------------------------+
```

## Session flow (edit + terminal)

The browser never reaches a pod directly (agents are ClusterIP-only); the server is the sole authenticated entry point.

```
  BROWSER                    SERVER (proxy)                AGENT (in pod)
  Monaco/tree/xterm  wss+JWT  1 verify JWT     ws dial     /agent
        |            ------->  2 owner check   ------->      fs | term(PTY)
        |            <-------  3 resume pod     <-------          |
        |             frames  4 copy frames     frames           v
        |                                              fs -> /workspace (PVC)
        |                                              term -> bash PTY
```

Wire protocol (JSON text frames; terminal payloads base64): `{ "ch": "fs|term", "op": "...", ... }`. `fs` ops carry an `id` for request/response correlation; `term` is `start`/`stdin`/`data`/`resize`/`exit`.

## Lifecycle & persistence

Files live on a per-workspace **PVC** mounted at `/workspace`, provisioned by the default StorageClass (`local-path` on this cluster). The PVC's lifecycle is decoupled from the pod:

```
  STATE      DEPLOYMENT       PVC (files)
  running    1 replica  <---  attached   (edit / run)
  stopped    0 replicas       KEPT       (hibernated, ~free)
  resume     1 replica  <---  re-attached -> same files
  deleted    removed          REMOVED    (files gone)
```

`local-path` is node-local hostPath storage: durable across pod restarts/hibernation, **but not across node loss** (not replicated, and the volume is pinned to its node). Production would pin a networked/replicated StorageClass (a cloud CSI class).

## Preview URLs

Each workspace gets an Ingress `ws-<id>.<PREVIEW_BASE_DOMAIN>` → Service:3000. In the lab, `PREVIEW_BASE_DOMAIN=preview.<nodeIP>.nip.io` — **nip.io** resolves any `*.<ip>.nip.io` to that IP, giving unique per-workspace subdomains with no owned domain. Because ingress-nginx is on a NodePort (not 80), the computed `preview_url` appends `PREVIEW_URL_PORT`.

```
  visitor -> ws-<id>.preview.<nodeIP>.nip.io:<nodePort>/path
          -> nip.io resolves host to nodeIP
          -> node:<nodePort> -> ingress-nginx (routes by Host header)
          -> Service ws-<id>:3000 -> your server (0.0.0.0:3000) in the pod
```

Config env (server): `WORKSPACE_NAMESPACE`, `WORKSPACE_IMAGE`, `WORKSPACE_AGENT_PORT` (8081), `WORKSPACE_PREVIEW_PORT` (3000), `PREVIEW_BASE_DOMAIN`, `PREVIEW_URL_PORT`.

## Security posture (important)

A workspace is a **long-lived pod running untrusted code with a shell and network** — a much larger surface than the locked-down runner. Current MVP is scoped to **personal / trusted use**:

- Workspace pods currently run as **root** with **open egress** (unlike runner pods, which are `nobody` + deny-egress). No per-user ResourceQuota/LimitRange, no PSS, no gVisor/Kata.
- The preview ingress makes any server you run **publicly reachable** (firewall-limited to allowlisted IPs in this lab).
- Access control is ownership-only (`GetWorkspace(id, userID)`), enforced at the API and the proxy — there is no sharing, so effectively one user (the owner) per workspace.

Before untrusted/multi-tenant use, add: NetworkPolicies + ResourceQuota/LimitRange per user + PSS `restricted` where possible + a sandboxed runtimeClass (gVisor/Kata). See the README hardening notes.

## Known gaps / next steps

- **Terminal-tied processes:** a server started in the terminal (`python app.py &`) dies when the IDE session closes (its stdout is the PTY). A proper detached **"run" channel** in the agent (supervised process, logs streamed separately) is the fix; today the workaround is `setsid`/`nohup`.
- **No live fs-watch:** the tree loads on connect and on save; files created out-of-band (e.g. via the terminal) appear only on reopen. `fsnotify` → a `watch` channel is the planned upgrade.
- **No collaboration:** the infra is multiplayer-ready (one pod per workspace), but there's no shared editing — concurrent same-file saves are last-write-wins. Real multiplayer needs a CRDT (e.g. Yjs) + sharing/authz.
- **Fixed single preview port (3000)** and **nip.io + NodePort** URLs; production wants dynamic ports (a Port-Authority-style detector), a real wildcard domain, and TLS.
- **Nix workspace image** for reproducible, all-language environments (the Replit `/nix` model).
