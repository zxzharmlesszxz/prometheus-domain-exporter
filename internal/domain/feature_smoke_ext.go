package domain

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

func FeatureSmoke(ctx featurekit.SmokeContext[Config]) featurekit.SmokeSpec {
	return featurekit.SmokeSpec{
		ServerArgs: []string{
			"--" + ctx.FeatureName + ".config-file=" + DefaultFeatureConfigPath,
		},
		RejectMetrics: []string{
			featurekit.FeatureMetricName(ctx.FeatureName, "", metricDomainLookupSuccess, featureMetricSpecs),
		},
	}
}
