#!/usr/bin/env bash
# Owns only the checkout application; never installs/removes shared infrastructure.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
FRAMEWORK_ROOT="$(dirname "$SCRIPT_DIR")/truva-g3"
CLUSTER_NAME="truvag3-demo-$(whoami)"
KIND_CONTEXT="kind-$CLUSTER_NAME"
NAMESPACE=truvag3-examples
BUILD_CONTEXT_DIR=''

usage() {
    printf '%s\n' 'Retail order service — order API' \
        'Usage: ./setup.sh <help|build|deploy|rebuild|rollout|smoke|status|logs>' \
        'build: isolated source build, vet and unit tests; no cluster required.' \
        'deploy/rebuild: build, load and deploy only retail-order-service.' \
        'rollout: update target configuration and restart; no code build.' \
        'smoke: real order creation, replay, validation and evidence checks.' \
        'No arguments show help. No full-deploy or shared cleanup.'
}

kubectl() { command kubectl --context "$KIND_CONTEXT" "$@"; }

load_config() {
    if [ -f "$SCRIPT_DIR/.env" ]; then set -a; source "$SCRIPT_DIR/.env"; set +a; fi
}

require_cluster() {
    kind get clusters | grep -Fxq "$CLUSTER_NAME" || {
        printf 'Bootstrap the existing %s cluster through the framework first.\n' "$CLUSTER_NAME" >&2; return 1;
    }
    kubectl get namespace "$NAMESPACE" >/dev/null
}

cleanup_snapshot() {
    if [ -n "$BUILD_CONTEXT_DIR" ]; then
        case "$BUILD_CONTEXT_DIR" in
            "${TMPDIR:-/tmp}"/retail-order-build.*) rm -rf -- "$BUILD_CONTEXT_DIR" ;;
            *) printf 'Unexpected snapshot path; leaving it untouched.\n' >&2; return 1 ;;
        esac
        BUILD_CONTEXT_DIR=''
    fi
}

prepare_snapshot() {
    local framework_version core_version telemetry_version git_revision file source_sha revision matches=true
    command -v jq >/dev/null
    core_version="$(cd "$SCRIPT_DIR" && GOWORK=off go list -m -f '{{.Version}}' github.com/truvaagents/truva-g3/core)"
    telemetry_version="$(cd "$SCRIPT_DIR" && GOWORK=off go list -m -f '{{.Version}}' github.com/truvaagents/truva-g3/telemetry)"
    [ "$core_version" = "$telemetry_version" ] || {
        printf 'TruvaG3 core and telemetry versions must match.\n' >&2; return 1;
    }
    framework_version="$core_version"
    [[ "$framework_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
        printf 'TruvaG3 dependencies must use a released semantic version.\n' >&2; return 1;
    }
    git_revision="$(git -C "$SCRIPT_DIR" rev-parse --verify HEAD)"
    BUILD_CONTEXT_DIR="$(mktemp -d "${TMPDIR:-/tmp}/retail-order-build.XXXXXX")"
    trap cleanup_snapshot EXIT
    mkdir -p "$BUILD_CONTEXT_DIR/retail-order-service"
    (
        cd "$SCRIPT_DIR"
        # Explicit root-level source/recipe allowlist: never copy credentials,
        # Git metadata, notes, debug artifacts or a sibling example package.
        local files=(*.go go.mod go.sum Dockerfile.workspace k8-deployment.yaml setup.sh)
        # Comparing only copied files misses deletions: a commit containing an
        # additional .go file is not the same source tree as this snapshot.
        printf '%s\n' "${files[@]}" | LC_ALL=C sort > "$BUILD_CONTEXT_DIR/source-inputs"
        git ls-tree --name-only "$git_revision" | awk '/\.go$/ || /^(go.mod|go.sum|Dockerfile.workspace|k8-deployment.yaml|setup.sh)$/' | LC_ALL=C sort > "$BUILD_CONTEXT_DIR/committed-inputs"
        if ! cmp -s "$BUILD_CONTEXT_DIR/source-inputs" "$BUILD_CONTEXT_DIR/committed-inputs"; then matches=false; fi
        for file in "${files[@]}"; do
            [[ "$file" =~ ^[A-Za-z0-9_.-]+$ ]] && [ -f "$file" ] && [ ! -L "$file" ] || {
                printf 'Unsupported build input: %s\n' "$file" >&2; exit 1;
            }
            cp "$file" "$BUILD_CONTEXT_DIR/retail-order-service/$file"
            if ! git show "$git_revision:$file" > "$BUILD_CONTEXT_DIR/committed-input" 2>/dev/null || \
                ! cmp -s "$BUILD_CONTEXT_DIR/committed-input" "$BUILD_CONTEXT_DIR/retail-order-service/$file"; then
                matches=false
            fi
        done
        cd "$BUILD_CONTEXT_DIR/retail-order-service"
        source_sha="$(printf '%s\n' "${files[@]}" | LC_ALL=C sort | xargs shasum -a 256 | shasum -a 256 | awk '{print $1}')"
        revision="$(printf '%s\n%s\n' "$source_sha" "$framework_version" | shasum -a 256 | awk '{print $1}')"
        jq -n --arg source "$source_sha" --arg git "$git_revision" --arg framework "$framework_version" --arg revision "build-$revision" --argjson matches "$matches" \
            '{schema_version:1,repository:"truvaagents/retail-order-service",revision:$revision,source_sha256:$source,git_revision:$git,source_matches_commit:$matches,framework_version:$framework}' > build-info.json
        printf 'Target build: build-%s\nSource inputs match Git %s: %s\nFramework modules: %s\n' "$revision" "$git_revision" "$matches" "$framework_version"
    )
}

configure() {
    local key value args=()
    if [ -n "${TRUVAG3_REDIS_MODE:-}" ]; then
        [ -z "${REDIS_URL:-}" ] || { printf 'Use REDIS_URL or structured topology, not both.\n' >&2; return 1; }
        args+=(--from-literal=REDIS_URL=)
    fi
    for key in REDIS_URL TRUVAG3_REDIS_MODE TRUVAG3_REDIS_ADDRS TRUVAG3_REDIS_NAMESPACE OTEL_EXPORTER_OTLP_ENDPOINT TRUVAG3_LOG_LEVEL TRUVAG3_LOG_FORMAT; do
        value="${!key:-}"
        [ -z "$value" ] || args+=("--from-literal=$key=$value")
    done
    kubectl create configmap retail-order-service-runtime -n "$NAMESPACE" ${args[@]+"${args[@]}"} --dry-run=client -o yaml | kubectl apply -f -
    args=()
    for key in TRUVAG3_REDIS_USERNAME TRUVAG3_REDIS_PASSWORD; do
        value="${!key:-}"
        [ -z "$value" ] || args+=("--from-literal=$key=$value")
    done
    kubectl create secret generic retail-order-service-redis -n "$NAMESPACE" ${args[@]+"${args[@]}"} --dry-run=client -o yaml | kubectl apply -f -
    if ! kubectl get secret retail-order-service-auth -n "$NAMESPACE" >/dev/null 2>&1; then
        value="$(openssl rand -hex 32)"
        kubectl create secret generic retail-order-service-auth -n "$NAMESPACE" "--from-literal=ORDER_API_TOKEN=$value" --dry-run=client -o yaml | kubectl apply -f -
        unset value
    fi
}

smoke() {
    local token
    command -v node >/dev/null
    token="$(kubectl get secret retail-order-service-auth -n "$NAMESPACE" -o json | jq -r '.data.ORDER_API_TOKEN | @base64d')"
    ORDER_TEST_TOKEN="$token" node "$SCRIPT_DIR/smoke.mjs"
    unset token
}

[ "$#" -le 1 ] || { usage >&2; exit 2; }
case "${1:-help}" in
    help|-h|--help) usage; exit ;;
    build|deploy|rebuild|rollout|smoke|status|logs) verb="$1" ;;
    *) usage >&2; exit 2 ;;
esac
case "$verb" in
    build|deploy|rebuild|rollout) load_config ;;
esac
if [ "$verb" = build ]; then
    prepare_snapshot
    (cd "$BUILD_CONTEXT_DIR/retail-order-service" && GOWORK=off GOFLAGS=-mod=readonly go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./...)
    exit
fi
require_cluster
case "$verb" in
    status) kubectl get deployment,pod,service,ingress -n "$NAMESPACE" -l app.kubernetes.io/part-of=retail-order-service ;;
    logs) kubectl logs -n "$NAMESPACE" deployment/retail-order-service --tail=100 ;;
    smoke) smoke ;;
    deploy|rebuild|rollout)
        if [ "$verb" != rollout ]; then
            # Shared helper requires unset optional variables to expand empty.
            set +u
            source "$FRAMEWORK_ROOT/examples/k8-deployment/setup-env-lib.sh"
            truvag3_check_prerequisites
            prepare_snapshot
            cache_flag=''
            [ "$verb" != rebuild ] || cache_flag=--no-cache
            truvag3_build_docker retail-order-service:latest "$BUILD_CONTEXT_DIR/retail-order-service/Dockerfile.workspace" "$BUILD_CONTEXT_DIR" "$cache_flag"
            truvag3_load_to_kind retail-order-service:latest "$CLUSTER_NAME"
            cleanup_snapshot; trap - EXIT
        fi
        configure
        kubectl apply -f "$SCRIPT_DIR/k8-deployment.yaml"
        kubectl rollout restart deployment/retail-order-service -n "$NAMESPACE"
        kubectl rollout status deployment/retail-order-service -n "$NAMESPACE" --timeout=180s
        printf 'Ready: http://retail-order-service.localhost\n'
        ;;
esac
