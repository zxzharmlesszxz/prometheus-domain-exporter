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
  `scaffold_snapshot_types.go` owns the scaffold-managed `Snapshot` alias from
  the feature package to `internal/domaincheck.Snapshot`.
  Domain-specific defaults and hook functions live in adjacent feature files:
  `feature_config_ext.go`, `feature_metrics_ext.go`,
  `feature_snapshotter_ext.go`, and `feature_smoke_ext.go`.
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
3. Framework `featurekit.Feature` registers common flags such as `--domain.refresh-interval` and `--domain.config-file`, then delegates `--domain.domain` plus `--domain.lookup-timeout` through the framework-owned feature contract.
4. Framework `featurekit.Feature` builds a typed snapshotter and collector from the extension-backed spec, then registers and starts the collector.
5. The feature snapshotter delegates to `domaincheck.Checker`, which uses RDAP lookup code to resolve each domain's registration expiration time.
6. `framework.SnapshotCollector` refreshes data in a background worker every `--domain.refresh-interval`; scrapes read the latest completed snapshot.
7. The collector exports per-domain registration metrics and collection health metrics.

## Failure Semantics

If no domains are configured, the exporter exposes collection health metrics but no per-domain registration metrics.

If any configured domain lookup fails, the exporter exposes per-domain lookup success metrics and sets:

- `domain_exporter_last_collection_success = 0`

The `/healthz` endpoint remains `200 OK` while the process is alive even if the latest collection failed.
