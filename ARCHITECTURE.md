# Architecture

`prometheus-domain-exporter` is a thin concrete exporter built on `prometheus-exporter-framework`.

## Package Layout

- `cmd`
  Minimal process entrypoint. The generated entrypoint file is
  `scaffold_main.go` and should stay scaffold-owned.
- `internal/exporter`
  Thin adapter that asks the feature package for a contract-backed feature and
  delegates bootstrap metadata to the framework. Files named `scaffold_*.go`
  are fully scaffold-owned.
- `internal/domain`
  Concrete feature package. `scaffold_feature.go` owns the scaffold-compatible
  `featurekit.SnapshotFeatureExtension` assembly and wires config-file flags,
  feature config flag specs, runtime config, collector construction, metrics,
  snapshot status, and smoke behavior through domain-specific hooks.
  `snapshot_types.go` owns the feature aggregate `Snapshot`, which combines the
  `internal/domaincheck` result with RDAP and WHOIS source-health data.
  Domain-specific defaults and hook functions live in adjacent feature files:
  `feature_config_ext.go`, `feature_metrics_ext.go`,
  `feature_snapshotter_ext.go`, `feature_smoke_ext.go`, and `metrics.go`.
- `internal/domaincheck`
  Domain check engine: domain-name normalization, RDAP and WHOIS service
  discovery/lookups, check result types, and the snapshot-backed `Checker`.
- `smoke`
  Binary smoke tests that build the real executable and verify CLI, HTTP, and
  metric behavior. The scaffold-owned smoke test is `scaffold_binary_test.go`.

Concrete exporter logic belongs in non-`scaffold_*.go` files. Treat
`scaffold_*.go` files as generated contract glue and update them through the
scaffold sync flow only.

## Data Flow

1. `cmd/scaffold_main.go` delegates to `internal/exporter.Main()`, which runs `framework.MainFromInjectedProject(...)`.
2. `internal/exporter` creates the concrete feature through
   `internal/domain.NewFeature(...)` and framework-injected feature metadata.
3. Framework `featurekit.Feature` registers common flags such as `--domain.refresh-interval` and `--domain.config-file`, then delegates `--domain.target`, `--domain.timeout`, and `--domain.max-concurrent-targets` through the framework-owned feature contract.
4. Framework `featurekit.Feature` builds a typed snapshotter and collector from the extension-backed spec, then registers and starts the collector.
5. The feature snapshotter delegates to `domaincheck.Checker`, which prefers
   RDAP and falls back to registry WHOIS when the TLD does not publish RDAP.
6. The feature snapshotter wraps the domain snapshot with independent RDAP and
   WHOIS source-health state and error counters. Both sources report the
   duration of the complete domain refresh represented by the snapshot.
7. `framework.SnapshotCollector` refreshes data in a background worker every `--domain.refresh-interval`; scrapes read the latest completed snapshot.
8. The collector exports per-domain registration metrics, including the RDAP or
   WHOIS source used by each lookup, registration source-health metrics, and
   framework collection health metrics.

## Failure Semantics

If no domains are configured, the exporter exposes collection health metrics but no per-domain registration metrics.

If any configured domain lookup fails, the exporter exposes per-domain lookup success metrics and sets:

- `domain_exporter_last_collection_success = 0`
- the failing source's `domain_<source>_up = 0`

If a registration lookup completes but does not verify the domain or return an
expiration timestamp, the exporter sets:

- `domain_exporter_last_collection_success = 0`
- the affected source's `domain_<source>_valid = 0`

WHOIS fallback failures and invalid responses follow the same rules through
`domain_whois_up` and `domain_whois_valid`. Source validity is independent: a
WHOIS failure does not mark RDAP invalid, while full collection success still
requires every configured domain to succeed.

Successful domain results are cached independently in the framework
`featurekit.TTLCache`. At each successful external lookup, the entry TTL is 24
hours when expiration is more than 30 days away, 6 hours when it is more than 7
and at most 30 days away, and 1 hour when it is 7 days away or closer. Failed or
incomplete results bypass the cache so the next framework refresh retries them.
Cached results preserve the success, verification, timestamp, and source of the
last external lookup. The cache is local to the process and is empty after a
restart. Framework cache statistics are exposed under `domain_cache_*` with
`cache="registration"`.

The checker separately keeps the last confirmed expiration in
`featurekit.LastKnownGood`. Temporary transport, service, and parse failures
preserve that expiration while current lookup and collection health report the
failure. The data becomes stale after 24 hours, and an authoritative not-found
response removes it. This state is process-local and starts empty after restart.

Each WHOIS TCP query is attempted at most twice within the configured
per-domain timeout. Context cancellation prevents further attempts.

The `/healthz` endpoint remains `200 OK` while the process is alive even if the latest collection failed.

## Metric Namespaces

- Domain registration metrics use the feature namespace `domain`, for example
  `domain_registration_lookup_success`.
- RDAP and WHOIS source-health metrics also use the feature namespace, for
  example `domain_rdap_up` and `domain_whois_up`.
- Framework-owned exporter metrics use the metric namespace `domain_exporter`,
  for example `domain_exporter_last_collection_success` and
  `domain_exporter_collection_duration_seconds`.

## Dashboard Shape

The bundled Grafana dashboard uses the Grafana v2 dashboard resource model. The
Overview tab contains:

- `Status`: exporter availability and collection age.
- `Main Metrics`: domain status stats and a compact domain status table.
- `Domain Details`: collapsed lookup health, registration expiry, earliest
  expiry, and latest external lookup age views.
- `Source Health`: collapsed shared registration source-health graphs powered
  by `domain_rdap_*` and `domain_whois_*` metrics.
- `Historical Graph`: collapsed change graphs for lookup and expiry series.
- `Exporter Collection`: collapsed framework collection metrics.

The Inventory tab contains a full-page domain status table with lookup source
and expiry details. The Runtime and Scrape tabs contain Go/process runtime
panels and Prometheus-side scrape health.

## Alerting

The repository ships equivalent Prometheus-managed and Grafana-managed alert
rules. Prometheus rules remain under `examples/prometheus`; Grafana file
provisioning reads the independent Grafana-managed copies from
`examples/grafana/alerting`. Both sets preserve rule names, pending durations,
severities, summaries, and descriptions. Grafana rules use `DS_PROMETHEUS` for
queries and add `service="prometheus-domain-exporter"` plus
`rule_source="grafana"` for notification-policy routing. Contact points and
notification policies belong to the deployment and are not stored here.

Prometheus and Grafana are independent evaluators. The local Prometheus
configuration evaluates every 15 seconds, while the Grafana rule group evaluates
every 30 seconds; each system owns its own pending and firing state. The
`examples/grafana/alerting_test.go` contract test compares rule names,
expressions, pending durations, severities, and annotations so changes to the
Prometheus rules cannot silently leave the Grafana copies behind.
