# Retail Order Service

A Go HTTP service for a retail product catalogue, checkout pricing, promotions,
and persistent order creation. Monetary values use integer cents and a currency
code. The service does not process payments or manage fulfilment.

[SERVICE_CONTRACT.md](SERVICE_CONTRACT.md) defines the business rules, retry
semantics and execution evidence. It is the reference for functional correctness.
Pricing includes automatic volume adjustments and fixed, percentage and
product-specific promotions.

## Local setup

Keep this checkout beside `truva-g3`. Bootstrap the shared Kind infrastructure
through that repository's `examples/travel-chat-agent/setup.sh full-deploy`
before deploying this service. This repository does not install or remove shared
Redis, ingress, or observability components.

```bash
./setup.sh help
if [ ! -f .env ]; then cp .env.example .env; fi
# Set any local configuration overrides in .env.
./setup.sh build
./setup.sh deploy
./setup.sh smoke
```

Use Go 1.27.2+, Node.js, Git, Bash, jq, curl, OpenSSL, kubectl, Kind, and a working
Docker or Podman runtime. The service builds against the published TruvaG3
`core` and `telemetry` v0.5.0 modules with `GOWORK=off`; it has no local module
replacement or dependency on a sibling framework checkout for compilation.

| Command | Purpose |
| --- | --- |
| `check` | Compilation, vet and JSON unit-test events from a checkout or source snapshot |
| `build` | Isolated compilation, vet, and unit tests |
| `deploy` | Build and deploy this service into the existing Kind cluster |
| `rebuild` | Deploy with an uncached image build |
| `rollout` | Promote configuration and restart without rebuilding |
| `smoke` | Exercise the real order API, persistence, retries, and validation |
| `status` / `logs` | Inspect this service |

No arguments show help. Cluster commands default to `kind-truvag3-demo-$(whoami)`
and namespace `truvag3-examples`, regardless of the current kubectl context. Set
`CLUSTER_NAME` in the environment or `.env` to select another existing Kind
cluster; its context is `kind-$CLUSTER_NAME`. Help, offline builds, and sourcing
repository helpers do not require username lookup or cluster access.
The application listens on container port 8398 and is exposed at
`http://retail-order-service.localhost`.

`./setup.sh check` needs Go, Bash and Git (used by isolated unit-test fixtures).
It requires no `.git` metadata, `.env`, sibling checkout, or running service.
Build output goes to a temporary directory, so source may be mounted read-only.
The Go module cache must contain the pinned dependencies or allow their download;
set `GOCACHE` and `GOMODCACHE` to writable locations when needed. `build` and
`deploy` run this same check before producing an application image. Live API
checks remain separate in `smoke`.

### Test a source snapshot or candidate

Run from the repository or extracted source snapshot root:

```bash
bash ./setup.sh check
```

Capture the exit code, stdout and stderr. Test results are JSON lines from
`go test -json -count=1`; a compilation, vet or test failure returns a nonzero
exit code. HTTP unit tests exercise application handlers in-process with an
in-memory store.

To reproduce an issue, add a regression test derived from the applicable
[service contract](SERVICE_CONTRACT.md) and run `check` against the original
source. Confirm that the test fails at the intended assertion. Apply the proposed
fix, keep the regression test identical, and run `check` again. The regression
and full suite must pass. Tools execute this command through their approved
runner configuration.

### Verify the deployed service

From a configured checkout with the shared Kind infrastructure running:

```bash
./setup.sh deploy
./setup.sh smoke
./setup.sh status
./setup.sh logs
```

`deploy` builds, checks and starts this service. `smoke` sends synthetic requests,
reads saved orders, and checks retries, validation and execution evidence. It
retrieves the write token internally and prints request correlation IDs. Match
those IDs to service logs and traces when investigating runtime behavior. Live
checks run separately from CI.

### Use a fork or another checkout

Keep the checkout directory named `retail-order-service` beside `truva-g3`.
Builds derive their `OWNER/REPOSITORY` identity from the GitHub `origin` remote
(HTTPS, `git@github.com:...`, or `ssh://git@github.com/...`). A fork therefore
reports its own repository instead of upstream. For an offline mirror or an
explicitly chosen identity, configure this in `.env` before building:

```dotenv
ORDER_BUILD_REPOSITORY=YOUR_OWNER/retail-order-service
```

Only a valid GitHub `OWNER/REPOSITORY` value is accepted; empty explicit values,
URLs and credentials fail the build. Without an override, an absent or unsupported
origin also fails. The identity is embedded in the binary. `rollout` or a runtime
environment change cannot rewrite it; use `setup.sh build`/`deploy`/`rebuild`.
The application module path may remain upstream; this repository builds a binary,
not a library consumed by the maintenance agents.

After deployment, inspect `GET /build-info`. Its repository must agree with your
maintenance profiles, and its `git_revision` must exist in that repository.
Commit reviewed source before a provenance-dependent maintenance run:
uncommitted build inputs correctly report `source_matches_commit: false`.
The build identity is evidence, not signed attestation or proof of GitHub access.
Never put credentials in a Git remote URL or the repository identity.

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

Prices come from the server catalogue, not the caller. Volume adjustments apply
before promotions. `WELCOME100`, `SAVE10` and `MUG15` have the conditions in
[the service contract](SERVICE_CONTRACT.md#requests-and-pricing).
Order IDs accept letters, digits, periods, underscores,
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

Observations retain `requested_promotion_code` from each decoded request before
order lookup or validation. A present empty string means no promotion was
submitted; an absent field means unavailable (older observations or undecodable
JSON). This attempted value is separate from the accepted order's immutable
request and pricing snapshot, including on replayed or rejected requests.

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

Authorized local clients can retrieve the generated order token without printing
it. Run this in a trusted Bash session with shell tracing disabled, replacing the
cluster name if necessary:

```bash
set +x
ORDER_API_TOKEN="$(
  set -euo pipefail
  kubectl --context "kind-truvag3-demo-$(whoami)" --namespace truvag3-examples \
    get secret retail-order-service-auth -o json |
    jq -er '.data.ORDER_API_TOKEN | select(type == "string" and length > 0) | @base64d'
)"
export ORDER_API_TOKEN
# Use the credential in your local client without echoing it or embedding it in URLs.
# Remove it when finished:
unset ORDER_API_TOKEN
```

### Sharing the source safely

Making this GitHub repository public is separate from exposing its running HTTP
service. Keep the local ingress, Redis and unauthenticated read endpoints off the
public Internet. Use synthetic orders only; this service is not a payment system.

Before changing repository visibility, review all branches and Git history for
credentials, private operator data and generated evidence, plus GitHub PR content,
Actions logs and release assets. Keep `.env`, tokens and live captures untracked;
the `.gitignore` and isolated build allowlist are safeguards, not proof that
history is clean. Run the prescribed build/tests and `govulncheck` using the
patched Go toolchain, and choose an explicit source license before distribution.
Repository visibility, licensing, branch permissions and any publication action
remain maintainer decisions. A fork's credentials and full maintenance workflow
must be verified independently.

## Observability and provenance

JSON outcome logs include operation, status, observation/order IDs, revision,
and request/trace/span correlation. HTTP tracing and metrics use
`retail-order-service`, matching the pod's `app` label. Health/readiness calls are
excluded from tracing. IDs are correlation fields, not metric labels.

`GET /build-info` exposes a SHA-256 manifest digest over root Go sources/tests,
module files, Dockerfile, manifest, setup script, service contract and smoke
recipe. The build revision combines
that source digest with the published framework module version. `git_revision` identifies
the local parent commit; when `source_matches_commit` is false, the source is not
claimed to be available at that commit. The generated manifest exists only in the
isolated build snapshot; direct `go run .` with the checked-in placeholder fails
startup validation.

This is build provenance, not signed attestation. It does not attest downloaded
dependencies, the toolchain, base image, or Kubernetes desired state. Pod identity
describes the current process, not historical executions. Consumers must compare
recorded revisions before joining an observation to source or deployment data.
