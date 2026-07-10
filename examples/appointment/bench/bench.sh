#!/usr/bin/env bash
# Benchmark feast's in-process JSX->PDF render under the SAME container resource
# profiles a Gotenberg docx->PDF sidecar is typically sized under: 0.25 / 0.5
# vCPU, capped memory, swap disabled. It cross-compiles a static linux binary,
# stages it with the appointment letter + assets, and runs it in a minimal
# container for each profile — reporting the same latency distribution,
# throughput and peak memory so the two engines can be compared apples-to-apples.
#
#   bash examples/appointment/bench/bench.sh
#   REQUESTS=50 bash examples/appointment/bench/bench.sh
#
# Requires Docker + Go. Run from anywhere.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
APPT="$ROOT/examples/appointment"
STAGE="$(mktemp -d)"
REQUESTS="${REQUESTS:-30}"
IMAGE="debian:bookworm-slim"

cleanup() { rm -rf "$STAGE"; }
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "Docker is not available/running. Start Docker and retry." >&2
  exit 1
fi

ARCH="$(docker version --format '{{.Server.Arch}}' 2>/dev/null || echo arm64)"
echo "Building feast-bench (linux/$ARCH, CGO_ENABLED=0, static) ..."
( cd "$ROOT" && GOTOOLCHAIN=local GOOS=linux GOARCH="$ARCH" CGO_ENABLED=0 \
    go build -trimpath -o "$STAGE/feast-bench" ./examples/appointment/bench ) || exit 1
cp "$APPT/appointment.jsx" "$STAGE/"
cp -R "$APPT/assets" "$STAGE/assets"
docker pull "$IMAGE" >/dev/null 2>&1 || true

# name | cpus | memory | concurrency  (swap disabled: --memory-swap == --memory)
PROFILES=(
  "gotenberg-floor|0.25|384m|1"
  "higher-cpu|0.5|512m|1"
)

for p in "${PROFILES[@]}"; do
  IFS='|' read -r pname cpus mem conc <<<"$p"
  echo
  echo "==================================================================="
  echo "PROFILE: $pname   cpus=$cpus  mem=$mem  concurrency=$conc  GOMAXPROCS=1  (swap off)"
  echo "==================================================================="
  cid=$(docker run -d --cpus="$cpus" --memory="$mem" --memory-swap="$mem" \
    --restart=no -e REQUESTS="$REQUESTS" -e CONCURRENCY="$conc" -e GOMAXPROCS=1 \
    -v "$STAGE:/app:ro" -w /app "$IMAGE" /app/feast-bench)
  docker wait "$cid" >/dev/null
  docker logs "$cid" 2>&1
  echo "-------------------------------------------------------------------"
  echo "OOMKilled: $(docker inspect -f '{{.State.OOMKilled}}' "$cid" 2>/dev/null)   ExitCode: $(docker inspect -f '{{.State.ExitCode}}' "$cid" 2>/dev/null)"
  docker rm -f "$cid" >/dev/null 2>&1
done

echo
echo "For reference, a warm Gotenberg (gotenberg/gotenberg:8, LibreOffice) at"
echo "0.25 vCPU converting the same class of document measures roughly p50 ~2s /"
echo "p95 ~7s, peak RSS ~200-260 MiB, throughput ~1/latency (conversions"
echo "serialize on one LibreOffice process). See bench/RESULTS.md."
