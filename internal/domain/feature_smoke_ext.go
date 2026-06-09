package domain

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

var DefaultFeatureConfigPath = "../examples/" + DefaultFeatureConfigFileName

func FeatureSmoke(ctx featurekit.SmokeContext[Config]) featurekit.SmokeSpec {
	checkSuccess := featurekit.FeatureMetricName(ctx.FeatureName, "", metricDomainLookupSuccess, featureMetricSpecs)

	return featurekit.SmokeSpec{
		ServerArgs: []string{
			"--" + ctx.FeatureName + ".config-file=" + DefaultFeatureConfigPath,
			"--" + ctx.FeatureName + ".max-concurrent-targets=2",
		},
		RejectMetrics: []string{
			checkSuccess,
		},
	}
}
