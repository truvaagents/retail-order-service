# Retail Order Service

A Go HTTP service for a retail product catalogue, checkout pricing, promotions,
and persistent order creation. Monetary values use integer cents and a currency
code. The service does not process payments or manage fulfilment.

## Local setup

Keep this checkout beside `truva-g3`. Bootstrap the shared Kind infrastructure
through that repository's `examples/travel-chat-agent/setup.sh full-deploy`
before deploying this service. This repository does not install or remove shared
Redis, ingress, or observability components.

```bash
./setup.sh help
cp .env.example .env
# Set any local configuration overrides in .env.
./setup.sh build
./setup.sh deploy
./setup.sh smoke
```

Use Go 1.27+, Node.js, Git, Bash, jq, curl, OpenSSL, kubectl, Kind, and a working
Docker or Podman runtime. The service builds against the published TruvaG3
`core` and `telemetry` v0.5.0 modules with `GOWORK=off`; it has no local module
replacement or dependency on a sibling framework checkout for compilation.

| Command | Purpose |
| --- | --- |
| `build` | Isolated compilation, vet, and unit tests |
| `deploy` | Build and deploy this service into the existing Kind cluster |
| `rebuild` | Deploy with an uncached image build |
| `rollout` | Promote configuration and restart without rebuilding |
| `smoke` | Exercise the real order API, persistence, retries, and validation |
| `status` / `logs` | Inspect this service |

No arguments show help. The script targets `kind-truvag3-demo-$(whoami)` and
namespace `truvag3-examples`, regardless of the current kubectl context.
The application listens on container port 8398 and is exposed at
`http://retail-order-service.localhost`.

## API

| Endpoint | Purpose |
| --- | --- |
| `GET /catalog` | Product names, prices, currencies, and order quantity limits |
| `POST /orders` | Create an order or reconcile a retry |
| `GET /orders/{id}` | Read the stored request, pricing snapshot, and outcome |
| `GET /evidence/{id}` | Read an observation with its separately retrieved order outcome |
| `GET /observations` | Query a bounded, advisory observation feed |
| `GET /build-info` | Read compiled provenance and current pod identity |
| `GET /health`, `GET /ready` | Process health and Redis readiness |

An order request contains an identifier chosen by the caller, a catalogue SKU,
a quantity, and an optional promotion code:

```json
{
  "order_id": "order-2026-0001",
  "sku": "COFFEE-250",
  "quantity": 1
}
```

Prices come from the server catalogue, not the caller. `WELCOME100` is a fixed
100-cent order promotion. Order IDs accept letters, digits, periods, underscores,
colons, and hyphens, begin with a letter or digit, and have at most 128 characters.
The catalogue publishes each product's maximum order quantity.

The API contract is HTTP 201 for a new order and HTTP 200 with
`Idempotency-Replayed: true` for an identical retry. Reusing an accepted order ID
with a different payload must return HTTP 409. Malformed JSON returns HTTP 400;
unsupported products, promotions, or quantities return HTTP 422. A storage failure
returns HTTP 503. A commit may have completed despite a transport failure, so
retry using the same order ID to reconcile it instead of creating another ID.

Each authenticated attempt receives a new observation ID in `X-Request-ID`, even
when the order is replayed. The observation's `revision` identifies the serving
build; `order_revision` identifies the build that priced the accepted order.
Rejected attempts do not inherit an accepted order's ledger outcome.

`GET /observations` requires `start` and `end` in RFC3339 format and `limit` from
1 to 50. A window must be positive, no longer than ten minutes, within the last
24 hours, and end no later than now. `truncated` means more entries are available;
the feed is not a transactional outbox or a completeness guarantee.

## Configuration and access

Standalone Redis in DB 0 is the local default. `.env.example` also shows the
structured Redis Cluster configuration. Choose structured topology or
`REDIS_URL`, never both. Orders use namespaced `retail-orders` keys. Orders have
no application TTL; observation receipts expire after 24 hours and the advisory
index is trimmed on writes. Redis durability and backups are deployment concerns.

Setup generates and preserves `retail-order-service-auth`, containing
`ORDER_API_TOKEN`. `POST /orders` requires that token as a bearer credential.
`setup.sh smoke` reads it internally without printing it. Read endpoints are
unauthenticated in this local deployment: restrict network access and do not use
customer or payment data without an appropriate authorization and storage policy.
The container has no GitHub credentials or Kubernetes service-account token.

## Observability and provenance

JSON outcome logs include operation, status, observation/order IDs, revision,
and request/trace/span correlation. HTTP tracing and metrics use
`retail-order-service`, matching the pod's `app` label. Health/readiness calls are
excluded from tracing. IDs are correlation fields, not metric labels.

`GET /build-info` exposes a SHA-256 manifest digest over root Go sources/tests,
module files, Dockerfile, manifest, and setup script. The build revision combines
that source digest with the published framework module version. `git_revision` identifies
the local parent commit; when `source_matches_commit` is false, the source is not
claimed to be available at that commit. The generated manifest exists only in the
isolated build snapshot; direct `go run .` with the checked-in placeholder fails
startup validation.

This is build provenance, not signed attestation. It does not attest downloaded
dependencies, the toolchain, base image, or Kubernetes desired state. Pod identity
describes the current process, not historical executions. Consumers must compare
recorded revisions before joining an observation to source or deployment data.
