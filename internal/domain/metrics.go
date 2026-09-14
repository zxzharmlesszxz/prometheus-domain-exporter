package domain

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

const (
	metricDomainExpirationRemaining = "domain_expiration_remaining"
	metricDomainExpirationTimestamp = "domain_expiration_timestamp"
	metricDomainLookupSuccess       = "domain_lookup_success"
	metricDomainLookupSourceInfo    = "domain_lookup_source_info"
	metricDomainLookupVerified      = "domain_lookup_verified"
	metricDomainLookupTimestamp     = "domain_lookup_timestamp"
	metricDomainConfiguredTotal     = "domain_configured_total"

	metricRDAPSource  = "rdap"
	metricWHOISSource = "whois"
	registrationCache = "registration"
)

var rdapMetricIDs = featurekit.FileScrapeMetricIDsFor(metricRDAPSource)
var whoisMetricIDs = featurekit.FileScrapeMetricIDsFor(metricWHOISSource)

var domainLabels = []string{
	"domain",
}

var domainSourceLabels = []string{
	"domain",
	"source",
}

var rdapMetricSpecs = registrationSourceMetricSpecs(metricRDAPSource)
var whoisMetricSpecs = registrationSourceMetricSpecs(metricWHOISSource)
var cacheMetricSpecs = featurekit.TTLCacheMetricSpecs(nil)

func registrationSourceMetricSpecs(source string) []featurekit.FeatureMetricSpec {
	ids := featurekit.FileScrapeMetricIDsFor(source)
	help := map[string]string{
		ids.MTimeSeconds:          "Unix timestamp represented by the current " + source + " source-health snapshot.",
		ids.Up:                    "Whether the latest " + source + " results had no lookup or source errors.",
		ids.Valid:                 "Whether the latest " + source + " results were verified and included registration expiration.",
		ids.ReadErrorsTotal:       "Cumulative total number of " + source + " lookup or source errors.",
		ids.ParseErrorsTotal:      "Cumulative total number of " + source + " response parsing or validity errors.",
		ids.ScrapeDurationSeconds: "Duration in seconds of the latest domain refresh that included " + source + " results.",
	}
	specs := featurekit.FileScrapeMetricSpecs(source, []string{"source"})
	for i := range specs {
		if value, ok := help[specs[i].ID]; ok {
			specs[i].Help = value
		}
	}
	return specs
}

var domainMetricSpecs = []featurekit.FeatureMetricSpec{
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
		ID:     metricDomainLookupSourceInfo,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_registration_lookup_source_info",
		Help:   "Registration source used for the last domain lookup",
		Labels: domainSourceLabels,
	},
	{
		ID:     metricDomainLookupVerified,
		Scope:  featurekit.MetricScopeFeature,
		Name:   "_registration_lookup_verified",
		Help:   "Whether the last domain registration lookup confirmed the domain exists",
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

var featureMetricSpecs = append(append(append(append([]featurekit.FeatureMetricSpec{}, domainMetricSpecs...), rdapMetricSpecs...), whoisMetricSpecs...), cacheMetricSpecs...)
