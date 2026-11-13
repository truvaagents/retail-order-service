# Retail Order Service Contract

This document defines the externally observable behavior of the order service.
README.md describes setup; ARCHITECTURE.md describes implementation. Contract
changes require maintainer review. Consumers should reference this file at an
approved immutable Git revision when assessing historical executions.

## Scope

The service accepts single-product retail orders, calculates volume and
promotional discounts, and saves an immutable accepted order. It does not manage
customers, inventory, tax, shipping, payment, or fulfilment. All prices and
adjustments are integer cents in the product's currency; the current catalogue
uses USD. The catalogue and pricing policy are fixed for a serving build.

## Requests and pricing

| ID | Requirement |
| --- | --- |
| ORD-01 | A request contains `order_id`, `sku`, integer `quantity`, and an optional `promotion_code`. Unknown JSON fields, trailing JSON values and bodies over 16 KiB are rejected with HTTP 400. |
| ORD-02 | Order IDs are case-sensitive, start with a letter or digit, contain only letters, digits, `.`, `_`, `:`, and `-`, and are at most 128 characters. Invalid order IDs return HTTP 422 with `invalid_order`. |
| ORD-03 | A new order uses the exact, case-sensitive SKU's price, currency and maximum quantity from `GET /catalog`. An unknown SKU returns HTTP 422 with `unknown_product`. Quantity must be between 1 and that maximum, inclusive; otherwise return HTTP 422 with `invalid_order`. |
| ORD-04 | Subtotal is `unit_price_cents × quantity`. Apply one automatic volume adjustment to the entire subtotal: 5% for quantities 10–24, 10% for quantities 25 and above, and none below 10. |
| ORD-05 | Apply at most one promotion after the volume adjustment. Percentage adjustments round to the nearest integer cent, with half a cent rounded up, at each adjustment step. Promotion minimum spend is evaluated against the subtotal remaining after the volume adjustment. |
| ORD-06 | `discount_cents` is the combined volume and promotion adjustment for the whole order. A promotion cap limits only the promotional part. Total discount cannot exceed subtotal; the persisted total is `subtotal - discount_cents`. |

Promotion codes are case-sensitive. An absent or empty code means no promotion.
Unknown codes return HTTP 422 with `unknown_promotion`; a known code whose
conditions are unmet returns HTTP 422 with `promotion_not_eligible`.

| Code | Adjustment | Conditions |
| --- | --- | --- |
| `WELCOME100` | 100 cents once per order | Any catalogue SKU |
| `SAVE10` | 10% of the remaining subtotal, capped at 1,000 cents | Remaining subtotal at least 5,000 cents |
| `MUG15` | 15% of the remaining subtotal | SKU starts with `MUG-` |

For example, three coffees at 1,000 cents with `WELCOME100` total 2,900 cents.
Two mugs at 2,500 cents with `MUG15` total 4,250 cents.

## Acceptance, retries and failures

| ID | Requirement |
| --- | --- |
| ORD-07 | First acceptance returns HTTP 201 and saves the decoded request, its identity digest, pricing snapshot and ledger together. `GET /orders/{id}` retrieves that immutable record. Accepted orders have no application TTL. |
| ORD-08 | An identical retry returns HTTP 200 with `Idempotency-Replayed: true`, using the accepted pricing and outcome even after a newer build changes policy or catalogue. Request identity includes order ID, SKU, quantity and promotion code; absent and empty promotion are equivalent. Changing any identity field while retaining an accepted order ID returns HTTP 409 with `order_id_conflict`. Concurrent identical requests create one accepted record. |
| ORD-09 | Decode and validate the order ID first. For an accepted ID, reconcile identity before evaluating current catalogue or pricing rules. For new IDs, validate SKU, quantity, then promotion. |
| ORD-10 | Storage failures return HTTP 503. `order_commit_unconfirmed` means the write's result is uncertain; the caller reconciles by retrying the same order ID and request. An HTTP failure does not prove rollback. Observation failures do not undo acceptance or change an otherwise successful response. |

`ledger.writes` is the accepted record's logical write marker. It is not an
independent database command counter. Stored orders predating a pricing change
retain their original values and are not recalculated.

## Execution evidence

| ID | Requirement |
| --- | --- |
| OBS-01 | Every authenticated submission receives a new observation ID in `X-Request-ID`, including replays and rejections. Its `operation` is `create_order`. `GET /evidence/{observation_id}` reads the saved attempt. |
| OBS-02 | An observation records attempted SKU, quantity and decoded `requested_promotion_code`. The promotion field is absent when input was undecodable or an older observation lacks it; a present empty string means no promotion. Attempted values remain separate from the accepted request returned by `GET /orders/{order_id}`. |
| OBS-03 | `revision` identifies the serving build, `target_trace_id` correlates its trace, and the observation ID correlates request logs. `order_revision` identifies the original accepted order's build and may differ on replay. A successful attempt's `order_id` identifies its persisted outcome; rejected attempts do not inherit another request's outcome. Missing outcome fields mean unavailable, not zero. |
| OBS-04 | Observation receipts expire after 24 hours. `GET /observations` requires exactly one `start`, `end` and `limit`. Times are RFC3339; the positive window is at most ten minutes, starts within the last 24 hours, and ends no later than server time. Limit is 1–50. `truncated` reports a bounded reply; the feed is advisory and does not guarantee complete capture. |

Use `GET /build-info` to map a serving revision to source. Compare revisions
before applying a current catalogue to an older observation. A false
`source_matches_commit` means the build contains source not represented by its
parent commit. Provenance is not signed attestation.

## Access and availability

`POST /orders` requires the dedicated bearer credential and returns HTTP 401
when unauthorized. Read endpoints are unauthenticated in the local deployment
and require network isolation. Health checks use `GET /health`; `GET /ready`
checks Redis and returns HTTP 503 when unavailable. Unknown order or observation
IDs return HTTP 404; failed backend reads return HTTP 503.
