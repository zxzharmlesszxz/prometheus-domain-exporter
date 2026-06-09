package domain

import (
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

func NewFeatureMetricHandlers() featurekit.FeatureMetricHandlers[Snapshot] {
	return featurekit.FeatureMetricHandlers[Snapshot]{
		Collect:  CollectFeatureMetrics,
		LogError: LogFeatureSnapshotError,
	}
}

func CollectFeatureMetrics(ctx featurekit.FeatureMetricsContext[Snapshot], ch chan<- prometheus.Metric, snapshot Snapshot, now time.Time) {
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricDomainConfiguredTotal),
		prometheus.GaugeValue,
		float64(len(snapshot.domain.Domains)),
	)

	for _, domain := range snapshot.domain.Domains {
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricDomainLookupSuccess),
			prometheus.GaugeValue,
			framework.BoolFloat(domain.Success),
			domain.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricDomainLookupTimestamp),
			prometheus.GaugeValue,
			framework.UnixTimestamp(domain.LookupTime),
			domain.Name,
		)
		if !domain.Success || domain.Expiration.IsZero() {
			continue
		}
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricDomainExpirationTimestamp),
			prometheus.GaugeValue,
			float64(domain.Expiration.Unix()),
			domain.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricDomainExpirationRemaining),
			prometheus.GaugeValue,
			domain.Expiration.Sub(now).Seconds(),
			domain.Name,
		)
	}
}

func LogFeatureSnapshotError(ctx featurekit.FeatureMetricsContext[Snapshot], logger *slog.Logger, snapshot Snapshot) {
	logged := false
	for _, result := range snapshot.domain.Domains {
		if result.Err != nil {
			logger.Error(
				ctx.FeatureName+" registration lookup failed",
				"domain", result.Name,
				"err", result.Err,
			)
			logged = true
		}
	}
	if snapshot.domain.Err != nil && !logged {
		logger.Error(ctx.FeatureName+" registration lookup failed", "err", snapshot.domain.Err)
	}
}
