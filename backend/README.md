# GoGame Backend

Contains services for Phases **C through G** of `BACKEND_SCOPE.md`, with
unfinished integrations and workflows. This is not a feature-complete online
release. The Flutter app remains offline; see
[PRODUCTION_READINESS.md](../PRODUCTION_READINESS.md) for the audited status.

Stack is as C1 recommends: Go, Postgres 16, Redis 7, containerized deploys on
Fly.io.

> Packaged as `backend/` inside the Flutter repo rather than a separate
> repository. It is a self-contained Go module with no dependency on anything
> above it, so `git subtree split -P backend` extracts it with no code changes
> when you want it to stand alone.

## Layout

```
cmd/api/            HTTP + WebSocket server, game hub, matchmaking loop
cmd/worker/         scheduled jobs (see "Worker jobs" below)

internal/config/    environment configuration with startup validation
internal/logging/   structured logging (slog; text in dev, JSON elsewhere)
internal/metrics/   Prometheus collectors
internal/store/     pgx pool, Redis client, embedded migrations
internal/blob/      S3-compatible object storage (SigV4, no AWS SDK)
internal/api/       routing, middleware, handlers

internal/auth/          C2  guest / Google / refresh rotation
internal/profile/       C3  profile sync, achievements, leaderboards
internal/archive/       C4  game archive and SGF retrieval
internal/notify/        C5  device tokens, prefs, transactional outbox, FCM

internal/goban/         the rules engine: board, groups, rules, scoring, SGF
internal/clock/         D3  server-authoritative clocks
internal/game/          D2  session actor + hub with Redis routing
internal/ws/            D1  WebSocket protocol
internal/matchmaking/   D4  queues, rating expansion, pairing
internal/rooms/         D5  friend rooms and spectating
internal/chat/          D5  in-game chat, rate limit, profanity filter

internal/rating/        E1  Glicko-2
internal/anticheat/     E2  signals, scoring, review queue
internal/tournament/    E3  arena / Swiss / McMahon / knockout
internal/correspondence/E4  daily games, vacation, conditional moves

internal/clubs/         F1  clubs, forums, team matches
internal/social/        F2  follows, DMs, reports, activity feed
internal/openings/      F3  joseki / opening explorer
internal/progames/      F4  pro game library and live relays
internal/coaching/      F5  coach marketplace

internal/billing/       G1  subscriptions and IAP
                        G2  entitlements and feature gating
                        G3  ad policy

deploy/             fly.toml, docker-compose for local dependencies
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

Auth, profile, API and session integration tests use real PostgreSQL; API,
hub and matchmaking tests also need Redis. Set both variables. Some packages
skip their entire test run when `TEST_DATABASE_URL` is absent, so a successful
bare `go test ./...` does not establish integration coverage.

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
| `S3_BUCKET`, `S3_ENDPOINT`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | outside dev | — | Durable SGF storage; endpoint must use HTTPS |
| `S3_REGION` | no | `auto` | Match the storage provider |
| `TRUSTED_PROXY_CIDRS` | behind a proxy | empty | Comma-separated networks of trusted immediate proxies; otherwise forwarding headers are ignored |
| `FCM_PROJECT_ID`, `GOOGLE_APPLICATION_CREDENTIALS` | for push | — | FCM project and application credentials |
| `SHUTDOWN_GRACE` | no | `20s` | Positive duration |

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

## The rules engine and the client

`internal/goban` is a parallel implementation of the Flutter client's
`lib/src/domain`, as D2 specifies. One divergence is deliberate and worth
knowing about:

The client hashes positions with a Zobrist table seeded from Dart's
`math.Random`, whose sequence no other language reproduces. Superko is
therefore checked with **each engine's own internal hash**, which is only ever
compared against itself. Server values crossing the wire — `GAME_SNAPSHOT`,
`MOVE_PLAYED`, `game_moves.state_hash`, the opening index key — uses
`StateHash`, a SHA-256 over one byte of board size, one byte of side to move,
then row-major cell bytes (`0` empty, `1` black, `2` white). Flutter does not
yet verify server hashes. Both engines now execute the same seven rule
fixtures from `internal/goban/testdata/rules.json`.

If you ever port the engine again, reproduce `StateHash`, not the Zobrist
table.

## Worker jobs

| Job | Cadence | Phase |
|---|---|---|
| Deliver push notifications | 10s | C5 |
| Matchmaking pairing sweep | 2s (in the API, where the hub lives) | D4 |
| Clock timeout watcher | 1s (in the API) | D3 |
| Correspondence deadlines + vacation drain | 1min | E4 |
| Leaderboard refresh | 5min | C3 |
| Reap finished sessions | 30s (in the API) | D2 |
| Recover unowned active sessions; retry durable game completions | 30s (in the API; also on startup) | D2/C4/E1 |
| Prune refresh tokens, sweep rooms, decay ratings, expire subscriptions | hourly | C2/D5/E1/G1 |

## External integrations

Provider-backed paths still need staging verification. Payments and purchases
also need implementation; adding credentials alone does not enable them.

| Feature | Needs | Behaviour without it |
|---|---|---|
| Push notifications (C5) | `FCM_PROJECT_ID` + service account | Queued in the outbox, never sent |
| SGF object storage (C4) | S3 endpoint, bucket and keys | In-memory only in dev; staging/production startup fails without durable storage |
| Google sign-in (C2) | `GOOGLE_CLIENT_IDS` | `/auth/google` rejects every token |
| Purchases (G1) | Receipt validator and authenticated store webhook implementations, then credentials | Redemption and store webhooks return 503 |
| Coach payments (F5) | Real processor implementation and account | Booking returns 503 before creating a session |
| Admin review queue (E2) | `ADMIN_USER_IDS` | All `/admin/*` routes return 403 |

Unverified receipts and webhooks never grant entitlements. An unconfigured
Google client never authenticates a user, and missing payment processing
never reports a successful charge.

## Not built here

`BACKEND_SCOPE.md` includes work that is not engineering, and none of it is
in this repository:

- Provisioning Postgres, Redis and object storage in staging and production
- App Store Connect and Play Console subscription products
- Licensing a professional game database for F3/F4 (the importer is written;
  the games are not included)
- Broadcaster agreements for live relays
- Recruiting coaches, and the trust-and-safety operations behind E2 and F2
