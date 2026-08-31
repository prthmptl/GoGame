# GoGame Backend

Phase C of `BACKEND_SCOPE.md`. This module covers **C1 (stack + infra)** and
**C2 (auth)**. C3–C5 and Phases D–G are not started.

Stack is as C1 recommends: Go, Postgres 16, Redis 7, containerized deploys on
Fly.io.

> Packaged as `backend/` inside the Flutter repo rather than a separate
> repository. It is a self-contained Go module with no dependency on anything
> above it, so `git subtree split -P backend` extracts it with no code changes
> when you want it to stand alone.

## Layout

```
cmd/api/          HTTP server
cmd/worker/       background jobs (token pruning today; D3 clock watcher,
                  D4 pairing loop later)
internal/config/  environment configuration with startup validation
internal/logging/ structured logging (slog; text in dev, JSON elsewhere)
internal/metrics/ Prometheus collectors
internal/store/   pgx pool, Redis client, embedded migrations
internal/auth/    C2: guest, Google, refresh rotation
internal/api/     routing, middleware, handlers
deploy/           fly.toml, docker-compose for local dependencies
```

## Running locally

```sh
make dev-up                       # postgres + redis via docker compose
export DATABASE_URL="postgres://gogame:gogame@localhost:5432/gogame?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
make run-api
```

Migrations apply automatically at startup. `APP_ENV` defaults to `dev`, which
supplies an insecure fixed `JWT_SECRET`; staging and production require a real
one of at least 32 bytes.

## Tests

```sh
export TEST_DATABASE_URL="postgres://gogame:gogame@localhost:5432/gogame?sslmode=disable"
export TEST_REDIS_URL="redis://localhost:6379/0"
make test
```

The auth tests run against a real Postgres — rotation and reuse detection are
transactional behaviour that a mock would not prove. Without
`TEST_DATABASE_URL` they skip rather than pass vacuously.

## Configuration

| Variable | Required | Default | Notes |
|---|---|---|---|
| `APP_ENV` | no | `dev` | `dev`, `staging`, `production` |
| `PORT` | no | `8080` | |
| `DATABASE_URL` | **yes** | — | Postgres 16 |
| `REDIS_URL` | **yes** | — | Redis 7 |
| `JWT_SECRET` | outside dev | — | ≥ 32 bytes; `openssl rand -base64 48` |
| `JWT_ISSUER` | no | `gogame` | |
| `GOOGLE_CLIENT_IDS` | in prod | — | comma-separated OAuth client IDs |
| `ACCESS_TOKEN_TTL` | no | `15m` | |
| `REFRESH_TOKEN_TTL` | no | `1440h` (60d) | |

## Endpoints

### `GET /healthz`
Liveness. Never touches Postgres or Redis — a database outage must not make
the orchestrator restart healthy processes.

### `GET /readyz`
Readiness. Checks Postgres and Redis; returns 503 when either is down so load
balancers drain the instance.

### `GET /metrics`
Prometheus exposition. Request counts and latency by route, auth outcomes, and
`gogame_auth_refresh_reuse_total` — **alert on any increase**, it means a
refresh token leaked.

### `POST /auth/guest`
```json
{ "displayName": "Prath" }        // optional; defaults to "Player"
```
Creates a guest user and returns `{ user, tokens }` (201). Guests have no
`auth_identities` row and are unrecoverable if the device is lost — this is
why the upgrade path below preserves the user id.

### `POST /auth/google`
```json
{ "idToken": "<Google ID token>" }
```
Verifies the token against Google's keys, accepting any configured client ID
as audience. Behaviour depends on the `Authorization` header:

- **No/invalid bearer token** → plain sign-in. An existing Google subject
  resolves to its existing user; a new one creates a user.
- **Valid bearer token** → guest upgrade. The identity links to the *existing*
  user row, so rating, games and progress survive sign-in. Returns 409
  `identity_taken` if that Google account already belongs to someone else.

### `POST /auth/refresh`
```json
{ "refreshToken": "<opaque token>" }
```
Rotates: the presented token is marked used and a successor is returned.

Refresh tokens are opaque 256-bit values, stored only as SHA-256 hashes, and
grouped into a *family* per login. Presenting an already-rotated token means
two parties hold it, so **the entire family is revoked** and both must sign in
again — returning 401 `refresh_reuse_detected`. This is the token-theft
detection C2 asks for; the alternative (revoking only the replayed token)
would let a thief keep the session while logging the victim out.

Access tokens are HS256 JWTs restricted to that one algorithm, so a token
cannot be downgraded to `alg=none`.

## Schema

`users`, `auth_identities`, `refresh_tokens` — see
`internal/store/migrations/0001_auth.up.sql`.

`users.rating` defaults to 1000 to match the Flutter client's `ProfileStore`.
E1 replaces it with per-board-size, per-time-class Glicko-2 ratings, which
need rating, deviation and volatility rather than a single integer.

## Deploying

Tagging `backend-v*` builds the image and deploys via `deploy/fly.toml`.
First-time Fly setup is documented at the top of that file.
