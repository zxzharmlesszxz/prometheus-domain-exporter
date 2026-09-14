# Metrics

Domain-owned metrics use the `domain` feature namespace.
Framework-owned exporter metrics use the `domain_exporter` metric namespace.

## Domain Registration

Per-domain registration metrics use the `domain` label:

- `domain`: normalized domain name configured with `--domain.target` or
  `targets` in `prometheus-domain-exporter.yml`.

The lookup source info metric additionally uses `source` to identify `rdap` or
`whois`.

`domain_registration_lookup_success`

Whether the registration lookup for the domain completed without a transport,
service, or response-parsing error. A completed RDAP or WHOIS lookup has value
`1`; network timeouts, connection errors, bootstrap failures, non-404 HTTP
error responses, and malformed expiration data have value `0`. A not-found
response is a completed lookup, so it has lookup success `1` and lookup
verified `0`.
The exporter supplements the IANA RDAP bootstrap for `.io` and falls back to
registry WHOIS for TLDs such as `.ws` that do not publish RDAP.

`domain_registration_lookup_source_info`

Identifies the registration source used for the last lookup. The metric has
`domain` and `source` labels, where `source` is `rdap` or `whois`, and always
has value `1`. It is emitted for both successful and failed lookup attempts.

`domain_registration_lookup_verified`

Whether the RDAP or WHOIS response confirmed that the domain is registered. A
registered domain has value `1`; a not-found response has value `0` even when
the lookup itself completed successfully.

`domain_registration_lookup_timestamp_seconds`

Unix timestamp of the last external registration lookup attempt for the domain.
Reading a successful cached result does not advance this timestamp.

On a cache hit, lookup success, verification, source, and timestamp continue to
describe the cached external lookup; no registration source is contacted.

`domain_registration_expiration_timestamp_seconds`

Unix timestamp of the last confirmed domain registration expiration time returned
by RDAP or WHOIS. After a temporary refresh failure, the metric retains the last
known good value. It is absent until a lookup succeeds and after an authoritative
not-found response.

`domain_registration_expiration_remaining_seconds`

Seconds until the domain registration expiration time, calculated at scrape time
from the cached expiration timestamp. The value becomes negative after
expiration.

`domain_registration_last_success_timestamp_seconds`

Unix timestamp of the external lookup that produced the current confirmed
registration expiration. Cache reads and failed refreshes do not advance it.

`domain_registration_consecutive_failures`

Number of consecutive external refresh failures since the last confirmed
registration expiration. A successful lookup resets the value to `0`.

`domain_registration_data_available`

Whether confirmed registration expiration data is currently available. A
temporary refresh failure preserves value `1`; startup without a successful
lookup and an authoritative not-found response have value `0`.

`domain_registration_data_stale`

Whether the available registration data is at least 24 hours old. The metric is
`0` when data is unavailable. Staleness does not change the current lookup or
collection success metrics.

## Configured Target Count

`domain_exporter_configured_domains`

Number of configured domain targets in the current snapshot.

## RDAP Source Health

RDAP source-health metrics use the label:

- `source`: always `rdap`.

These metrics follow the framework source-health naming contract so shared
Grafana panels can discover them with `domain.*_(up|valid)` and related source
queries.

`domain_rdap_up`

Whether the latest RDAP collection had no lookup, transport, bootstrap, or HTTP
service errors. If any RDAP-backed domain result has a lookup error, this metric
is `0`.

`domain_rdap_valid`

Whether every RDAP-backed domain in the latest collection was verified and had
an expiration timestamp. WHOIS-backed domains do not affect this metric.

`domain_rdap_mtime_seconds`

Unix timestamp of the latest external RDAP lookup represented by the current
snapshot. RDAP is not a file-backed source, so this metric represents source
lookup time rather than file modification time. Reading cached results does not
advance it. With no configured domains, the idle RDAP source-health series uses
the current refresh timestamp.

`domain_rdap_scrape_duration_seconds`

Duration of the complete domain refresh represented by the RDAP source-health
snapshot. When both sources are present, RDAP and WHOIS expose the same complete
refresh duration.

`domain_rdap_read_errors_total`

Total number of RDAP lookup/source errors observed by the exporter. Lookup
errors include network failures, RDAP bootstrap/service failures, and non-404
HTTP errors.

`domain_rdap_parse_errors_total`

Total number of RDAP data validity errors observed by the exporter. This counts
domain results that completed the lookup but were not verified or did not expose
an expiration timestamp.

## WHOIS Source Health

WHOIS source-health metrics use the label:

- `source`: always `whois`.

The exporter emits these series when at least one configured target uses the
WHOIS fallback:

- `domain_whois_up`
- `domain_whois_valid`
- `domain_whois_mtime_seconds`
- `domain_whois_scrape_duration_seconds`
- `domain_whois_read_errors_total`
- `domain_whois_parse_errors_total`

They have the same semantics as the corresponding `domain_rdap_*` metrics, but
cover IANA WHOIS service discovery, registry WHOIS queries, and WHOIS expiration
parsing. WHOIS uses its standard unencrypted TCP port 43 protocol. The WHOIS
mtime metric likewise records the latest external lookup represented by the
snapshot rather than a cache read. Each TCP query is attempted at most twice
within the configured per-domain timeout. The WHOIS scrape-duration metric is
the complete domain refresh duration, matching the RDAP duration when both
sources are represented.

## Registration Cache

The in-memory registration lookup cache exposes the framework TTL cache metrics
with the stable label `cache="registration"`:

- `domain_cache_entries`: current number of live successful lookup results.
- `domain_cache_hits_total`: lookup refreshes served from cache.
- `domain_cache_misses_total`: lookup refreshes that required an external query.
- `domain_cache_sets_total`: successful results written to cache.
- `domain_cache_expired_total`: entries removed after their TTL elapsed.
- `domain_cache_deletes_total`: entries removed explicitly.
- `domain_cache_clears_total`: complete cache clear operations.

All counters are process-local and reset when the exporter restarts. Failed or
incomplete registration lookups are not cached, so they increment misses but
not sets.

The TTL cache suppresses unnecessary external queries; the per-domain
last-known-good state has a different purpose. It preserves the most recently
confirmed expiration across temporary transport, service, and parse failures.
Both stores are process-local and are cleared on restart.

## Exporter Collection Health

`domain_exporter_last_collection_success`

Whether the last refresh succeeded. The value is `0` when any configured domain
lookup fails, is not verified as registered, or does not provide a
registration expiration timestamp. With no configured domains, the completed
empty refresh is successful and the value is `1`.

`domain_exporter_last_collection_timestamp_seconds`

Unix timestamp of the last refresh attempt. The value is `0` before the first
collection attempt.

`domain_exporter_last_successful_collection_timestamp_seconds`

Unix timestamp of the last successful refresh. The value is `0` until the first
successful refresh.

`domain_exporter_collection_duration_seconds`

Histogram of framework collection refresh duration. It is emitted by the
framework collector and uses the standard Prometheus histogram series:

- `domain_exporter_collection_duration_seconds_bucket`
- `domain_exporter_collection_duration_seconds_sum`
- `domain_exporter_collection_duration_seconds_count`

## Build Info

`domain_exporter_build_info`

Build and runtime metadata exposed by the framework. Labels include version,
revision, branch, Go version, GOOS, and GOARCH. The metric value is always `1`.

## Runtime and Scrape Metrics

The framework also registers the standard Go and process collectors. Their
metrics use the upstream `go_*` and `process_*` names rather than the
`domain_exporter` namespace.

Metrics such as `up`, `scrape_duration_seconds`, `scrape_samples_scraped`,
`scrape_samples_post_metric_relabeling`, and `scrape_series_added` are generated
by Prometheus for the scrape target. They are not emitted by the exporter.

## Alert Evaluation

The repository provides two independent consumers of these metrics:

- Prometheus-managed rules in
  `examples/prometheus/prometheus-domain-exporter.yml`.
- Grafana-managed rules in
  `examples/grafana/alerting/prometheus-domain-exporter.yml`, using the
  `DS_PROMETHEUS` datasource.

Both sets implement the same 17 alert conditions, pending durations,
severities, summaries, and descriptions. Grafana rules add
`service="prometheus-domain-exporter"` and `rule_source="grafana"` labels for
notification-policy routing. Grafana contact points and notification policies
are deployment-owned and are not part of this repository. Because the two rule
engines evaluate independently, their pending and firing timestamps can differ
by an evaluation interval.

Expiration severity windows do not overlap: warning covers registrations with
at least 7 and less than 30 days remaining, while critical covers less than 7
days remaining, including already expired registrations.
