# Metrics

## Domain Registration Metrics

All domain registration metrics use a `domain` label containing the normalized
domain name configured with `--domain.name`.

`domain_registration_lookup_success`

Whether the last registration lookup for the domain succeeded.
Successful lookups have value `1`; failed lookups have value `0`.

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
The value is `0` when any configured domain lookup fails.

`domain_exporter_last_collection_timestamp_seconds`

Unix timestamp of the last refresh attempt.
The value is `0` before the first collection attempt.

`domain_exporter_last_successful_collection_timestamp_seconds`

Unix timestamp of the last successful refresh.
The value is `0` until the first successful refresh.
