# Metrics

Domain-owned metrics use the `domain` feature namespace.
Framework-owned exporter metrics use the `domain_exporter` metric namespace.

## Domain Registration

Per-domain registration metrics use the label:

- `domain`: normalized domain name configured with `--domain.target` or
  `targets` in `prometheus-domain-exporter.yml`.

`domain_registration_lookup_success`

Whether the RDAP lookup for the domain completed without a transport or service
error. A successful RDAP lookup has value `1`; network timeouts, connection
errors, bootstrap failures, and non-404 HTTP errors have value `0`.

`domain_registration_lookup_verified`

Whether the RDAP response confirmed that the domain is registered. A registered
domain has value `1`; an RDAP HTTP 404 Not Found response has value `0` even
when the lookup itself completed successfully.

`domain_registration_lookup_timestamp_seconds`

Unix timestamp of the last registration lookup attempt for the domain.

`domain_registration_expiration_timestamp_seconds`

Unix timestamp of the domain registration expiration time returned by RDAP.
This metric is emitted only for successful lookups with an expiration event.

`domain_registration_expiration_remaining_seconds`

Seconds until the domain registration expiration time, calculated at scrape time
from the cached expiration timestamp. The value becomes negative after
expiration.

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
service errors. If any domain result has a lookup error, this metric is `0`.

`domain_rdap_valid`

Whether the latest RDAP collection produced a fully valid domain snapshot. This
is equivalent to `domain_exporter_last_collection_success`: every configured
domain must be verified and have an expiration timestamp.

`domain_rdap_mtime_seconds`

Unix timestamp of the latest RDAP collection attempt. RDAP is not a file-backed
source, so this metric represents source refresh time rather than file
modification time.

`domain_rdap_scrape_duration_seconds`

Duration in seconds of the latest RDAP collection work.

`domain_rdap_read_errors_total`

Total number of RDAP lookup/source errors observed by the exporter. Lookup
errors include network failures, RDAP bootstrap/service failures, and non-404
HTTP errors.

`domain_rdap_parse_errors_total`

Total number of RDAP data validity errors observed by the exporter. This counts
domain results that completed the lookup but were not verified or did not expose
an expiration timestamp.

## Exporter Collection Health

`domain_exporter_last_collection_success`

Whether the last refresh succeeded. The value is `0` when any configured domain
lookup fails, is not verified as registered by RDAP, or does not provide a
registration expiration timestamp.

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
