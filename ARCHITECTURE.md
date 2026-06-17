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
  `internal/domaincheck` result with RDAP source-health data.
  Domain-specific defaults and hook functions live in adjacent feature files:
  `feature_config_ext.go`, `feature_metrics_ext.go`,
  `feature_snapshotter_ext.go`, `feature_smoke_ext.go`, and `metrics.go`.
- `internal/domaincheck`
  Domain check engine: domain-name normalization, RDAP bootstrap/lookup,
  check result types, and the snapshot-backed `Checker`.
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
3. Framework `featurekit.Feature` registers common flags such as `--domain.refresh-interval` and `--domain.config-file`, then delegates `--domain.target` plus `--domain.timeout` through the framework-owned feature contract.
4. Framework `featurekit.Feature` builds a typed snapshotter and collector from the extension-backed spec, then registers and starts the collector.
5. The feature snapshotter delegates to `domaincheck.Checker`, which uses RDAP lookup code to resolve each domain's registration expiration time.
6. The feature snapshotter wraps the domain snapshot with aggregate RDAP
   source-health state such as `domain_rdap_up`,
   `domain_rdap_valid`, source error counters, and refresh duration.
7. `framework.SnapshotCollector` refreshes data in a background worker every `--domain.refresh-interval`; scrapes read the latest completed snapshot.
8. The collector exports per-domain registration metrics, RDAP source-health
   metrics, and framework collection health metrics.

## Failure Semantics

If no domains are configured, the exporter exposes collection health metrics but no per-domain registration metrics.

If any configured domain lookup fails, the exporter exposes per-domain lookup success metrics and sets:

- `domain_exporter_last_collection_success = 0`
- `domain_rdap_up = 0`

If a domain lookup completes but RDAP does not verify the domain or does not
return an expiration timestamp, the exporter sets:

- `domain_exporter_last_collection_success = 0`
- `domain_rdap_valid = 0`

The `/healthz` endpoint remains `200 OK` while the process is alive even if the latest collection failed.

## Metric Namespaces

- Domain registration metrics use the feature namespace `domain`, for example
  `domain_registration_lookup_success`.
- RDAP source-health metrics also use the feature namespace, for example
  `domain_rdap_up`.
- Framework-owned exporter metrics use the metric namespace `domain_exporter`,
  for example `domain_exporter_last_collection_success` and
  `domain_exporter_collection_duration_seconds`.

## Dashboard Shape

The bundled Grafana dashboard uses the Grafana v2 dashboard resource model. The
Overview tab contains:

- `Status`: exporter availability and collection age.
- `Main Metrics`: domain status stats, bad-domain table, timing snapshot table,
  lookup health, and registration expiry views.
- `Source Health`: shared RDAP source-health graphs powered by
  `domain_rdap_*` metrics.
- `Historical Graph`: collapsed change graphs for lookup and expiry series.
- `Exporter Collection`: collapsed framework collection metrics.

The Runtime and Scrape tabs contain Go/process runtime panels and
Prometheus-side scrape health.
