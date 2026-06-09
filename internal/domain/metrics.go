package domain

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

const (
	metricDomainExpirationRemaining = "domain_expiration_remaining"
	metricDomainExpirationTimestamp = "domain_expiration_timestamp"
	metricDomainLookupSuccess       = "domain_lookup_success"
	metricDomainLookupTimestamp     = "domain_lookup_timestamp"
	metricDomainConfiguredTotal     = "domain_configured_total"
)

var domainLabels = []string{
	"domain",
}

var featureMetricSpecs = []featurekit.FeatureMetricSpec{
	{
		ID:     metricDomainExpirationRemaining,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_registration_expiration_remaining_seconds",
		Help:   "Seconds until the domain registration expiration time",
		Labels: domainLabels,
	},
	{
		ID:     metricDomainExpirationTimestamp,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_registration_expiration_timestamp_seconds",
		Help:   "Unix timestamp of the domain registration expiration time",
		Labels: domainLabels,
	},
	{
		ID:     metricDomainLookupSuccess,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_registration_lookup_success",
		Help:   "Whether the last domain registration lookup succeeded",
		Labels: domainLabels,
	},
	{
		ID:     metricDomainLookupTimestamp,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_registration_lookup_timestamp_seconds",
		Help:   "Unix timestamp of the last domain registration lookup attempt",
		Labels: domainLabels,
	},
	{
		ID:    metricDomainConfiguredTotal,
		Scope: featurekit.MetricScopeNamespace,
		Name:  "_configured_domains",
		Help:  "Number of configured targets",
	},
}
