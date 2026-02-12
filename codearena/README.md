# CodeArena

A web-based **Go playground**: write any Go code in the browser editor, hit **Run**, and watch the output stream into a terminal in real time. Every run executes in an isolated **Kubernetes pod** (or a local subprocess in dev) and is hard-killed after **10 seconds**.

## Features

- Browser code editor (React UI) with a live terminal — output streams as your program prints it
- Run **any** Go program: no problems, no test cases, just code and a Run button
- Every run is isolated: one ephemeral Kubernetes Job pod per run (`EXECUTOR=k8s`), or a subprocess in dev (`EXECUTOR=local`)
- Hard 10s wall-clock limit (`RUN_TIMEOUT_MS`) — infinite loops come back as `time_limit_exceeded`
- Clear run statuses: `success`, `compile_error`, `runtime_error`, `time_limit_exceeded` (plus `queued`/`running`/`internal_error`)
- Async pipeline: Kafka `runs` topic in, streaming `run-events` (status/chunk/done) out, pushed to the browser over WebSocket
- Run history per user, JWT auth, pre-seeded demo account, idempotent migrations applied at server startup

## Quickstart

### Path 1 — everything in Docker

```bash
docker compose up --build
```

Open <http://localhost:8080>, log in with **demo / demo123**, type some Go, hit Run, and watch the terminal.

> In compose the worker uses `EXECUTOR=local` so one command gives you the full product without a cluster. The production design runs code in Kubernetes Job pods — see [Deploying to Kubernetes (GKE)](#deploying-to-kubernetes-gke).

### Path 2 — local dev (Go on your machine, infra in Docker)

```bash
make dev-infra    # postgres :5432 + kafka :9094 in docker
make run-server   # terminal 1 — API server on :8080, runs migrations, seeds demo user
make run-worker   # terminal 2 — run worker with EXECUTOR=local
```

Then open <http://localhost:8080> (demo / demo123). The React frontend lives in `frontend/` and is built into `web/` (served by the server); `make help` lists every target.

## Architecture

```mermaid
flowchart LR
    B[Browser<br/>React editor + terminal<br/>WebSocket]
    S[Go API server<br/>REST /api/* + WS /ws]
    P[(Postgres<br/>users, runs)]
    K1[[Kafka topic<br/>runs]]
    W[Go run worker<br/>group: codearena-workers]
    J[K8s Job pod<br/>codearena-runner<br/>or local subprocess]
    K2[[Kafka topic<br/>run-events<br/>status / chunk / done]]

    B -->|POST /api/runs| S
    S -->|read/write| P
    S -->|produce| K1
    K1 -->|consume| W
    W -->|launch + follow log| J
    W -->|update status/output| P
    W -->|produce| K2
    K2 -->|consume, group: codearena-server| S
    S -->|WS stream to terminal| B
```

### Run lifecycle (including streaming and the 10s kill)

```mermaid
sequenceDiagram
    participant B as Browser
    participant S as API Server
    participant P as Postgres
    participant K as Kafka
    participant W as Worker
    participant R as Runner pod

    B->>S: POST /api/runs {code}
    S->>P: INSERT run (status=queued)
    S->>K: produce "runs" {run_id}
    S-->>B: 202 {id, status: queued}
    B->>S: WS /ws (subscribe)
    K->>W: consume {run_id}
    W->>P: status=running
    W->>K: produce run-events {type:status, status:running}
    K->>S: consume
    S-->>B: WS "running"
    W->>R: create ConfigMap(main.go) + Job, follow pod log
    R->>R: go build, then exec program under `timeout`
    loop while program prints
        R-->>W: log lines (streamed)
        W->>K: produce run-events {type:chunk, data:"..."}
        K->>S: consume chunk
        S-->>B: WS chunk -> terminal
    end
    alt program exceeds 10s
        R->>R: timeout kills process (exit 124)
        R-->>W: __CODEARENA_RESULT__{status:time_limit_exceeded,...}
    else program finishes
        R-->>W: __CODEARENA_RESULT__{status:success|runtime_error,...}
    end
    W->>P: UPDATE run (status, output, exit_code, runtime_ms)
    W->>K: produce run-events {type:done, status, runtime_ms}
    K->>S: consume done
    S-->>B: WS "done" -> terminal shows final status
```

## Components

| Component | Path | Role |
|---|---|---|
| API server | `./cmd/server` | Serves the built UI from `web/`, REST API, WebSocket; runs migrations; produces to `runs`; consumes `run-events` (group `codearena-server`) and streams to WS clients |
| Run worker | `./cmd/worker` | Consumes `runs` (group `codearena-workers`), executes code via local or k8s executor, streams output chunks, updates Postgres, produces `run-events` |
| Runner image | `runner/` | `golang:1.26-alpine` + `run.sh`; compiles and runs the program, streams output to the pod log, ends with a `__CODEARENA_RESULT__` JSON trailer |
| Frontend | `frontend/` | React + Vite app (editor + terminal), built into `web/` by `Dockerfile.server` |
| Migrations | `migrations/` | Idempotent SQL applied by the server at startup from `MIGRATIONS_DIR` |
| K8s manifests | `deploy/k8s/` | Namespace, Postgres, Kafka, server, worker, RBAC, config |

## API reference

All `/api` routes are JSON. Authenticated routes expect `Authorization: Bearer <JWT>` from login.

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/register` | – | Create account `{username, email, password}` |
| POST | `/api/login` | – | Returns `{token}` (JWT) |
| GET | `/api/me` | ✓ | Current user profile |
| POST | `/api/runs` | ✓ | Submit `{language, code}` → `202` with run id (status `queued`) |
| GET | `/api/runs` | ✓ | Current user's runs, newest first |
| GET | `/api/runs/{id}` | ✓ | One run: status, captured output, exit code, runtime |
| WS | `/ws` | ✓ | Live run events: status changes, output chunks, done |
| GET | `/healthz` | – | Liveness/readiness probe |

## Run lifecycle & statuses

| Status | Terminal? | Meaning |
|---|---|---|
| `queued` | no | Accepted, waiting for a worker |
| `running` | no | Worker picked it up; pod/subprocess executing |
| `success` | yes | Program exited 0 within the time limit |
| `compile_error` | yes | `go build` failed — compiler output is shown in the terminal |
| `runtime_error` | yes | Program exited non-zero (`process exited with code N`) |
| `time_limit_exceeded` | yes | Killed after `RUN_TIMEOUT_MS` (default 10s): `execution exceeded 10s and was killed` |
| `internal_error` | yes | Infrastructure problem (pod failed to start, malformed input, …) |

Under the hood the runner prints the program's output live and then a final line `__CODEARENA_RESULT__{"status":…,"exit_code":…,"runtime_ms":…,"error":…}` that the worker parses — see `docs/architecture.md`.

## Environment variables

| Variable | Used by | Description | Example |
|---|---|---|---|
| `PORT` | server | HTTP listen port | `8080` |
| `DATABASE_URL` | server, worker | Postgres DSN | `postgres://codearena:codearena@localhost:5432/codearena?sslmode=disable` |
| `KAFKA_BROKERS` | server, worker | Comma-separated brokers | `localhost:9094` (host) / `kafka:9092` (compose) |
| `JWT_SECRET` | server | HMAC secret for auth tokens | `openssl rand -hex 32` |
| `EXECUTOR` | worker | `local` (subprocess) or `k8s` (Job pods) | `k8s` |
| `K8S_NAMESPACE` | worker | Namespace for runner Jobs | `codearena` |
| `RUNNER_IMAGE` | worker | Runner pod image | `codearena-runner:latest` |
| `RUN_TIMEOUT_MS` | worker (→ runner as `TIME_LIMIT_MS`) | Hard kill for a run, ms | `10000` |
| `MIGRATIONS_DIR` | server | SQL migrations applied at startup | `/app/migrations` |
| `TIME_LIMIT_MS` | runner | Set by the worker on the runner pod from `RUN_TIMEOUT_MS` | `10000` |

## Deploying to Kubernetes (GKE)

This is a complete, step-by-step path from zero to a running CodeArena on Google Kubernetes Engine. You will build three images, push them to Artifact Registry, point the manifests at them, and apply `deploy/k8s/`.

### 0. Prerequisites

- A GCP project with billing enabled
- [gcloud CLI](https://cloud.google.com/sdk/docs/install) installed and authenticated (`gcloud auth login`)
- `kubectl` (`gcloud components install kubectl` works)
- Docker running locally

Set these once and the rest of the guide is copy-paste:

```bash
export PROJECT_ID="your-gcp-project-id"
export REGION="europe-west1"            # pick your region
export REPO="codearena"                 # Artifact Registry repo name
export CLUSTER="codearena"

gcloud config set project "$PROJECT_ID"
```

### 1. Enable the required APIs

```bash
gcloud services enable container.googleapis.com artifactregistry.googleapis.com
```

### 2. Create an Artifact Registry repository

```bash
gcloud artifacts repositories create "$REPO" \
  --repository-format=docker \
  --location="$REGION" \
  --description="CodeArena images"

# Let your local docker push to it
gcloud auth configure-docker "$REGION-docker.pkg.dev"
```

### 3. Build and push the three images

All three are built from the repo root. **Apple Silicon (M1/M2/M3) note:** GKE nodes are amd64 — add `--platform linux/amd64` to every `docker build` below or the pods will crash with `exec format error`.

```bash
export TAG="$REGION-docker.pkg.dev/$PROJECT_ID/$REPO"

# API server (includes the React frontend build stage)
docker build --platform linux/amd64 -t "$TAG/codearena-server:v1" -f Dockerfile.server .
docker push "$TAG/codearena-server:v1"

# Run worker
docker build --platform linux/amd64 -t "$TAG/codearena-worker:v1" -f Dockerfile.worker .
docker push "$TAG/codearena-worker:v1"

# Runner (the sandbox image each run executes in)
docker build --platform linux/amd64 -t "$TAG/codearena-runner:v1" ./runner
docker push "$TAG/codearena-runner:v1"
```

### 4. Create the cluster

Autopilot (recommended — GKE manages nodes, per-pod billing suits the bursty runner Jobs):

```bash
gcloud container clusters create-auto "$CLUSTER" --region "$REGION"
```

Or a small standard cluster:

```bash
gcloud container clusters create "$CLUSTER" \
  --region "$REGION" --num-nodes 1 --machine-type e2-standard-4
```

Then fetch credentials:

```bash
gcloud container clusters get-credentials "$CLUSTER" --region "$REGION"
```

### 5. Point the manifests at your images

Edit exactly these fields (replace the region/project with your values):

| File | Field | Set to |
|---|---|---|
| `deploy/k8s/03-server.yaml` | `spec.template.spec.containers[0].image` (the `image: codearena-server:latest` line) | `$REGION-docker.pkg.dev/$PROJECT_ID/codearena/codearena-server:v1` |
| `deploy/k8s/04-worker.yaml` | `containers[0].image` (`image: codearena-worker:latest`) | `.../codearena-worker:v1` |
| `deploy/k8s/04-worker.yaml` | the `RUNNER_IMAGE` env value (`value: codearena-runner:latest`) | `.../codearena-runner:v1` — this is the image the worker's Jobs pull, so it must be the full registry path |
| `deploy/k8s/06-config.yaml` | `JWT_SECRET` in the Secret | a long random string (`openssl rand -hex 32`) |
| `deploy/k8s/06-config.yaml` + `01-postgres.yaml` | `DATABASE_URL` password / `POSTGRES_PASSWORD` | matching non-default values |

Or patch the images with sed:

```bash
sed -i '' "s|image: codearena-server:latest|image: $TAG/codearena-server:v1|" deploy/k8s/03-server.yaml
sed -i '' "s|image: codearena-worker:latest|image: $TAG/codearena-worker:v1|" deploy/k8s/04-worker.yaml
sed -i '' "s|value: codearena-runner:latest|value: $TAG/codearena-runner:v1|"  deploy/k8s/04-worker.yaml
```

> GKE's default node service account can pull from Artifact Registry in the same project out of the box. If you use a custom node SA, grant it `roles/artifactregistry.reader`.

### 6. Deploy

```bash
kubectl apply -f deploy/k8s/     # or: make k8s-apply
```

This creates, in order: the `codearena` namespace, Postgres (StatefulSet + PVC), Kafka (single-node KRaft), the server Deployment + LoadBalancer, the worker Deployment, RBAC, and config. Postgres and Kafka take a minute or two to become Ready; the server/worker pods may restart once or twice until they are — that's normal.

### 7. Wait for the external IP

```bash
kubectl -n codearena get svc codearena-server --watch
# NAME               TYPE           CLUSTER-IP    EXTERNAL-IP     PORT(S)
# codearena-server   LoadBalancer   10.x.x.x      34.xx.xx.xx     80:...
```

When `EXTERNAL-IP` stops being `<pending>`, open `http://<EXTERNAL-IP>/` and log in with demo / demo123.

### 8. Verify

```bash
kubectl -n codearena get pods                          # everything Running/Ready
kubectl -n codearena logs deploy/codearena-server      # migrations applied, listening
kubectl -n codearena logs deploy/codearena-worker      # connected to kafka, EXECUTOR=k8s
curl "http://$(kubectl -n codearena get svc codearena-server -o jsonpath='{.status.loadBalancer.ingress[0].ip}')/healthz"
```

Smoke-test a run end to end: log in in the browser, run `fmt.Println("hello from GKE")`, and in another terminal watch the runner Job appear and vanish:

```bash
kubectl -n codearena get jobs,pods --watch
```

### How the worker spawns runner Jobs (RBAC)

The worker Deployment runs as ServiceAccount `codearena-worker` (`deploy/k8s/05-rbac.yaml`). Its namespace-scoped Role allows exactly what the flow needs: create/get/list/watch/delete on **jobs**, **pods**, and **configmaps**, plus get/list/watch on **pods/log** (which is what streaming the log with follow requires). Per run it creates a ConfigMap with `main.go`, a Job using `RUNNER_IMAGE` mounting it at `/input`, follows the pod log to stream output, parses the `__CODEARENA_RESULT__` trailer, and deletes both. No cluster-wide permissions.

### Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `ImagePullBackOff` on server/worker | Image path typo, image not pushed, or node SA can't read Artifact Registry | `kubectl -n codearena describe pod <pod>`; re-check step 3/5; grant `roles/artifactregistry.reader` to the node SA |
| Runner Jobs fail with `ImagePullBackOff` (server/worker fine) | `RUNNER_IMAGE` in `04-worker.yaml` still says `codearena-runner:latest` instead of the full registry path | Fix the env value, `kubectl apply -f deploy/k8s/04-worker.yaml` |
| `exec format error` in pod logs | Image built on Apple Silicon without `--platform linux/amd64` | Rebuild + push all images with the flag |
| Server/worker `CrashLoopBackOff` early on | Kafka/Postgres not Ready yet | `kubectl -n codearena get pods` — wait for `kafka-0`/`postgres-0` Ready; the Deployments self-heal |
| Worker logs "connection refused" to Kafka for minutes | Single-node Kafka still electing/formatting storage | `kubectl -n codearena logs kafka-0`; give it 1–2 min; PVC full or pending → check `kubectl get pvc -n codearena` |
| `EXTERNAL-IP` stuck `<pending>` | LB provisioning (slow), or quota | Wait ~5 min; `gcloud compute addresses list`; check `kubectl -n codearena describe svc codearena-server` events |
| Runs stuck `queued` | Worker not consuming (crash, wrong `KAFKA_BROKERS`) | `kubectl -n codearena logs deploy/codearena-worker` |
| Runs end `internal_error` | Worker can't create Jobs (RBAC) or runner pod failed | Worker logs + `kubectl -n codearena get events --sort-by=.lastTimestamp` |

### Teardown

```bash
kubectl delete -f deploy/k8s/                 # or: make k8s-delete
kubectl delete pvc --all -n codearena || true # PVCs from StatefulSets survive deletes
gcloud container clusters delete "$CLUSTER" --region "$REGION"
gcloud artifacts repositories delete "$REPO" --location "$REGION"   # optional
```

### Security hardening (read this — users run arbitrary code)

Already in place: each run gets its own short-lived pod, runner pods have CPU/memory **resource limits**, the runner image runs as `nobody`, and the Job has a hard `activeDeadlineSeconds`. Recommended next steps before exposing this publicly:

- **NetworkPolicy denying all egress** from runner pods — otherwise user code can reach the cluster network and the internet
- `runAsNonRoot: true` + `allowPrivilegeEscalation: false` + read-only root fs in the runner Job's pod securityContext
- gVisor (`RuntimeClass: gvisor`, one flag on GKE Sandbox) for kernel-level isolation
- A dedicated node pool (or Autopilot workload separation) for runner pods so bursts can't starve the service plane

## Design decisions & tradeoffs

- **Why Kafka?** Runs are bursty and take seconds; a durable log decouples the HTTP path from execution and gives horizontal scaling by partition. The `run-events` topic doubles as the streaming transport from workers to whichever server replica holds the user's WebSocket.
- **Why Kubernetes Jobs?** Arbitrary user code never runs inside a trusted long-lived process: a Job pod is a fresh filesystem, cgroup limits, and a kill switch (`activeDeadlineSeconds`), and cleanup is one delete. `EXECUTOR=local` exists purely so dev doesn't need a cluster.
- **Log-as-stream:** the runner writes program output straight to stdout and the worker follows the pod log — no sidecars or extra plumbing; the `__CODEARENA_RESULT__` trailer cleanly separates output from verdict on the same channel.
- **10s hard limit** keeps pods cheap and queues short; it's `RUN_TIMEOUT_MS`, change it in one place.
- **Single language (Go)** keeps one toolchain in one runner image; the `language` column leaves the door open.

## Roadmap

- [ ] Share runs via link, fork someone's snippet
- [ ] More languages (Python, Rust) behind the same runner contract
- [ ] stdin support for interactive-ish programs
- [ ] NetworkPolicy + gVisor on runner pods by default
- [ ] HPA on worker Deployment driven by Kafka consumer lag

---

*See [docs/architecture.md](docs/architecture.md) for the deeper dive: message schemas, streaming design, executor abstraction, failure modes, and the scaling story.*
