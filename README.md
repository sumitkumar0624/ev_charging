# EV Charging Network Backend

A Go REST API backed by PostgreSQL for registering drivers and stations, safely
claiming connectors, completing charging sessions, calculating tiered prices,
and managing promo codes.

## Run

Prerequisites: Docker with Compose.

```bash
docker compose up --build
curl http://localhost:8080/health
```

PostgreSQL is initialized from `migrations/001_init.sql`. To rebuild an existing
local database from scratch:

```bash
docker compose down -v
docker compose up --build
```

For local development with PostgreSQL already running:

```bash
go run ./cmd/api
go test ./...
go vet ./...
```

See [docs/API.md](docs/API.md) for a complete runnable curl walkthrough.

## Architecture

Dependencies point inward:

```text
HTTP (internal/httpapi)
  -> application/domain service (internal/service)
      -> repository contract (internal/repository/repository.go)
      -> billing strategy (internal/billing)
          -> PostgreSQL adapter (internal/repository/postgres.go)
```

Key seams:

- `billing.Calculator` is a Strategy interface. A peak/load multiplier or another
  pricing model can be composed without changing session orchestration.
- `repository.Repository` keeps PostgreSQL details out of business logic and
  makes the AC-on-DC behavior testable without a database.
- Connector claiming is one repository operation because selection, row locking,
  promo redemption, connector state, and session creation must share a transaction.
- Prices are stored as integer paise. Floating point is used only for measured
  kWh; the final raw charge is rounded once to the nearest paise.
- API/domain errors map centrally to stable HTTP status families.

## Assumptions and ambiguous decisions

1. Tariff slabs are **progressive/marginal**, not retroactive. For DC, 11 kWh is
   `10×₹20 + 1×₹14 = ₹214`.
2. DC uses the example tariff: minimum ₹150; 0–10 kWh ₹20/kWh; 10–25 kWh
   ₹14/kWh; above 25 kWh ₹9/kWh. AC is assumed to be minimum ₹100, then
   ₹12/₹9/₹7 over the same boundaries. Both are seeded in the migration.
3. The minimum is applied before the promo discount. A 20% promo on a ₹150
   minimum produces ₹120. Starting and ending at 0 kWh still incurs the minimum.
4. Promo validity and usage are checked at session start. The percentage is
   snapshotted onto the session, so deleting/expiring a promo while charging
   cannot change the final bill. "Delete" is a soft delete for auditability.
5. A driver can have one registered vehicle and at most one active session. A
   connector can have at most one active session.
6. AC-to-DC fallback is permitted only if the registered vehicle supports both
   types. The physical connector remains DC, while `billed_connector_type` is AC.
   A DC request never falls back to AC.
7. "Near the driver" means great-circle distance from the coordinates supplied
   at session start. The requested connector type is preferred across the full
   radius; only when none is free is fallback considered. Within that type, the
   nearest station wins, followed by higher power. Location is trusted input;
   authentication/GPS attestation is out of scope.
8. An in-use connector cannot be manually taken out of service. Finish the
   session first. Completing a session returns the connector to `AVAILABLE`.
9. Session energy is accepted once, as requested by the PRD. Meter signatures,
   partial readings, taxes, payment capture, and currencies other than INR are
   outside scope.

## Concurrency and consistency

`ClaimConnector` uses a PostgreSQL transaction and
`SELECT ... FOR UPDATE OF c SKIP LOCKED`. Two requests cannot claim the same
connector. Partial unique indexes independently enforce one active session per
connector and per driver. Promo rows are locked before incrementing usage, so a
limited-use promo cannot be over-redeemed. Session completion locks its session
row before changing both session and connector state.

The current `READ COMMITTED` transaction is sufficient because connector rows are
locked and uniqueness is enforced by the database. Under extreme contention a
client may receive "no connector available" rather than wait; retry behavior
belongs at the client/API gateway.

## Trade-offs and scope intentionally omitted

- Plain SQL keeps the locking behavior visible and avoids ORM lifecycle surprises.
- Haversine SQL avoids requiring PostGIS for this exercise. At fleet scale, use a
  PostGIS `geography` column with a GiST index.
- Tariffs are seeded, not exposed as admin APIs, because tariff management was not
  required. Production would version immutable tariff plans and snapshot a
  `tariff_id` at session start.
- No authentication/authorization, pagination, idempotency keys, observability
  backend, or generated OpenAPI client was added. These matter in production but
  would distract from matching, billing, and transaction correctness here.
- The bonus peak multiplier, cancellation/no-show policy, and switchable
  station-selection strategies were left out to keep mandatory behavior solid.

## With more time

1. Add integration tests with Testcontainers, especially simultaneous claims,
   promo exhaustion, transaction rollback, and geospatial boundaries.
2. Add idempotency keys for start/end, auth/RBAC, pagination, OpenAPI generation,
   and structured metrics/tracing.
3. Move station selection behind a strategy that returns ranked candidates
   (nearest, cheapest, highest power), while retaining the transactional claim.
4. Add immutable tariff versions, taxes, payment authorization/capture, signed
   meter readings, and an outbox for billing/session events.

## AI usage

AI was prompted to turn the PRD into a small layered Go API, identify ambiguity,
and prioritize concurrency and billing tests. Generated structure and SQL were
reviewed against these invariants: money must not use floating-point storage,
promo terms must be snapshotted, connector allocation must be transactional, and
AC-on-DC must separate physical from billed connector type.

Ideas rejected or narrowed: an ORM (it obscured row locking), a generic
"repository per table" pattern (it split one atomic use case), microservices
(unnecessary for this scope), and implementing every bonus before testing the
mandatory billing path. The final code was kept deliberately small enough to
explain and modify during the live extension.
