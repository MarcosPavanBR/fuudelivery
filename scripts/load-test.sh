#!/bin/bash
# ============================================================
# FuuDelivery — Load Test Script
# Usage: bash scripts/load-test.sh [api_url] [requests] [concurrency]
# Default: 100 requests, 10 concurrent
# ============================================================

API_URL="${1:-https://fuudelivery-api-8y6l.onrender.com}"
REQUESTS="${2:-100}"
CONCURRENCY="${3:-10}"

echo ""
echo "╔══════════════════════════════════════════════╗"
echo "║  FuuDelivery — Load Test                    ║"
echo "╚══════════════════════════════════════════════╝"
echo ""
echo "Target:     $API_URL"
echo "Requests:   $REQUESTS"
echo "Concurrent: $CONCURRENCY"
echo ""

# Check dependencies
if ! command -v curl &>/dev/null; then
  echo "ERROR: curl not found"
  exit 1
fi

echo "── Testing /health endpoint ──"

# Run load test with curl
total_time=0
success=0
failed=0
start_ts=$(date +%s%N)

for i in $(seq 1 $REQUESTS); do
  http_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "$API_URL/health" 2>/dev/null)
  if [ "$http_code" = "200" ]; then
    success=$((success + 1))
  else
    failed=$((failed + 1))
  fi
  # Print progress every 10 requests
  if [ $((i % 10)) -eq 0 ]; then
    printf "\r  Progress: %d/%d requests" $i $REQUESTS
  fi
done

end_ts=$(date +%s%N)
ms_duration=$(( (end_ts - start_ts) / 1000000 ))

# Calculate RPS
if [ $ms_duration -gt 0 ]; then
  rps=$(( (REQUESTS * 1000) / ms_duration ))
else
  rps="N/A"
fi

echo ""
echo ""
echo "── Results ──"
echo "  Total requests:  $REQUESTS"
echo "  Successful:      $success"
echo "  Failed:          $failed"
echo "  Duration:        ${ms_duration}ms"
echo "  Requests/sec:    $rps"
echo "  Success rate:    $(( (success * 100) / REQUESTS ))%"
echo ""

if [ $failed -gt 0 ]; then
  echo "⚠️  $failed requests failed — check if service is under pressure"
else
  echo "✅ All requests succeeded"
fi

echo ""
echo "── Testing /health payment service ──"
PAY_URL="${PAY_URL:-https://fuudelivery-payment.onrender.com}"
pay_success=0
pay_failed=0

for i in $(seq 1 20); do
  http_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "$PAY_URL/health" 2>/dev/null)
  if [ "$http_code" = "200" ]; then
    pay_success=$((pay_success + 1))
  else
    pay_failed=$((pay_failed + 1))
  fi
done

echo "  Payment Service: $pay_success/20 successful"
if [ $pay_failed -gt 0 ]; then
  echo "⚠️  $pay_failed payment health checks failed"
else
  echo "✅ Payment Service healthy"
fi
echo ""
