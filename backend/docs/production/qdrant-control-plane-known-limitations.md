# Qdrant Control Plane Known Remaining Limitations

These are known operational risks after Phase 2.6 and are intentionally not redesigned in this phase.

## 1) In-memory rate limit scope
- Current limiter is process-local.
- In multi-instance deployments, limits are not globally synchronized.
- Action: if strict global limits are required, move to shared limiter storage in a future phase.

## 2) Qdrant outage / partial availability
- Control-plane operations may return `qdrant_error` during outage windows.
- Retry/backoff exists at client level, but prolonged outages still fail requests.
- Action: rely on alerts/runbooks and incident policy; no control-plane redesign in this phase.

## 3) Long-running scan edge cases
- `vector_health` is bounded with duration/vector/chunk limits, so large datasets may require multiple scans.
- Action: use iterative bounded scans and trend metrics instead of single full scans under load.

## 4) Repair backlog risk
- If missing/orphan vectors grow faster than repair throughput, backlog can persist.
- Action: monitor missing/orphan counters and reindex trigger rates; tune operational playbooks.

## 5) Alert threshold tuning
- Proposed thresholds are practical starting points, but environment-specific tuning is expected.
- Action: tune based on normal baseline traffic and incident history after deployment.

## 6) Metrics dependency assumptions
- Some alerts rely on route-level `http_requests_total` labels (`path`, `status`).
- If those labels differ in your Prometheus pipeline, update the rule selectors accordingly.
