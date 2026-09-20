# Retail Order Service Architecture

## Boundary and ownership

This is an application HTTP service, not an agent or a tool. It imports TruvaG3
core for Redis configuration/keyspace and logging primitives, and telemetry for
HTTP observability. It has no discovery, orchestration, model client, source
retrieval, or dependency on another application's Go packages.

| File | Responsibility |
| --- | --- |
| `main.go` | Configuration, telemetry/client/logger ownership, HTTP lifecycle |
| `catalog.go` | Catalogue selection, quantity policy, promotion resolution |
| `checkout.go` | Checkout calculation |
| `contract.go` | Request, product, order, receipt, and feed data types |
| `service.go` | Routing, request processing, retries, and outcome observations |
| `store.go` | Application-owned storage interface and Redis implementation |
| `http.go` | Bounded JSON handling and write-route authentication |
| `build.go` | Compiled provenance validation and serving-pod metadata |

Consumers define their own transport DTOs rather than importing this main
package. Business rules stay application-owned, not in framework modules.

## Order processing and storage

The service reads any accepted order before consulting today's catalogue so
retries can return the original pricing snapshot. A new order records the request,
its identity digest, receipt, and ledger in one immutable JSON value using Redis
`SET NX`. Concurrent duplicate submissions cannot overwrite that value. Changed
requests must conflict; identical retries return the accepted outcome.

The `orderStore` interface isolates external storage for focused tests. The Redis
adapter uses a universal client and namespaced DB-0 single-key commands, without
multi-key atomicity requirements. `retail-orders` is the application keyspace
subsystem. The application does not expire accepted orders. Redis persistence,
capacity, access control, and backups belong to deployment operations.

Observation receipts expire after 24 hours. Their sorted-set index is advisory,
trimmed on writes, and queried with bounded windows/replies. Observation failures
are logged but cannot undo an accepted order or change its successful response.
Evidence reads retrieve the business outcome separately; rejected attempts do not
inherit a different request's accepted outcome. The feed is not an outbox.

## Runtime and observability

Startup validates compiled provenance and the write token, initializes telemetry,
then constructs the Redis client and context-aware logger. Shutdown drains HTTP
before closing the client and flushing telemetry. There are no business background
workers or outbound application HTTP clients.

HTTP middleware provides the server span and status/latency metrics. Handlers
retain the incoming context and add their observation ID for request correlation.
Outcome logs use INFO for success, WARN for rejection, and ERROR for server
failure. Backend diagnostics use fixed classifications rather than raw Redis
errors, credentials, or payloads. No new automatic redaction layer is installed.
Order and observation IDs are not metric labels. Service identity is consistently
`retail-order-service` across the logger, telemetry, and Kubernetes labels.

## Security, provenance, and deployment

JSON input is bounded and rejects unknown fields. The mutation route requires a
dedicated bearer credential. Local read routes remain unauthenticated, so the
deployment must be network-isolated until an appropriate read-access policy is
provided. The container runs nonroot with a read-only filesystem, dropped
capabilities, and no service-account token. The application exposes neither a
command-execution endpoint nor a source-code endpoint.

Setup copies an explicit source/build-file allowlist into an isolated snapshot.
Published framework modules are resolved from `go.mod` with `GOWORK=off`.
Credentials and Git metadata are not build inputs. The source digest and parent
Git commit are distinct; uncommitted input is never represented as code from that
commit. Runtime configuration cannot override compiled provenance. This mechanism
is not a signed attestation.

This repository owns only `retail-order-service` resources. Its setup script
reuses the sibling framework's existing Kind infrastructure and does not rebuild,
restart, or delete other applications. Compilation, vet, and unit tests run with
`setup.sh build`; `setup.sh smoke` performs separate live API checks. Framework
modules and CI contracts are not modified by this application.
