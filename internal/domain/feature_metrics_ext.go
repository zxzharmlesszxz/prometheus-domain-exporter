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
	collectSourceMetrics(ctx, ch, rdapMetricIDs, snapshot.RDAPResult, snapshot.RDAPValid)
	collectSourceMetrics(ctx, ch, whoisMetricIDs, snapshot.WHOISResult, snapshot.WHOISValid)

	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricDomainConfiguredTotal),
		prometheus.GaugeValue,
		float64(len(snapshot.domain.Domains)),
	)

	for _, domain := range snapshot.domain.Domains {
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricDomainLookupSourceInfo),
			prometheus.GaugeValue,
			1,
			domain.Name,
			domain.Source,
		)
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricDomainLookupSuccess),
			prometheus.GaugeValue,
			framework.BoolFloat(domain.Success),
			domain.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricDomainLookupVerified),
			prometheus.GaugeValue,
			framework.BoolFloat(domain.Verified),
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

func collectSourceMetrics(ctx featurekit.FeatureMetricsContext[Snapshot], ch chan<- prometheus.Metric, metricIDs featurekit.FileScrapeMetricIDs, result framework.FileScrapeResult, valid bool) {
	if result.Path == "" {
		return
	}
	labelValues := []string{result.Path}
	ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricIDs.MTimeSeconds), prometheus.GaugeValue, result.MTimeSeconds, labelValues...)
	ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricIDs.Up), prometheus.GaugeValue, framework.BoolFloat(result.Up), labelValues...)
	ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricIDs.Valid), prometheus.GaugeValue, framework.BoolFloat(valid), labelValues...)
	ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricIDs.ReadErrorsTotal), prometheus.CounterValue, float64(result.ReadErrorsTotal), labelValues...)
	ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricIDs.ParseErrorsTotal), prometheus.CounterValue, float64(result.ParseErrorsTotal), labelValues...)
	ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(metricIDs.ScrapeDurationSeconds), prometheus.GaugeValue, result.ScrapeDurationSeconds, labelValues...)
}

func LogFeatureSnapshotError(ctx featurekit.FeatureMetricsContext[Snapshot], logger *slog.Logger, snapshot Snapshot) {
	logged := false
	for _, result := range snapshot.domain.Domains {
		if result.Err != nil {
			logger.Error(
				ctx.FeatureName+" registration lookup failed",
				"domain", result.Name,
				"source", result.Source,
				"err", result.Err,
			)
			logged = true
		}
	}
	if snapshot.domain.Err != nil && !logged {
		logger.Error(ctx.FeatureName+" registration lookup failed", "err", snapshot.domain.Err)
	}
}
