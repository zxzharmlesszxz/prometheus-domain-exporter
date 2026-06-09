# prometheus-domain-exporter

`prometheus-domain-exporter` exposes domain state as Prometheus metrics.

It is built as a thin exporter on top of `prometheus-exporter-framework`.

## Local Run

```bash
make build
./dist/prometheus-domain-exporter \
  --web.listen-address=:9853 \
  --domain.config-file=examples/prometheus-domain-exporter.yml \
  --domain.target=example.com
```

Useful flags:

```bash
--domain.config-file
--domain.target
--domain.refresh-interval
--domain.timeout
--domain.max-concurrent-targets
--web.listen-address
--web.telemetry-path
--web.enable-pprof
--log.level
--log.format
```

By default, the exporter listens on `:9853` and refreshes data every `1m`.
The Docker Compose setup passes `--domain.config-file=/etc/prometheus/prometheus-domain-exporter.yml` explicitly. If no `--domain.config-file` flag is provided, defaults and CLI flags are used; a config file is not required.
The generated `examples/prometheus-domain-exporter.yml` file lists every supported domain config key with its default value.
Make, Compose, and smoke defaults use `FEATURE_CONFIG_FILE`, which defaults to `prometheus-domain-exporter.yml`, and pass that path explicitly with `--domain.config-file=...`.
Runtime config can always be overridden with another `--domain.config-file=...` value.
Configure one or more domains with repeatable `--domain.target` flags. Data refresh runs through the framework snapshot collector in a background worker; scrapes return the last collected snapshot.

## Metrics

Example output:

```code
domain_configured_domains 1
domain_registration_lookup_success{domain="example.com"} 1
domain_registration_lookup_timestamp_seconds{domain="example.com"} 1742812800
domain_registration_expiration_timestamp_seconds{domain="example.com"} 1893456000
domain_registration_expiration_remaining_seconds{domain="example.com"} 150643200
domain_exporter_last_collection_success 1
domain_exporter_last_collection_timestamp_seconds 1742812800
domain_exporter_last_successful_collection_timestamp_seconds 1742812800
```

The full metric contract lives in [`METRICS.md`](METRICS.md).

## Docker Compose

The repository includes [`docker-compose.yml`](docker-compose.yml) for local testing.
The Prometheus scrape config is embedded in Compose, while alerting rules live
under [`examples/prometheus`](examples/prometheus).
It starts:

- `exporter`
- `prometheus`
- `grafana`

```bash
make compose
```

Endpoints:

- `http://localhost:9853`
- `http://localhost:9853/metrics`
- `http://localhost:9853/healthz`
- `http://localhost:9090`
- `http://localhost:3000`

## Grafana

Docker Compose provisions Grafana with:

- Prometheus datasource `DS_PROMETHEUS`
- dashboards from [`examples/grafana`](examples/grafana)
- default login `admin` / `admin`

Open `http://localhost:3000` after `make compose`.

For a direct Docker build, run:

```bash
make docker-build
```

## Tests

```bash
make go-check
```

The repository includes the same maintenance target layout used by the concrete exporter repos:

```bash
make help
make go-check
make check
make docker-smoke
make full-check
```

`make go-check` runs Go-only checks. `make check` also validates the Prometheus and Docker Compose examples, so it requires Docker.

## Scaffold-Owned Go Files

Go files named `scaffold_*.go` are generated contract glue and should stay
identical to the scaffold output. Add exporter-specific behavior in adjacent
non-scaffold files such as `feature_config_ext.go`, `feature_metrics_ext.go`,
`feature_snapshotter_ext.go`, `feature_smoke_ext.go`, `metrics.go`, and the
domain check package. The feature package `Snapshot` alias lives in
`scaffold_snapshot_types.go`; the actual snapshot structure lives in
`internal/domaincheck`.

Build local release artifacts:

```bash
make build VERSION=v0.1.0
make release VERSION=v0.1.0
make release-smoke VERSION=v0.1.0
```

Build and push a Docker image:

```bash
make docker-build VERSION=v0.1.0 DOCKER_IMAGE=prometheus-domain-exporter:v0.1.0
make docker-push DOCKER_IMAGE=prometheus-domain-exporter:v0.1.0
make docker-buildx-push VERSION=v0.1.0 DOCKER_IMAGE=registry.example.com/prometheus-domain-exporter:v0.1.0
```

## Architecture

The high-level design is documented in [`ARCHITECTURE.md`](ARCHITECTURE.md).

## License

This project is licensed under the MIT License. See [`LICENSE`](LICENSE).
