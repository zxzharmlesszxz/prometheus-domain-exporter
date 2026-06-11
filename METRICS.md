# Metrics

## Domain Registration Metrics

All domain registration metrics use a `domain` label containing the normalized
domain name configured with `--domain.target`.

`domain_exporter_configured_domains`

Number of configured domain targets in the current snapshot.

`domain_registration_lookup_success`

Whether the RDAP HTTP request for the domain succeeded (network/infra
health). A successful HTTP response has value `1`; network timeouts,
connection errors, or non-404 HTTP errors have value `0`.

`domain_registration_lookup_verified`

Whether the RDAP response confirmed the domain is registered (domain
config health). A confirmed domain has value `1`; an HTTP 404 Not
Found (unregistered domain) has value `0` even though the lookup
itself succeeded. This metric is emitted for every configured domain
regardless of lookup outcome.

`domain_registration_lookup_timestamp_seconds`

Unix timestamp of the last registration lookup attempt for the domain.

`domain_registration_expiration_timestamp_seconds`

Unix timestamp of the domain registration expiration time returned by RDAP.
This metric is only emitted for successful lookups with an expiration event.

`domain_registration_expiration_remaining_seconds`

Seconds until the domain registration expiration time, calculated at scrape
time from the cached expiration timestamp. The value becomes negative after
expiration.

## Exporter Collection Health

`domain_exporter_last_collection_success`

Whether the last refresh succeeded.
The value is `0` when any configured domain lookup fails, is not verified as
registered by RDAP, or does not provide a registration expiration timestamp.

`domain_exporter_last_collection_timestamp_seconds`

Unix timestamp of the last refresh attempt.
The value is `0` before the first collection attempt.

`domain_exporter_last_successful_collection_timestamp_seconds`

Unix timestamp of the last successful refresh.
The value is `0` until the first successful refresh.
