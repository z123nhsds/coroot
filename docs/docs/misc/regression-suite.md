---
sidebar_position: 2
---

# Cross-module regression suite

This regression suite covers three cross-module scenarios added for Coroot 2.7.1 maintenance work.
The goal is to protect the integration points that were most likely to regress after Go vet-driven syntax cleanups,
MCP telemetry updates, deployment tracking changes, and utility-layer fixes.

## Coverage matrix

### 1. RCA summary stays linked to SLO incident notifications

Files:

- `notifications/notifications_test.go`

What it validates:

- An SLO incident can still carry an RCA short summary and remediation text into the incident notification payload.
- Supporting incident reports are preserved without duplicating the SLO check in the payload.
- The webhook JSON generated for the incident remains stable for downstream integrations.

Why it matters:

- SLO incidents are derived from Prometheus-backed burn-rate evaluation.
- Notification formatting is a common place for regressions after syntax cleanup or template refactoring.

### 2. MCP log workflows remain stable when deployment tracking detects memory growth

Files:

- `watchers/deployments_test.go`
- `rbac/rbac_test.go`
- `stats/stats_test.go`

What it validates:

- Deployment snapshots still record `MemoryLeakPercent` from `auditor.MemoryGrowthPct`.
- Log counters and log-pattern similarity sets stay intact while deployment tracking runs.
- Project-scoped MCP log access remains guarded by RBAC.
- MCP tool invocations such as `query_logs` continue to be counted by usage statistics and are reset after collection.

Why it matters:

- This path spans deployment watchers, log clustering, RBAC authorization, and MCP usage accounting.
- A regression in any of these layers can silently break log-centric investigations.

### 3. Utility formatting changes do not break the Prometheus health warning path

Files:

- `utils/format_test.go`
- `api/ctx_test.go`

What it validates:

- `utils.FormatDuration` keeps producing stable human-readable lag strings.
- The Prometheus status warning rendered by the API still surfaces lag with the expected formatted message.

Why it matters:

- The Prometheus health/status path is user-facing and sensitive to utility-layer regressions.
- Keeping the formatting contract stable prevents broken health messaging during cache lag or restart windows.

## Running the suite

Run the focused regression packages locally:

```bash
go test ./notifications ./watchers ./rbac ./stats ./api ./utils
```

Run the repository CI baseline:

```bash
go test ./...
go vet ./...
```

## CI and workspace compatibility

- The new coverage is implemented as Go unit tests only, so it fits the existing GitHub Actions workflow without changing the pipeline shape.
- No frontend build inputs, Vue sources, or Docusaurus runtime code paths were modified.
- No Nx, Playwright, or Knip configuration files exist in this repository at the time of writing, and this suite does not introduce any dependency on them.
- Because the additions are package-local tests and one documentation page, they remain cache-friendly for Go test execution and do not affect frontend dependency graphs.
