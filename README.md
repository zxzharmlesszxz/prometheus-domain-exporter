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
--web.config.file
--web.telemetry-path
--web.enable-pprof
--log.level
--log.format
--version
```

By default, the exporter listens on `:9853` and refreshes data every `1h`.
The Docker Compose setup passes `--domain.config-file=/etc/prometheus/prometheus-domain-exporter.yml` explicitly. If the flag is omitted, the exporter loads that default path when it exists; otherwise it uses defaults and CLI flags, so a config file is not required.
The generated `examples/prometheus-domain-exporter.yml` file lists every supported domain config key with its default value.
Make, Compose, and smoke defaults use `FEATURE_CONFIG_FILE`, which defaults to `prometheus-domain-exporter.yml`, and pass that path explicitly with `--domain.config-file=...`.
Runtime config can always be overridden with another `--domain.config-file=...` value.
Configure one or more domains with repeatable `--domain.target` flags. Data refresh runs through the framework snapshot collector in a background worker; scrapes return the last collected snapshot.
Successful registration lookups are cached per domain with the framework TTL
cache. At each successful external lookup, domains expiring in more than 30 days
receive a 24-hour TTL, domains expiring in more than 7 and at most 30 days
receive a 6-hour TTL, and domains expiring in 7 days or less receive a 1-hour
TTL. The cache is in memory and is cleared when the exporter restarts. Failed or
incomplete lookups are not cached and are retried on the next background refresh.

The exporter also keeps the last confirmed registration expiration per domain.
A temporary RDAP or WHOIS failure leaves the expiration metrics available while
the current lookup and collection health metrics report the failure. The
last-known-good metrics expose its last-success timestamp, availability,
staleness, and consecutive refresh failures; age is derived from the timestamp.
Data becomes stale 24 hours after its last successful external lookup. An
authoritative not-found response clears the saved registration data.

## Configuration Example

The YAML config file accepts these domain-specific keys:

```yaml
targets:
  - example.com
  - example.net
  - example.ws
timeout: 10s
max_concurrent_targets: 8
```

`targets` may be empty when the exporter should start with framework/runtime
metrics only:

```yaml
targets: []
timeout: 10s
max_concurrent_targets: 8
```

Registration lookup prefers RDAP and falls back to registry WHOIS when a TLD
does not publish an RDAP service. The exporter supplements the IANA RDAP
bootstrap for `.io`; for TLDs such as `.ws`, it discovers the registry WHOIS
server through IANA and reads the expiration date over the standard TCP port 43
protocol. WHOIS traffic is not encrypted, so deployments using fallback must
allow outbound TCP port 43 and account for that protocol in their network policy.
Each WHOIS TCP query is attempted at most twice within the configured per-domain
timeout; cancellation stops further attempts.

## Metrics

Example output:

```text
domain_exporter_configured_domains 3
domain_registration_lookup_success{domain="example.com"} 1
domain_registration_lookup_source_info{domain="example.com",source="rdap"} 1
domain_registration_lookup_verified{domain="example.com"} 1
domain_registration_lookup_timestamp_seconds{domain="example.com"} 1742812800
domain_registration_expiration_timestamp_seconds{domain="example.com"} 1893456000
domain_registration_expiration_remaining_seconds{domain="example.com"} 150643200
domain_registration_last_success_timestamp_seconds{domain="example.com"} 1742812800
domain_registration_consecutive_failures{domain="example.com"} 0
domain_registration_data_available{domain="example.com"} 1
domain_registration_data_stale{domain="example.com"} 0
domain_cache_entries{cache="registration"} 3
domain_cache_hits_total{cache="registration"} 5
domain_rdap_up{source="rdap"} 1
domain_rdap_valid{source="rdap"} 1
domain_rdap_scrape_duration_seconds{source="rdap"} 0.452
domain_rdap_read_errors_total{source="rdap"} 0
domain_rdap_parse_errors_total{source="rdap"} 0
domain_whois_up{source="whois"} 1
domain_whois_valid{source="whois"} 1
domain_exporter_last_collection_success 1
domain_exporter_last_collection_timestamp_seconds 1742812800
domain_exporter_last_successful_collection_timestamp_seconds 1742812800
```

Domain metrics use the `domain` feature namespace. Framework-owned exporter
collection metrics use the `domain_exporter` metric namespace. The full metric
contract, including framework runtime and Prometheus-side scrape metrics, lives
in [`METRICS.md`](METRICS.md).

## Docker Compose

The repository includes [`docker-compose.yml`](docker-compose.yml) for local testing.
The Prometheus scrape config is embedded in Compose. Prometheus-managed alert
rules live under [`examples/prometheus`](examples/prometheus), while equivalent
Grafana-managed rules live under
[`examples/grafana/alerting`](examples/grafana/alerting).
The bundled rules cover exporter availability, framework collection
failure/staleness, RDAP and WHOIS source health, per-domain lookup failure,
last-known-good data availability/staleness, expiration windows, and incomplete
lookup coverage.
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

The container image is based on Alpine 3.24 and includes CA certificates plus
pinned OpenSSL runtime libraries for HTTPS RDAP requests. The exporter, its
container health check, and the Prometheus target use the fixed internal port
`9853`. `COMPOSE_EXPORTER_HOST_PORT` changes only the host-side port published
by Docker Compose.

## Grafana

Docker Compose provisions Grafana with:

- Prometheus datasource `DS_PROMETHEUS`
- dashboards from [`examples/grafana`](examples/grafana)
- Grafana-managed alert rules from
  [`examples/grafana/alerting`](examples/grafana/alerting)
- default login `admin` / `admin`

Open `http://localhost:3000` after `make compose`.
The main dashboard uses the Grafana v2 dashboard resource model and includes
domain status stats, a detailed Inventory table, registration source-health
graphs, historical changes, exporter collection health, Go runtime panels, and
Prometheus scrape health.

The Grafana-managed rules mirror the bundled Prometheus rules and carry
`service="prometheus-domain-exporter"` and `rule_source="grafana"` labels for
notification-policy routing. Contact points and notification policies are not
bundled because their credentials and ownership are deployment-specific. To
deliver notifications through Grafana, configure a contact point and route a
notification policy using those labels. Grafana evaluates its copies every
`30s`; their pending and firing state is independent from Prometheus-managed
rules.

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

`make go-check` runs Go-only checks, including the contract test that keeps
Prometheus and Grafana alert metadata and expressions synchronized. `make check`
also validates the Prometheus and Docker Compose examples, so it requires Docker.

## Scaffold-Owned Go Files

Go files named `scaffold_*.go` are generated contract glue and should stay
identical to the scaffold output. Add exporter-specific behavior in adjacent
non-scaffold files such as `feature_config_ext.go`, `feature_metrics_ext.go`,
`feature_snapshotter_ext.go`, `feature_smoke_ext.go`, `metrics.go`, and the
domain check package. The feature package `Snapshot` aggregate lives in
`snapshot_types.go`; the registration/domain engine snapshot lives in
`internal/domaincheck`.

Build local release artifacts:

```bash
make build VERSION=vX.Y.Z
make release VERSION=vX.Y.Z
make release-smoke VERSION=vX.Y.Z
```

Build and push a Docker image:

```bash
make docker-build VERSION=vX.Y.Z DOCKER_IMAGE=prometheus-domain-exporter:vX.Y.Z
make docker-push DOCKER_IMAGE=prometheus-domain-exporter:vX.Y.Z
make docker-buildx-push VERSION=vX.Y.Z DOCKER_IMAGE=registry.example.com/prometheus-domain-exporter:vX.Y.Z
```

## Architecture

The high-level design is documented in [`ARCHITECTURE.md`](ARCHITECTURE.md).

## License

This project is licensed under the MIT License. See [`LICENSE`](LICENSE).
