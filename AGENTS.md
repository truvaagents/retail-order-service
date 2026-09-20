# Coding-assistant instructions

This repository owns a Go retail order HTTP service. It is an ordinary
application, not an agent or tool. Keep catalogue, pricing, and order semantics
here rather than adding application-specific behavior to framework modules.

Read ARCHITECTURE.md and README.md before changes. Always build and deploy through
`./setup.sh`; inspect supported commands with `./setup.sh help`. The sibling
`truva-g3` repository owns shared Kind, Redis, ingress, and observability. Never
recreate or delete that infrastructure here. Preserve unrelated workloads and
source changes. This repository's setup owns only retail-order-service resources.

Run `./setup.sh build` for compilation, vet, and unit tests, then
`./setup.sh smoke` against the deployed application. Verify runtime changes with
real requests, persisted data, and correlated logs/traces. Unit tests are required;
live checks are separate and must not be added to CI. Do not add `-short` to CI.
Framework changes retain the framework's own gates.

Maintain the documented API and storage contracts. Keep request/receipt/log/trace
correlation and never identify uncommitted source as code from its parent Git
commit. Before changing framework integration, review the sibling framework's
core and telemetry architecture documents, FRAMEWORK_DESIGN_PRINCIPLES.md,
docs/observability/DISTRIBUTED_TRACING_GUIDE.md, and
docs/observability/LOGGING_IMPLEMENTATION_GUIDE.md.

Never commit .env, credentials, generated binaries, private operator records, or
live evidence. Keep .env.example current. Never log authentication headers or
backend connection secrets. Documents need explicit human sign-off before commit.
Do not commit or push without authorization.
