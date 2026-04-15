# Local Development Runbook

A guide for getting the Kadto backend running locally, running tests, and integrating from a frontend client.

---

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.23+ | [https://go.dev/dl](https://go.dev/dl) |
| Docker Desktop | latest | Enable Kubernetes in Settings → Kubernetes |
| kubectl | 1.29+ | Bundled with Docker Desktop; or install separately |
| Skaffold | v2.10+ | See install instructions below |
| `go-swagger` | latest | For regenerating the OpenAPI spec |

**Alternative to Docker Desktop Kubernetes:** [kind](https://kind.sigs.k8s.io) or [minikube](https://minikube.sigs.k8s.io) both work. Just make sure your `kubectl` context points at the local cluster before running `skaffold dev`.

### Install Skaffold

```bash
# macOS
brew install skaffold

# Linux / WSL2
curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64
sudo install skaffold /usr/local/bin/

# Verify
skaffold version   # must be >= v2.10
```

### Install go-swagger

```bash
go install github.com/go-swagger/go-swagger/cmd/swagger@latest
```

### Install golangci-lint (optional, for linting)

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

---

## Quick Start

```bash
# 1. Clone and enter the repo
cd backend/

# 2. Install Go dependencies
go mod download

# 3. Start everything
skaffold dev
```

That's it. Skaffold will:
1. Build the API and worker images (using the `dev` stage — full Go toolchain + [Air](https://github.com/air-verse/air))
2. Create the `kadto-local` Kubernetes namespace
3. Deploy Postgres 15+PostGIS, Redis 7 inside the cluster
4. Run `migrate up` via an init container before the API pod starts
5. Deploy the API server and background worker with **hot reload enabled**
6. Forward ports to your machine:
   - API → `http://localhost:8080`
   - Postgres → `localhost:5432`
   - Redis → `localhost:6379`

Skaffold watches for source file changes. When you edit a `.go` file, it is synced into the running container and Air recompiles and restarts the binary automatically — no pod restart, no image rebuild.

Press `Ctrl+C` to tear down all resources when done.

---

## How It Works

### Hot reload

The dev-stage Docker image runs [Air](https://github.com/air-verse/air), a Go live-reload tool. When you save a `.go` file:

1. Skaffold detects the change and copies it into the running container (`kubectl cp` under the hood)
2. Air detects the new file via inotify and runs `go build`
3. The rebuilt binary replaces the running process

Air config: `.air.api.toml` (API) and `.air.worker.toml` (worker).

> **WSL2 note:** inotify events are sometimes unreliable in WSL2. If hot reload is not triggering, use `kind` with a Linux Docker daemon instead of Docker Desktop.

### Migrations

DB migrations run automatically via an init container on the API pod. Every time the API pod starts (including after a crash or restart), it runs `migrate up` before the main container starts. `golang-migrate` is idempotent — already-applied migrations are skipped.

Migrations live in `migrations/` as numbered SQL files. Apply a new migration by adding a file there; the init container picks it up on next pod start.

To run migrations manually:

```bash
# Exec into the running API pod
kubectl exec -it -n kadto-local deployment/kadto-api -c migrate -- /api migrate up

# Or apply against the forwarded Postgres port
go run ./cmd/api migrate up
```

To roll back the last migration:

```bash
go run ./cmd/api migrate down
```

---

## Connecting to Local Infrastructure

Once `skaffold dev` is running and ports are forwarded:

### PostgreSQL

```bash
# Using psql
psql "postgres://kadto:kadto@localhost:5432/booking_platform?sslmode=disable"

# Verify extensions
\dx
```

### Redis

```bash
redis-cli -h localhost -p 6379 ping
# PONG
```

The `.env` file is **not used** in the Skaffold flow — secrets and config are mounted from `deploy/overlays/local/secret.yaml` and `deploy/overlays/local/configmap.yaml`. Edit those files to change local environment variables.

---

## Running the API and Worker Standalone (without Kubernetes)

If you want to run the Go processes natively (e.g. for debugging with `dlv`), you still need Postgres and Redis. Start them via the forwarded ports from Skaffold, or use a separate terminal with the individual binaries:

```bash
# Requires Postgres and Redis already running (forwarded by skaffold dev or separate)
cp .env.example .env
# Edit .env with local values

go run ./cmd/api      # API server on :8080
go run ./cmd/worker   # Background worker
```

---

## Running Tests

### All tests

```bash
go test ./...
```

### A specific module

```bash
go test ./internal/bookings/...
go test ./internal/auth/...
```

### With race detector (always use for concurrency work)

```bash
go test -race ./...
```

### Integration tests (require a real PostgreSQL + Redis)

Integration tests are tagged with `//go:build integration` and use `testcontainers-go` to spin up disposable DB/Redis instances automatically.

```bash
go test -tags integration ./...
```

Make sure Docker is running before executing integration tests.

### Verbose output

```bash
go test -v ./internal/bookings/...
```

---

## Linting

```bash
golangci-lint run ./...
```

---

## OpenAPI Spec

The spec at `docs/openapi.yaml` is **generated from code annotations** — never edit it by hand.

### Regenerate

```bash
swagger generate spec -o docs/openapi.yaml --scan-models
swagger validate docs/openapi.yaml
```

### Browse interactively

```bash
swagger serve docs/openapi.yaml --port 8082
# Opens http://localhost:8082/docs
```

### Add missing annotations to a module

```
/annotate-swagger <module>
```

---

## Skaffold Reference

### One-time deploy (no watch)

```bash
skaffold run
```

### Re-deploy after a manifest or Dockerfile change

Skaffold `dev` only hot-syncs `.go` files. For changes to Dockerfiles, kustomize manifests, or Air configs, stop Skaffold and restart it:

```bash
# Ctrl+C to stop, then:
skaffold dev
```

### Force a full image rebuild

```bash
skaffold dev --cache-artifacts=false
```

### Tail logs from all pods

```bash
kubectl logs -n kadto-local -l app.kubernetes.io/part-of=kadto -f --max-log-requests=10
```

### Inspect deployed resources

```bash
kubectl get all -n kadto-local
```

### Delete all local dev resources

```bash
skaffold delete
# or
kubectl delete namespace kadto-local
```

---

## Stage and Prod Deployments

Stage and prod use the distroless production image (no Go toolchain, no Air). Migrations run as a Kubernetes **Job** — apply the rendered manifests, wait for the Job to complete, then roll the Deployment.

```bash
# Stage
IMAGE_TAG=$(git rev-parse --short HEAD)
IMAGE_TAG=$IMAGE_TAG skaffold run -p stage --kube-context=<stage-context>

# In CI — wait for the migration Job before the Deployment rollout completes:
kubectl wait --for=condition=complete job/kadto-migrate \
  -n kadto-stage --timeout=120s

# Prod
IMAGE_TAG=v1.2.3 skaffold run -p prod --kube-context=<prod-context>
kubectl wait --for=condition=complete job/kadto-migrate \
  -n kadto-prod --timeout=120s
```

The migration Job manifest lives at `deploy/base/migrate-job.yaml` and is included in the stage/prod overlays via kustomize. It auto-deletes 2 minutes after completion (`ttlSecondsAfterFinished: 120`).

---

## Environment Variable Reference

Local values are in `deploy/overlays/local/secret.yaml` (secrets) and `deploy/overlays/local/configmap.yaml` (non-sensitive config). Edit those files and restart Skaffold to apply changes.

| Variable | Required | Default | Description |
|---|---|---|---|
| `APP_ENV` | No | — | `development`, `staging`, or `production` |
| `PORT` | No | `8080` | HTTP listen port |
| `DATABASE_URL` | **Yes** | — | Full PostgreSQL connection string |
| `REDIS_URL` | No | — | Redis connection string |
| `JWT_SECRET` | **Yes** | — | HS256 signing key. Generate: `openssl rand -hex 32` |
| `PAYMONGO_SECRET_KEY` | No* | — | `sk_test_...` for test, `sk_live_...` for prod |
| `PAYMONGO_WEBHOOK_SECRET` | No* | — | PayMongo webhook signing secret |
| `FCM_SERVER_KEY` | No* | — | Firebase Cloud Messaging server key |
| `APNS_KEY_ID` | No* | — | Apple Push Notification Service key ID |
| `APNS_TEAM_ID` | No* | — | Apple developer team ID |
| `APNS_BUNDLE_ID` | No* | — | iOS app bundle identifier |
| `S3_BUCKET` | No* | — | S3 or R2 bucket name |
| `S3_REGION` | No | `ap-southeast-1` | AWS region |
| `S3_ACCESS_KEY` | No* | — | S3 access key |
| `S3_SECRET_KEY` | No* | — | S3 secret key |
| `S3_ENDPOINT` | No | — | Leave empty for AWS; set for MinIO or Cloudflare R2 |

\* Only required when testing that specific feature.

---

## Frontend Integration Guide

### Base URL

```
Local:      http://localhost:8080/v1
Production: https://api.kadto.com/v1  (see docs/openapi.yaml servers block)
```

### Authentication

All protected endpoints expect a JWT in the `Authorization` header:

```
Authorization: Bearer <token>
```

Obtain a token from:
- `POST /v1/auth/register` — email + password registration
- `POST /v1/auth/login` — email + password login
- `POST /v1/auth/sso` — Google / Facebook / Apple SSO (send the provider's identity token)

The JWT payload contains `user_id`, `role` (`customer`, `owner`, `admin`), and standard `exp`/`iat` claims.

### Error shape

Every 4xx and 5xx response uses a consistent shape:

```json
{
  "error": "slot_unavailable",
  "message": "The selected time slot is no longer available"
}
```

The `error` field is machine-readable (snake_case). The `message` field is safe to display directly to users.

### Money / amounts

All monetary values are in **centavos** (integers). Divide by 100 for Philippine peso display.

```
35000 centavos → ₱350.00
```

Never send or store decimal peso amounts — always centavos.

### Timestamps

All timestamps are **UTC ISO 8601** (`2026-01-13T03:00:00Z`). Convert to `Asia/Manila` (UTC+8) client-side for display. Do not send local time to the API — always UTC.

### SSO flow (mobile)

1. Use the provider's native SDK to authenticate the user and obtain an identity token.
2. `POST /v1/auth/sso` with `{ provider, token, role? }`. For Apple first sign-in, include `apple_name: { given_name, family_name }`.
3. On `200` — store the returned JWT and proceed.
4. On `409` with `code: "email_exists"` — the email already exists under a different provider. Display a verification screen. Use the returned `pending_link_token` with `POST /v1/auth/sso/verify-link` after the user completes OTP or re-authentication.

### Profile photo upload (2-step pre-signed URL)

Never POST file bytes directly to the API. The flow is:

1. `POST /v1/users/me/photo/upload-url` → receive `{ upload_url, cdn_url }`.
2. Upload the file directly to S3 using the `upload_url` (PUT request, no auth header — the URL is pre-signed).
3. `POST /v1/users/me/photo/confirm` → server saves the `cdn_url` to the user record.

Store the `cdn_url` for display. The `upload_url` expires and must not be stored.

### Booking lifecycle (customer)

```
1. GET  /v1/availability?service_id=...&date=...   → available slots
2. POST /v1/bookings/lock                           → reserve slot (2–5 min TTL)
3. POST /v1/payments/intents                        → create PayMongo payment intent
4. (customer completes payment in app)
5. POST /v1/payments/webhook                        → PayMongo calls this automatically
6. GET  /v1/bookings/:id                            → poll or use push to confirm status
```

Booking statuses (in order): `pending` → `awaiting_approval` → `confirmed` → `completed`.
Side exits: `rejected`, `cancelled`, `rescheduled`.

### Push notifications

Register the device token immediately after the user logs in:

```
POST /v1/notifications/device-tokens
{ "token": "<FCM or APNs token>", "platform": "android" | "ios" }
```

Re-register on every app launch — tokens can change. The server deduplicates by token value.

### Pagination

List endpoints that return potentially large sets accept:

```
?page=1&limit=20
```

Response envelopes include `total`, `page`, `limit`, and `data` array.

### Rate limits

| Caller | Limit |
|---|---|
| Unauthenticated (by IP) | 100 req/min |
| Authenticated | 1000 req/min |

On `429 Too Many Requests`, wait for the `Retry-After` header value (seconds) before retrying.

### Browsing the full API spec

The OpenAPI spec documents every endpoint, request body, response shape, and enum value:

```bash
swagger serve docs/openapi.yaml --port 8082
```

Or open `docs/openapi.yaml` in any OpenAPI viewer (Swagger UI, Insomnia, Postman).

---

## Common Issues

**`skaffold dev` fails with "no context"**
Docker Desktop Kubernetes is not enabled or the `kubectl` context is not set. In Docker Desktop: Settings → Kubernetes → Enable Kubernetes. Then verify: `kubectl config current-context` (should show `docker-desktop` or your local cluster).

**API pod stuck in `Init:0/1`**
The migrate init container is running. Check its logs:
```bash
kubectl logs -n kadto-local -l app.kubernetes.io/name=kadto-api -c migrate
```
If it's waiting for Postgres, Postgres may still be starting. Wait a few seconds or check:
```bash
kubectl logs -n kadto-local -l app.kubernetes.io/name=postgres
```

**Hot reload not triggering**
inotify events from Skaffold's file sync (`kubectl cp`) may not propagate in some WSL2 configurations. Workarounds:
- Use `kind` instead of Docker Desktop Kubernetes
- Or touch the file again after saving: `touch <file>.go`

**`DATABASE_URL is required` when running Go binaries standalone**
The `.env` file is missing or not in the working directory. Copy and edit it: `cp .env.example .env`.

**`connection refused` to PostgreSQL (standalone mode)**
PostgreSQL inside the cluster is only accessible while `skaffold dev` is running and port forwarding is active. Start it first, then run the Go binary.

**`ERROR: type "geography" does not exist`**
PostGIS extension is not installed on the database. This should not happen with the Skaffold setup (the `postgis/postgis:15-3.4` image includes it). If running standalone, re-run:
```bash
psql "$DATABASE_URL" -c "CREATE EXTENSION IF NOT EXISTS postgis;"
```

**`ERROR: function uuid_generate_v4() does not exist`**
The `uuid-ossp` extension is missing. This is enabled automatically by the migration files. If running standalone before migrations:
```bash
psql "$DATABASE_URL" -c "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";"
```

**Worker not processing notifications**
Redis is not running or `REDIS_URL` is wrong. With `skaffold dev`, Redis runs in the cluster and is forwarded to `localhost:6379`. Verify: `redis-cli ping`.

**`swagger: command not found`**
Install go-swagger: `go install github.com/go-swagger/go-swagger/cmd/swagger@latest`. Make sure `$(go env GOPATH)/bin` is in your `PATH`.
