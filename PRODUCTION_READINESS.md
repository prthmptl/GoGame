# Production readiness audit — 2026-09-10

The offline Flutter app and Go backend have received a bug-fix and regression-test
pass against the roadmap, continuation, and backend scope documents. This is not
a certification that the complete online platform is ready for production.

The source documents describe the intended product. Some architectural details
are historical: the application has migrated from Kotlin to Flutter, and backend
services now live in this repository. The mobile app still runs offline and does
not authenticate with, sync to, or play games through the Go backend. These facts
are reflected in both READMEs; the original requirements documents are preserved.

## Fixed and verified

| Area / requirement | Bug or discrepancy | Result |
|---|---|---|
| Local saves / offline-first | Clock balances, scoring marks and custom rule settings were lost; completed records could reopen as active games | Runtime state is serialized; empty games are saved; save/archive operations are ordered; archive insert and current-game deletion share a SQLite transaction |
| Local clocks | Time between a tap and the next timer callback could be charged to the opponent | Elapsed time is charged before a move using a monotonic clock; timed-out moves are not accepted |
| AI / lifecycle | Delayed AI work could play into an undone, replaced or disposed game; computation blocked the UI isolate | AI computation runs through `compute`; generation checks discard obsolete results; background/offscreen local games pause and save |
| Local scoring / resignation | Dead marking toggled individual stones; scoring resume and AI-turn resignation behaved incorrectly | Whole groups toggle; scoring can resume; the human's resignation selects the correct loser; resignation increments the move counter in both engines |
| SGF / review | Main-line import mixed variations and move-like comments, ignored invalid moves and setup stones, and lost results/escaping | Tree-based main-line import, root setup and turn handling, bounded input, explicit invalid/unsupported-input errors, escaped exports and result tags; review preserves setup and runs analysis off the UI isolate |
| Puzzle progress | Corrupt preferences could fail loading; daily selection used a runtime hash; an old streak remained visible after missing a day | Damaged attempt entries are skipped, UTC dates select puzzles deterministically, and missed days reset the current streak |
| Touch input | Starting a drag on the board placed a stone | Stone placement waits for the completed tap; widget tests cover 9/13/19 boards |
| D2 sessions | Actors inherited short-lived request contexts; restore raced with the actor; Redis claims were overwritten or released by another owner | Hub-owned contexts, inert restore before actor startup, unique compare-and-renew/delete leases, PostgreSQL version fencing, recovery sweeps, and safe subscriber shutdown |
| D2/D3 persistence | Moves could be acknowledged before durable writes; runtime scoring/sequence state was missing after restart | Move, clock/runtime state and final result commit atomically before acceptance; ambiguous persistence failure closes the session and requires reconnect |
| D3 clocks | Invalid requests and ticks could double-charge elapsed time; ticks erased think-duration evidence; scoring time was charged on recovery | Separate charge/move timestamps, validation, paused scoring recovery and authoritative timeout handling |
| D4 matchmaking | Different rulesets could pair; cancellation raced with pairing; unconfigured/failed pairing consumed tickets | Ruleset-specific queues, one queued ticket per user, atomic reservations/cancellation, failure requeue and category-specific ratings |
| D5 friend rooms | Concurrent starts created separate games; expired rooms could start | Room lock, game creation and room link share a transaction; concurrent retries return the existing game; expired rooms reject starts |
| D5 WebSocket/chat | Chat bypassed filtering/storage; identities could switch on an existing socket; private game access and token lifetime were not enforced consistently | Moderated persistent chat, archive visibility checks including room spectators, suspension/expiry checks, fixed connection identity, frame/command limits and wrong-game rejection |
| C2 / request handling | Unlimited auth attempts, spoofed forwarding headers, unbounded/ambiguous JSON and malformed configuration | Redis auth limits, explicitly trusted proxies, bounded strict JSON, REST deadlines, required JWT expiry and startup validation |
| C4/E1 completion | Ratings, archive generation and notifications were best-effort post-game work; retries could duplicate ratings; storage failures appeared successful | Durable completion queue, idempotent rating updates, atomic notification enqueue/queue deletion and retry backoff; SGF reads can still regenerate during storage outages |
| E1 ratings | Concurrent reversed-color games could deadlock; hourly decay repeatedly charged the full inactivity period | Ordered user locks and per-game idempotency locks; decay counts only newly elapsed inactivity |
| C5 notifications | Missing FCM configuration discarded queued work; a cached OAuth source retained the first batch's cancelled context | No delivery without a sender; token refresh uses a lasting context with bounded HTTP requests |
| G1/F5 payments | Unauthenticated store webhook IDs could be recorded; fake coach processing reported successful bookings | Unconfigured store webhooks/redemption and coach booking fail with 503; no false charge/receipt success |
| Build/deployment | Go toolchain disagreed with `go.mod`; worker process overrides conflicted with Docker entrypoint; release signing silently used the debug key | Go version aligned in Docker/CI, overridable runtime command, live API autostop disabled, explicit release signing requirement and Flutter CI added |

The seven shared rules fixtures are now real: both engines read
`backend/internal/goban/testdata/rules.json`. They verify captures, rejected moves,
suicide under situational superko, passing, resignation, and handicap turns on
each supported board size. These are a regression baseline, not exhaustive proof
of all Go rules.

## Validation evidence

- Full uncached Go suite: **155 passing tests/subtests**, with `-race` and real
  isolated PostgreSQL 16 and Redis. No failed tests. The subsequently added FCM
  token-context regression also passes with `-race` against a local mock endpoint.
- `go vet ./...` and `go build ./...`: passed, including after the FCM change.
- Flutter suite: **79 tests passed**. Shared fixture tests were rerun after strict
  type annotations; `flutter analyze` then reported **no issues**.
- PostgreSQL migrations 0013–0015: applied by the integration suite; down/up SQL
  also executed successfully inside a rolled-back transaction on the isolated DB.
- SQLite SQL smoke check: fresh creation and v6-to-v7 upgrade preserve existing
  moves and populate the runtime defaults. This does not replace device testing
  of the sqflite plugin.
- `git diff --check`: passed.
- Android debug APK: attempted, **blocked by missing Android SDK**. Release AAB,
  signing guard, iOS build, physical-device lifecycle/storage tests, store flows,
  provider-backed integration tests and load/soak tests remain unverified.

Reproduce the backend checks with disposable services and both test variables:

```sh
cd backend
export TEST_DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/TEST_DB?sslmode=disable'
export TEST_REDIS_URL='redis://localhost:6379/0'
go test -race -count=1 ./...
go vet ./...
go build ./...
```

From the repository root, run `flutter test`, `flutter analyze`, and (with an
Android SDK installed) `flutter build apk --debug`. Flutter CI uses the locally
tested Flutter 3.47.3 SDK. Some Go packages skip their integration tests entirely
when `TEST_DATABASE_URL` is unset; bare test success is insufficient evidence.

## Remaining release blockers and scope gaps

1. **Online client integration (C–G).** Implement secure token storage/rotation,
   account upgrade UI, API repositories, conflict-aware sync, WebSocket reconnect
   and authoritative snapshot reconciliation. The app currently has no such
   runtime path. Remote puzzles/analysis and online social/business screens are
   also not complete.
2. **Multiple API instances (D1/D2).** Exclusive leases prevent concurrent owners,
   but WebSocket requests reaching another instance receive
   `game_on_another_instance`; there is no proxy/redirect routing implementation.
   Treat live play as a single-API-instance staging deployment until routing and
   failover are exercised. Presence and one-connection-per-device behavior remain
   incomplete. Sequential clock ticking and recovery scans need load validation.
3. **Matchmaking crash recovery (D4).** Redis reservations and PostgreSQL game
   creation still cross a transaction boundary. A process crash after reserving
   tickets or creating a game can leave tickets in `matching` until expiry, or
   create an undiscovered game. Durable pairing IDs and reconciliation are needed
   before dependable online launch. Color assignment is still ticket-age based.
4. **Competition workflows (E3/E4).** Tournament pairing algorithms and daily-game
   services are present, but orchestration into playable sessions, move/deadline
   updates, conditional moves, round advancement and complete result processing
   are not integrated end to end. The daily timeout worker does not use the live
   completion outbox. Do not advertise these as working production game modes.
5. **External services and business workflows (C5/F5/G1).** Google/FCM/S3 need
   configured staging accounts and real tests. Purchases and coach payments need
   actual validator/processor implementations, authenticated webhooks, and
   transactional/idempotent business workflows; credentials alone are insufficient.
   Notification delivery still operates per message rather than per device, so
   partial delivery and provider retries need further design and testing.
6. **Rules, review and content.** Japanese/seki adjudication, false-eye expectations,
   nonstandard six/eight-stone handicap placement and more superko/snapback cases
   require expert-reviewed fixtures. Existing handicap placement was preserved
   to avoid changing saved-game replay. SGF supports one game tree and root setup;
   later setup changes reject explicitly. Heuristic AI/review is not a strong
   engine or a calibrated win-probability service. Licensed pro games, relays and
   expanded learning content remain separate deliverables.
7. **Operational acceptance.** Signed Android builds and device QA, backup/restore,
   dependency/image vulnerability scanning, key rotation, least-privilege database
   access, metrics/alerts, general API abuse limits, and realistic load/failure
   testing remain to be completed. The continuation's device/QA and release
   checklists have not been signed off by this audit.

## Actual backend contract and rollout notes

Use `backend/internal/api/api.go` and `backend/internal/ws/protocol.go` as the
implemented API contract. Examples in the original roadmap are not all compatible:

- Auth returns `{ "user": ..., "tokens": ... }`; token expiry is `expiresAt`.
- Matchmaking uses `POST /matchmaking/tickets`, `GET /matchmaking/tickets/{id}`,
  and `DELETE /matchmaking/tickets/{id}`.
- WebSocket endpoint is `/ws`, with `{type, seq, gameId, payload}`. Authenticate
  through the first `AUTHENTICATE` frame (query-token support also exists).
- Mutating game commands use a positive, increasing sequence per player/game.
  Reconnect snapshots include `lastSeq`; clients must resume above the persisted
  value. Duplicate/old commands are rejected, not replayed as a cached success.
- Wire ruleset/clock names use the backend's lowercase spellings. State hashes
  are SHA-256 over board size, side to move and row-major cell bytes; the mobile
  client does not yet implement server hash verification.

New database migrations add session runtime/version fields (0013), the durable
completion outbox (0014), and retry scheduling (0015). SQLite advances to schema
7. Existing server games without runtime data can replay stored moves, but old
unsaved scoring marks/confirmations cannot be reconstructed.

Drain older API processes before rolling out this session-persistence change:
older binaries do not honor the new database version fence. Preserve database
backups; dropping the new runtime/outbox columns after serving new games discards
recovery information. Validate migration and recovery behavior in staging first.
Only temporary local databases were migrated during this audit. No deployment,
commit, production migration or external provider mutation was performed.
